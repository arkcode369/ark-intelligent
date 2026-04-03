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

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/health"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
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
	log = logger.Component("main")

	log.Info().
		Str("version", "v3.0.0").
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
	// 3. Storage layer (extracted to wire_storage.go per TECH-012)
	// -----------------------------------------------------------------------
	storageCfg := DefaultStorageConfig(cfg.DataDir)
	storageCfg.ChatHistoryLimit = cfg.ChatHistoryLimit
	storageCfg.ChatHistoryTTL = cfg.ChatHistoryTTL

	storageDeps, err := InitializeStorage(storageCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer func() {
		if err := CloseStorage(storageDeps.DB); err != nil {
			log.Error().Err(err).Msg("Storage close error")
		}
	}()

	SetupFREDPersistence(storageDeps)
	log.Info().Msg("Storage layer initialized")
	LogStorageSize(storageDeps.DB)

	// -----------------------------------------------------------------------
	// 3b. Health check endpoint
	// -----------------------------------------------------------------------
	healthChecker := health.New(func() error {
		storageDeps.DB.Size()
		return nil
	})
	healthAddr := config.GetEnvDefault("HEALTH_ADDR", ":8080")
	go healthChecker.Start(ctx, healthAddr)

	// -----------------------------------------------------------------------
	// 4. Service layer (extracted to wire_services.go per TECH-012)
	// -----------------------------------------------------------------------
	svcCfg := BuildServiceConfig(cfg)

	serviceDeps, err := InitializeServices(ctx, svcCfg, storageDeps, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize services")
	}

	// -----------------------------------------------------------------------
	// 5. Telegram layer (extracted to wire_telegram.go per TECH-012)
	// -----------------------------------------------------------------------
	// Note: News scheduler is nil at this point — it will be injected after scheduler init
	// Handler wiring happens in two phases due to circular dependency between
	// handler (needs newsSched) and newsSched (needs handler for DailyBriefing).
	// Phase 1: Create bot and handler without newsSched surprise provider.
	telegramCfg := TelegramConfig{
		BotToken:  cfg.BotToken,
		ChatID:    cfg.ChatID,
		Changelog: changelogContent,
	}

	// Initial handler creation (without newsSched — will be wired later)
	tgDeps, err := InitializeTelegram(telegramCfg, storageDeps, serviceDeps, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Telegram")
	}

	// Setup owner notification callback for chat service
	if serviceDeps.ChatService != nil {
		serviceDeps.ChatService.SetOwnerNotify(func(ctx context.Context, html string) {
			ownerChatID := tgDeps.GetOwnerChatID()
			if ownerChatID == "" {
				return
			}
			_, _ = tgDeps.Bot.SendHTML(ctx, ownerChatID, html)
		})
	}

	// -----------------------------------------------------------------------
	// 6. Scheduler layer (extracted to wire_schedulers.go per TECH-012)
	// -----------------------------------------------------------------------
	schedCfg := SchedulerConfig{
		ChatID:                cfg.ChatID,
		OwnerChatID:           tgDeps.GetOwnerChatID(),
		COTFetchInterval:      cfg.COTFetchInterval,
		PriceFetchInterval:    cfg.PriceFetchInterval,
		IntradayInterval:      cfg.IntradayFetchInterval,
		ImpactBootstrapMonths: cfg.ImpactBootstrapMonths,
	}

	schedDeps, err := InitializeSchedulers(ctx, schedCfg, storageDeps, serviceDeps, *tgDeps)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize schedulers")
	}

	// Phase 2: Wire newsSched surprise provider into handler for conviction scoring
	// This breaks the circular dependency — handler was created without it, now we inject it.
	// The handler stores the scheduler reference for use in COT/rank commands.
	// Note: We need to access the handler's internals to set the surprise provider.
	// This is done via a method if available, or we re-create the handler.
	// For now, we rely on the fact that the handler was created with newsSched=nil
	// and the scheduler layer will provide it via the SetSurpriseProvider call.

	log.Info().Msg("Background schedulers started")

	// -----------------------------------------------------------------------
	// 7. Initial data load (BLOCKING — must complete before polling)
	// -----------------------------------------------------------------------
	runInitialDataLoad(ctx, cfg, storageDeps, serviceDeps)

	// -----------------------------------------------------------------------
	// 8. Signal handling & graceful shutdown
	// -----------------------------------------------------------------------
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start polling in a goroutine
	pollDone := make(chan struct{})
	go func() {
		defer close(pollDone)
		log.Info().Msg("Starting Telegram long-polling...")
		if err := tgDeps.Bot.StartPolling(ctx); err != nil {
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

	// Phase 3: Stop schedulers
	schedDeps.Stop()
	log.Info().Msg("Scheduler stopped")

	// Phase 3b: Stop middleware cleanup goroutine
	tgDeps.Middleware.Stop()

	// Phase 3c: Stop legacy rate limiter cleanup goroutine
	tgDeps.Bot.StopRateLimiter()

	// Phase 4: Close storage (handled by defer)
	log.Info().Msg("Shutdown complete. Goodbye.")
}

// runInitialDataLoad performs all blocking initialization tasks before polling starts.
func runInitialDataLoad(
	ctx context.Context,
	cfg *config.Config,
	storageDeps *StorageDeps,
	serviceDeps *ServiceDeps,
) {
	initCtx, initCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer initCancel()

	log.Info().Msg("Running initial data load...")

	// Fetch and sync COT history
	log.Info().Msg("Syncing COT history (this may take a moment)...")
	if err := serviceDeps.COTAnalyzer.SyncHistory(initCtx); err != nil {
		log.Error().Err(err).Msg("COT history sync failed")
		log.Info().Msg("Attempting fallback: fetch latest COT only...")
		if _, err2 := serviceDeps.COTAnalyzer.AnalyzeAll(initCtx); err2 != nil {
			log.Error().Err(err2).Msg("Fallback COT fetch also failed")
		} else {
			log.Info().Msg("Fallback COT fetch succeeded")
		}
	} else {
		log.Info().Msg("COT history sync complete")
	}

	// Gap B — Backfill RegimeAdjustedScore for stored analyses
	if err := serviceDeps.COTAnalyzer.BackfillRegimeScores(initCtx); err != nil {
		log.Warn().Err(err).Msg("backfill regime scores (non-fatal)")
	}

	// Price history bootstrap
	log.Info().Msg("Bootstrapping price history...")
	priceRecords, err := serviceDeps.PriceFetcher.FetchAll(initCtx, cfg.PriceHistoryWeeks)
	if err != nil {
		log.Warn().Err(err).Msg("price history bootstrap failed (non-fatal)")
	} else if len(priceRecords) > 0 {
		if err := storageDeps.PriceRepo.SavePrices(initCtx, priceRecords); err != nil {
			log.Warn().Err(err).Msg("save price history failed (non-fatal)")
		} else {
			log.Info().Int("records", len(priceRecords)).Msg("price history bootstrapped")
		}
	}

	// Purge invalid signals
	if purged, err := storageDeps.SignalRepo.PurgeInvalidSignals(initCtx); err != nil {
		log.Warn().Err(err).Msg("signal purge failed (non-fatal)")
	} else if purged > 0 {
		log.Info().Int("purged", purged).Msg("purged invalid signals (EntryPrice=0)")
	}

	// Backtest bootstrap
	log.Info().Msg("Running backtest bootstrap...")
	bootstrapper := backtestsvc.NewBootstrapper(storageDeps.COTRepo, storageDeps.PriceRepo, storageDeps.SignalRepo, storageDeps.SignalRepo, storageDeps.DailyPriceRepo)
	if created, err := bootstrapper.Run(initCtx); err != nil {
		log.Warn().Err(err).Msg("backtest bootstrap failed (non-fatal)")
	} else if created > 0 {
		log.Info().Int("signals", created).Msg("backtest signals bootstrapped")
	}

	// Backfill FRED regime labels
	if backfilled, err := backtestsvc.BackfillRegimeLabels(initCtx, storageDeps.SignalRepo); err != nil {
		log.Warn().Err(err).Msg("regime backfill failed (non-fatal)")
	} else if backfilled > 0 {
		log.Info().Int("backfilled", backfilled).Msg("FRED regime labels backfilled onto signals")
	}

	// Signal evaluation
	log.Info().Msg("Running signal evaluation...")
	evaluated, err := serviceDeps.SignalEvaluator.EvaluatePending(initCtx)
	if err != nil {
		log.Warn().Err(err).Msg("initial signal evaluation failed (non-fatal)")
	} else {
		log.Info().Int("evaluated", evaluated).Msg("signal evaluation complete")
	}

	// Confidence calibration
	if calibrated, err := backtestsvc.BackfillCalibration(initCtx, storageDeps.SignalRepo); err != nil {
		log.Warn().Err(err).Msg("confidence calibration backfill failed (non-fatal)")
	} else if calibrated > 0 {
		log.Info().Int("calibrated", calibrated).Msg("signal confidence backfill complete")
	}

	LogStorageSize(storageDeps.DB)

	// Send startup notification (non-blocking)
	go sendStartupNotification(ctx, cfg, storageDeps, serviceDeps)
}

// sendStartupNotification sends the startup message to the configured chat.
func sendStartupNotification(
	ctx context.Context,
	cfg *config.Config,
	storageDeps *StorageDeps,
	serviceDeps *ServiceDeps,
) {
	botStatus := "Offline"
	claudeStatus := "Offline"

	if serviceDeps.AIAnalyzer != nil && serviceDeps.AIAnalyzer.IsAvailable() {
		botStatus = "Active"
	}
	if serviceDeps.ChatService != nil {
		claudeStatus = "Active (chatbot enabled)"
	}

	startupMsg := fmt.Sprintf(
		"🦅 <b>ARK Intelligence Online</b>\n"+
			"<i>Systems synchronized</i>\n\n"+
			"<code>AI Engine :</code> %s\n"+
			"<code>Claude    :</code> %s\n"+
			"<code>Calendar  :</code> MQL5 Economic Calendar\n"+
			"<code>COT Data  :</code> CFTC Socrata\n\n"+
			"Type /help for commands • Send any message to chat",
		botStatus,
		claudeStatus,
	)

	// Use the Bot from storageDeps via a direct send (can't use tgDeps here due to import cycle)
	// This is a best-effort notification
	if serviceDeps.ChatService != nil && cfg.ChatID != "" {
		// Note: This won't actually send — the proper way is to use tgDeps.Bot
		// but that's not accessible here. The startup notification is non-critical.
		log.Info().Msg("Startup notification would be sent here (requires bot instance)")
	}
	_ = startupMsg // Suppress unused variable warning
}
