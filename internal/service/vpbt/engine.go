package vpbt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	"github.com/rs/zerolog/log"
)

// VPBacktestParams holds parameters for VP backtest.
type VPBacktestParams struct {
	Symbol    string      `json:"symbol"`
	Timeframe string      `json:"timeframe"`
	Mode      string      `json:"mode"`
	Grade     string      `json:"grade"`
	BarsData  []OHLCVJSON `json:"bars_data"`
	Params    VPParams    `json:"params"`
}

type VPParams struct {
	InitialCapital float64 `json:"initial_capital"`
	RiskPerTrade   float64 `json:"risk_per_trade"`
	StopLossPips   float64 `json:"stop_loss_pips"`
	TakeProfitPips float64 `json:"take_profit_pips"`
}

type OHLCVJSON struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

// VPBacktestResult holds the result from Python engine.
type VPBacktestResult struct {
	Success    bool     `json:"success"`
	TextOutput string   `json:"text_output"`
	ChartPath  string   `json:"chart_path"`
	Result     VPResult `json:"result"`
}

type VPResult struct {
	TotalTrades   int       `json:"total_trades"`
	WinRate       float64   `json:"win_rate"`
	ProfitFactor  float64   `json:"profit_factor"`
	SharpeRatio   float64   `json:"sharpe_ratio"`
	MaxDrawdown   float64   `json:"max_drawdown"`
	ExpectedValue float64   `json:"expected_value"`
	AvgWin        float64   `json:"avg_win"`
	AvgLoss       float64   `json:"avg_loss"`
	TotalPnL      float64   `json:"total_pnl"`
	WinCount      int       `json:"win_count"`
	LossCount     int       `json:"loss_count"`
	EquityCurve   []float64 `json:"equity_curve"`
	Drawdown      []float64 `json:"drawdown"`
	Trades        []VPTrade `json:"trades"`
	VPLLevels     VPLLevels `json:"vp_levels"`
}

type VPTrade struct {
	EntryTime  string  `json:"entry_time"`
	ExitTime   string  `json:"exit_time"`
	EntryPrice float64 `json:"entry_price"`
	ExitPrice  float64 `json:"exit_price"`
	Direction  string  `json:"direction"`
	PnL        float64 `json:"pnl"`
	Reason     string  `json:"reason"`
	Grade      string  `json:"grade"`
}

type VPLLevels struct {
	POC      float64      `json:"poc"`
	VAH      float64      `json:"vah"`
	VAL      float64      `json:"val"`
	HVNZones [][2]float64 `json:"hvn_zones"`
	LVNZones [][2]float64 `json:"lvn_zones"`
	Prices   []float64    `json:"prices"`
	Volumes  []float64    `json:"volumes"`
}

// RunVPBacktest executes the Python VP backtest engine with real data.
func RunVPBacktest(ctx context.Context, symbol, timeframe, mode, grade string, bars []ta.OHLCV) (*VPBacktestResult, error) {
	if len(bars) == 0 {
		return nil, fmt.Errorf("no price data available for backtest")
	}

	// Convert bars to JSON format with proper volume handling
	barsJSON := make([]OHLCVJSON, len(bars))
	hasVolume := false
	for i, bar := range bars {
		vol := float64(bar.Volume)
		if vol > 0 {
			hasVolume = true
		}
		barsJSON[i] = OHLCVJSON{
			Time:   bar.Date.Format(time.RFC3339),
			Open:   bar.Open,
			High:   bar.High,
			Low:    bar.Low,
			Close:  bar.Close,
			Volume: vol,
		}
	}

	if !hasVolume {
		log.Warn().Str("symbol", symbol).Msg("No volume data available, using range-based proxy")
	}

	// Build input config with configurable params (can be extended via env or user prefs)
	initialCapital := 10000.0
	if env := os.Getenv("VPBT_INITIAL_CAPITAL"); env != "" {
		if val, err := fmt.Sscanf(env, "%f", &initialCapital); err == nil && val == 1 {
			// Parse success
		}
	}

	input := VPBacktestParams{
		Symbol:    symbol,
		Timeframe: timeframe,
		Mode:      mode,
		Grade:     grade,
		BarsData:  barsJSON,
		Params: VPParams{
			InitialCapital: initialCapital,
			RiskPerTrade:   2.0,
			StopLossPips:   0, // 0 = use dynamic ATR
			TakeProfitPips: 0, // 0 = use dynamic ATR
		},
	}

	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	// Write temp input file
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("vpbt_input_%d.json", time.Now().UnixNano()))
	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write input file: %w", err)
	}
	defer os.Remove(inputPath)

	// Find Python script
	scriptPath := findVPBTScript()
	if scriptPath == "" {
		return nil, fmt.Errorf("vpbt_engine.py not found")
	}

	// Run Python engine with longer timeout for large datasets
	timeout := 120 * time.Second
	if len(bars) > 500 {
		timeout = 180 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", scriptPath, inputPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Error().Int("bars", len(bars)).Msg("vpbt engine timeout")
			return nil, fmt.Errorf("backtest timeout after %v (processing %d bars). Try reducing timeframe or mode complexity", timeout, len(bars))
		}
		log.Error().Err(err).Str("stderr", stderr.String()).Int("bars", len(bars)).Msg("vpbt engine failed")
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return nil, fmt.Errorf("backtest failed: %s", errMsg)
	}

	// Parse output
	var result VPBacktestResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		log.Error().Str("stdout", stdout.String()[:min(200, len(stdout.String()))]).Msg("failed to parse vpbt output")
		return nil, fmt.Errorf("failed to parse backtest output: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("backtest failed: %s", result.TextOutput)
	}

	// Clean up chart file after reading (chart will be sent via Telegram and then deleted)
	if result.ChartPath != "" {
		defer func(path string) {
			if err := os.Remove(path); err != nil {
				log.Warn().Str("path", path).Err(err).Msg("failed to clean up chart file")
			}
		}(result.ChartPath)
	}

	return &result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func findVPBTScript() string {
	candidates := []string{
		"scripts/vpbt_engine.py",
		"../scripts/vpbt_engine.py",
	}
	if d := os.Getenv("SCRIPTS_DIR"); d != "" {
		candidates = append([]string{filepath.Join(d, "vpbt_engine.py")}, candidates...)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		rel := filepath.Join(execDir, "scripts", "vpbt_engine.py")
		if _, err := os.Stat(rel); err == nil {
			return rel
		}
	}
	return ""
}
