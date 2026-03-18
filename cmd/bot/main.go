// Package main is the entry point for ARK Community Intelligent.
// It wires all dependencies using manual DI (no framework), starts
// background schedulers, and runs the Telegram long-polling loop.
//
// Shutdown is graceful: SIGINT/SIGTERM stops polling, drains in-flight
// handlers (10s deadline), cancels background jobs, flushes storage,
// then exits.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	tgbot "github.com/arkcode369/ark-intelligent/internal/adapter/telegram"
	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/health"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/scheduler"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	cotsvc "github.com/arkcode369/ark-intelligent/internal/service/cot"
	newssvc "github.com/arkcode369/ark-intelligent/internal/service/news"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

//go:embed CHANGELOG.md
var changelogContent string

var log = logger.Component("main")

const banner = `
╔══════════════════════════════════════════════════╗
║     Institutional Positioning (COT) • Macro Intel ║
║     Built for institutional-grade macro intel     ║
╚══════════════════════════════════════════════════╝`

func main() {
	fmt.Println(banner)

	// -----------------------------------------------------------------------
	// 1. Configuration
	// -----------------------------------------------------------------------
	cfg := config.MustLoad()
	logger.Init(cfg.LogLevel)
	// Re-initialize component logger after Init
	log = logger.Component("main")

	log.Info().
		Str("version", "v1.0").
		Str("go", runtime.Version()).
		Str("os", runtime.GOOS).
		Str("arch", runtime.GOARCH).
		Msg("Starting ARK Community Intelligent")

	log.Info().Str("config", cfg.String()).Msg("Config loaded")

	// -----------------------------------------------------------------------
	// 2. Root context with cancellation (drives graceful shutdown)
	// -----------------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// -----------------------------------------------------------------------
	// 3. Storage layer
	// -----------------------------------------------------------------------
	db, err := storage.Open(cfg.DataDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open storage")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("Storage close error")
		}
	}()

	eventRepo := storage.NewEventRepo(db)
	cotRepo := storage.NewCOTRepo(db)
	prefsRepo := storage.NewPrefsRepo(db)
	newsRepo := storage.NewNewsRepo(db)
	cacheRepo := storage.NewCacheRepo(db)

	log.Info().Msg("Storage layer initialized")
	logStorageSize(db)

	// -----------------------------------------------------------------------
	// 3b. Health check endpoint
	// -----------------------------------------------------------------------
	healthChecker := health.New(func() error {
		// Simple DB liveness check via Size() — if it panics, DB is dead
		db.Size()
		return nil
	})
	healthAddr := config.GetEnvDefault("HEALTH_ADDR", ":8080")
	go healthChecker.Start(ctx, healthAddr)

	// -----------------------------------------------------------------------
	// 4. Telegram bot
	// -----------------------------------------------------------------------
	bot := tgbot.NewBot(cfg.BotToken, cfg.ChatID)

	log.Info().Msg("Telegram bot created")

	// -----------------------------------------------------------------------
	// 5. AI layer (optional — graceful degradation)
	// -----------------------------------------------------------------------
	var aiAnalyzer ports.AIAnalyzer
	var cachedAI *aisvc.CachedInterpreter

	if cfg.HasGemini() {
		gemini, err := aisvc.NewGeminiClient(ctx, cfg.GeminiAPIKey)
		if err != nil {
			log.Warn().Err(err).Msg("Gemini init failed, AI features disabled")
		} else {
			rawAI := aisvc.NewInterpreter(gemini, eventRepo, cotRepo)
			cachedAI = aisvc.NewCachedInterpreter(rawAI, cacheRepo)
			aiAnalyzer = cachedAI
			log.Info().Msg("Gemini AI initialized (with cache layer)")
		}
	} else {
		log.Info().Msg("No GEMINI_API_KEY — AI features disabled (template fallback active)")
	}

	// -----------------------------------------------------------------------
	// 6. Service layer
	// -----------------------------------------------------------------------

	// COT services
	cotFetcher := cotsvc.NewFetcher()
	cotAnalyzer := cotsvc.NewAnalyzer(cotRepo, cotFetcher)

	// News services (uses MQL5 Economic Calendar API — no API key required)
	newsFetcher := newssvc.NewMQL5Fetcher()
	log.Info().Msg("Service layer initialized")

	// -----------------------------------------------------------------------
	// 7. Background schedulers
	// -----------------------------------------------------------------------
	sched := scheduler.New(&scheduler.Deps{
		COTAnalyzer: cotAnalyzer,
		AIAnalyzer:  aiAnalyzer,
		Bot:         bot,
		COTRepo:     cotRepo,
		PrefsRepo:   prefsRepo,
		ChatID:      cfg.ChatID,
		CachedAI:    cachedAI,
		DB:          db,
	})

	sched.Start(ctx, &scheduler.Intervals{
		COTFetch: cfg.COTFetchInterval,
	})

	// News Background Scheduler (always starts — uses MQL5 Economic Calendar)
	// P1.1: cotRepo injected for Confluence Alert cross-check on actual releases
	// newsSched is created before NewHandler so the surprise accumulator can be injected.
	newsSched := newssvc.NewScheduler(newsRepo, newsFetcher, aiAnalyzer, bot, prefsRepo, cotRepo)

	// Wire AI cache invalidation on significant news releases
	if cachedAI != nil {
		newsSched.SetNewsInvalidateFunc(cachedAI.InvalidateOnNewsUpdate)
	}

	newsSched.Start(ctx)
	log.Info().Msg("News Background scheduler started")

	// -----------------------------------------------------------------------
	// 8. Telegram handler (registers commands on bot)
	// -----------------------------------------------------------------------
	// Handler is wired after newsSched so it can receive the surprise accumulator.
	// newsSched implements SurpriseProvider via GetSurpriseSigma — enables full
	// 3-source conviction scoring (COT + FRED + Calendar) in /rank and /cot detail.
	_ = tgbot.NewHandler(
		bot,
		eventRepo,
		cotRepo,
		prefsRepo,
		newsRepo,
		newsFetcher,
		aiAnalyzer,     // nil-safe: handler checks IsAvailable()
		changelogContent,
		newsSched,      // SurpriseProvider: weekly per-currency surprise accumulator
	)

	log.Info().Msg("Telegram handler registered")

	log.Info().Msg("Background schedulers started")

	// -----------------------------------------------------------------------
	// 9. Initial data load (non-blocking)
	// -----------------------------------------------------------------------
	go func() {
		initCtx, initCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer initCancel()

		log.Info().Msg("Running initial data load...")

		// Fetch and sync COT history (this pulls 52 weeks for all contracts)
		log.Info().Msg("Syncing COT history (this may take a moment)...")
		if err := cotAnalyzer.SyncHistory(initCtx); err != nil {
			log.Error().Err(err).Msg("COT history sync failed")
			// Even if full history sync fails, attempt a fresh fetch of latest data
			log.Info().Msg("Attempting fallback: fetch latest COT only...")
			if _, err2 := cotAnalyzer.AnalyzeAll(initCtx); err2 != nil {
				log.Error().Err(err2).Msg("Fallback COT fetch also failed")
			} else {
				log.Info().Msg("Fallback COT fetch succeeded")
			}
		} else {
			log.Info().Msg("COT history sync complete")
		}

		// Gap B — Backfill RegimeAdjustedScore for any stored analyses that predate the feature.
		// Non-fatal: logs warning and continues if FRED data is unavailable.
		if err := cotAnalyzer.BackfillRegimeScores(initCtx); err != nil {
			log.Warn().Err(err).Msg("backfill regime scores (non-fatal)")
		}

		// Send startup notification
		startupMsg := fmt.Sprintf(
			"🦅 <b>ARK Intelligence Online</b>\n"+
				"<i>Systems synchronized</i>\n\n"+
				"<code>AI Engine :</code> %s\n"+
				"<code>Calendar  :</code> MQL5 Economic Calendar\n"+
				"<code>COT Data  :</code> CFTC Socrata\n\n"+
				"Type /help for commands",
			aiStatus(aiAnalyzer),
		)
		if _, err := bot.SendHTML(initCtx, cfg.ChatID, startupMsg); err != nil {
			log.Error().Err(err).Msg("Failed to send startup notification")
		}
	}()

	// -----------------------------------------------------------------------
	// 10. Signal handling & graceful shutdown
	// -----------------------------------------------------------------------
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start polling in a goroutine
	pollDone := make(chan struct{})
	go func() {
		defer close(pollDone)
		log.Info().Msg("Starting Telegram long-polling...")
		if err := bot.StartPolling(ctx); err != nil {
			log.Error().Err(err).Msg("Polling exited with error")
		}
		log.Info().Msg("Polling stopped")
	}()

	// Block until signal
	sig := <-sigCh
	log.Info().Str("signal", sig.String()).Msg("Received signal — initiating graceful shutdown")

	// Phase 1: Cancel context (stops polling + schedulers)
	cancel()

	// Phase 2: Wait for polling to drain (max 10s)
	select {
	case <-pollDone:
		log.Info().Msg("Polling drained cleanly")
	case <-time.After(10 * time.Second):
		log.Warn().Msg("Polling drain timed out after 10s")
	}

	// Phase 3: Stop scheduler
	sched.Stop()
	log.Info().Msg("Scheduler stopped")

	// Phase 4: Close storage (handled by defer)
	log.Info().Msg("Shutdown complete. Goodbye.")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// aiStatus returns a human-readable AI status string.
func aiStatus(ai ports.AIAnalyzer) string {
	if ai != nil && ai.IsAvailable() {
		return "Active"
	}
	return "Offline"
}

// logStorageSize logs the current database size.
func logStorageSize(db *storage.DB) {
	lsm, vlog := db.Size()
	total := lsm + vlog
	if total > 1<<20 {
		log.Info().
			Float64("total_mb", float64(total)/(1<<20)).
			Float64("lsm_mb", float64(lsm)/(1<<20)).
			Float64("vlog_mb", float64(vlog)/(1<<20)).
			Msg("Storage size")
	} else {
		log.Info().
			Int64("total_kb", total>>10).
			Int64("lsm_kb", lsm>>10).
			Int64("vlog_kb", vlog>>10).
			Msg("Storage size")
	}
}
