package quantbt

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

// QuantBTParams holds parameters for quant backtest.
type QuantBTParams struct {
	Symbol    string           `json:"symbol"`
	Timeframe string           `json:"timeframe"`
	BarsData  []OHLCVJSON      `json:"bars"`
	Params    QuantBTParamsMap `json:"params"`
}

type QuantBTParamsMap struct {
	InitialCapital float64 `json:"initial_capital"`
	RiskPerTrade   float64 `json:"risk_per_trade"`
	Grade          string  `json:"grade"`
	Lookback       int     `json:"lookback"`
	MaxBars        int     `json:"max_bars"`
}

type OHLCVJSON struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

// QuantBTResult holds the result from Python engine.
type QuantBTResult struct {
	Mode       string   `json:"mode"`
	Symbol     string   `json:"symbol"`
	Success    bool     `json:"success"`
	Error      string   `json:"error"`
	Result     BTResult `json:"result"`
	TextOutput string   `json:"text_output"`
	ChartPath  string   `json:"chart_path"`
}

type BTResult struct {
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
	Trades        []BTTrade `json:"trades"`
}

type BTTrade struct {
	EntryTime  string  `json:"entry_time"`
	ExitTime   string  `json:"exit_time"`
	EntryPrice float64 `json:"entry_price"`
	ExitPrice  float64 `json:"exit_price"`
	Direction  string  `json:"direction"`
	PnL        float64 `json:"pnl"`
	Reason     string  `json:"reason"`
	Grade      string  `json:"grade"`
}

// RunQuantBacktest executes the Python quant backtest engine with real data.
func RunQuantBacktest(ctx context.Context, symbol, timeframe, grade string, bars []ta.OHLCV) (*QuantBTResult, error) {
	if len(bars) == 0 {
		return nil, fmt.Errorf("no price data available for quant backtest")
	}

	// Convert bars to JSON format (date-only for quant engine)
	barsJSON := make([]OHLCVJSON, len(bars))
	for i, bar := range bars {
		barsJSON[i] = OHLCVJSON{
			Date:   bar.Date.Format("2006-01-02"),
			Open:   bar.Open,
			High:   bar.High,
			Low:    bar.Low,
			Close:  bar.Close,
			Volume: float64(bar.Volume),
		}
	}

	// Build input config
	input := QuantBTParams{
		Symbol:    symbol,
		Timeframe: timeframe,
		BarsData:  barsJSON,
		Params: QuantBTParamsMap{
			InitialCapital: 10000,
			RiskPerTrade:   0.02,
			Grade:          grade,
			Lookback:       100,
			MaxBars:        50,
		},
	}

	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	// Write temp input file
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("quantbt_input_%d.json", time.Now().UnixNano()))
	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write input file: %w", err)
	}
	defer os.Remove(inputPath)

	// Find Python script
	scriptPath := findQuantBTScript()
	if scriptPath == "" {
		return nil, fmt.Errorf("quant_engine.py not found")
	}

	// Run Python engine with timeout
	timeout := 180 * time.Second
	if len(bars) < 300 {
		timeout = 120 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", scriptPath, inputPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("quant backtest timeout after %v (processing %d bars)", timeout, len(bars))
		}
		log.Error().Err(err).Str("stderr", stderr.String()).Int("bars", len(bars)).Msg("quant engine failed")
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("quant backtest failed: %s", errMsg)
	}

	// Parse output
	var result QuantBTResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		log.Error().Str("stdout", stdout.String()[:min(200, len(stdout.String()))]).Msg("failed to parse quant output")
		return nil, fmt.Errorf("failed to parse quant output: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("quant backtest failed: %s", result.Error)
	}

	return &result, nil
}

func findQuantBTScript() string {
	candidates := []string{
		"scripts/quant_engine.py",
		"../scripts/quant_engine.py",
	}
	if d := os.Getenv("SCRIPTS_DIR"); d != "" {
		candidates = append([]string{filepath.Join(d, "quant_engine.py")}, candidates...)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		rel := filepath.Join(execDir, "scripts", "quant_engine.py")
		if _, err := os.Stat(rel); err == nil {
			return rel
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
