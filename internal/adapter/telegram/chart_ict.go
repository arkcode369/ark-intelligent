package telegram

// chart_ict.go — ICT chart generation: prepares data and invokes ict_chart.py.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	ictsvc "github.com/arkcode369/ark-intelligent/internal/service/ict"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ictChartInput is the JSON structure passed to the Python ICT chart renderer.
type ictChartInput struct {
	Symbol          string              `json:"symbol"`
	Timeframe       string              `json:"timeframe"`
	Bars            []chartBar          `json:"bars"`
	FVGZones        []ictChartFVG       `json:"fvg_zones"`
	OrderBlocks     []ictChartOB        `json:"order_blocks"`
	Structure       []ictChartStruct    `json:"structure"`
	Sweeps          []ictChartSweep     `json:"sweeps"`
	OTE             []ictChartOTE       `json:"ote"`
	Equilibrium     float64             `json:"equilibrium"`
	PremiumZone     bool                `json:"premium_zone"`
	DiscountZone    bool                `json:"discount_zone"`
	CurrentPrice    float64             `json:"current_price"`
	LiquidityLevels []ictChartLiqLevel  `json:"liquidity_levels"`
	KillzoneBoxes   []ictChartKZBox     `json:"killzone_boxes"`
	DWMPivots       []ictChartDWMPivot  `json:"dwm_pivots"`
	SilverBullets   []ictChartSB        `json:"silver_bullets"`
	AMD             []ictChartAMD       `json:"amd"`
	RelevantHigh    *ictChartAnchor     `json:"relevant_high"`
	RelevantLow     *ictChartAnchor     `json:"relevant_low"`
}

type ictChartFVG struct {
	Type     string  `json:"type"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	BarIndex int     `json:"bar_index"`
	Filled   bool    `json:"filled"`
	FillPct  float64 `json:"fill_pct"`
}

type ictChartOB struct {
	Type     string  `json:"type"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	BarIndex int     `json:"bar_index"`
	Broken   bool    `json:"broken"`
}

type ictChartStruct struct {
	Type      string  `json:"type"`
	Direction string  `json:"direction"`
	Level     float64 `json:"level"`
	BarIndex  int     `json:"bar_index"`
}

type ictChartSweep struct {
	Type     string  `json:"type"`
	Level    float64 `json:"level"`
	BarIndex int     `json:"bar_index"`
	Reversed bool    `json:"reversed"`
}

type ictChartOTE struct {
	Direction string  `json:"direction"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Midpoint  float64 `json:"midpoint"`
}

type ictChartLiqLevel struct {
	Price float64 `json:"price"`
	Type  string  `json:"type"`
	Count int     `json:"count"`
	Swept bool    `json:"swept"`
}

type ictChartKZBox struct {
	Name      string  `json:"name"`
	StartUTC  int     `json:"start_utc"`
	EndUTC    int     `json:"end_utc"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Mitigated bool    `json:"mitigated"`
	Date      string  `json:"date"`
}

type ictChartDWMPivot struct {
	Type        string  `json:"type"`
	Level       float64 `json:"level"`
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
	Broken      bool    `json:"broken"`
}

type ictChartSB struct {
	Window   string  `json:"window"`
	StartUTC int     `json:"start_utc"`
	EndUTC   int     `json:"end_utc"`
	Active   bool    `json:"active"`
	FVGIndex int     `json:"fvg_index"`
	Date     string  `json:"date"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
}

type ictChartAMD struct {
	Session   string  `json:"session"`
	TF        string  `json:"tf"`
	Phase     string  `json:"phase"`
	Direction string  `json:"direction"`
	RangeHigh float64 `json:"range_high"`
	RangeLow  float64 `json:"range_low"`
	AccHigh   float64 `json:"acc_high"`
	AccLow    float64 `json:"acc_low"`
}

type ictChartAnchor struct {
	Type        string  `json:"type"`
	Level       float64 `json:"level"`
	BarIndex    int     `json:"bar_index"`
	ATRMultiple float64 `json:"atr_multiple"`
}

// generateICTChart creates an ICT analysis chart image (PNG) from the analysis result.
// It marshals the data to JSON, invokes ict_chart.py, and returns the PNG bytes.
func generateICTChart(ctx context.Context, symbol, timeframe string, bars []ta.OHLCV, result *ictsvc.ICTResult) ([]byte, error) {
	if result == nil || len(bars) == 0 {
		return nil, fmt.Errorf("no data for ICT chart")
	}

	// Convert bars to chart format (newest-first → oldest-first).
	n := len(bars)
	chartBars := make([]chartBar, n)
	for i, b := range bars {
		chartBars[n-1-i] = chartBar{
			Date:   b.Date.Format(time.RFC3339),
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		}
	}

	// Convert FVG zones.
	fvgZones := make([]ictChartFVG, len(result.FVGZones))
	for i, z := range result.FVGZones {
		fvgZones[i] = ictChartFVG{
			Type: z.Type, High: z.High, Low: z.Low,
			BarIndex: z.BarIndex, Filled: z.Filled, FillPct: z.FillPct,
		}
	}

	// Convert Order Blocks.
	obs := make([]ictChartOB, len(result.OrderBlocks))
	for i, ob := range result.OrderBlocks {
		obs[i] = ictChartOB{
			Type: ob.Type, High: ob.High, Low: ob.Low,
			BarIndex: ob.BarIndex, Broken: ob.Broken,
		}
	}

	// Convert Structure events.
	structs := make([]ictChartStruct, len(result.Structure))
	for i, ev := range result.Structure {
		structs[i] = ictChartStruct{
			Type: ev.Type, Direction: ev.Direction,
			Level: ev.Level, BarIndex: ev.BarIndex,
		}
	}

	// Convert Sweeps.
	sweeps := make([]ictChartSweep, len(result.Sweeps))
	for i, s := range result.Sweeps {
		sweeps[i] = ictChartSweep{
			Type: s.Type, Level: s.Level,
			BarIndex: s.BarIndex, Reversed: s.Reversed,
		}
	}

	// Convert OTE zones.
	otes := make([]ictChartOTE, len(result.OTE))
	for i, o := range result.OTE {
		otes[i] = ictChartOTE{
			Direction: o.Direction, High: o.High,
			Low: o.Low, Midpoint: o.Midpoint,
		}
	}

	// Convert Liquidity Levels.
	lls := make([]ictChartLiqLevel, len(result.LiquidityLevels))
	for i, ll := range result.LiquidityLevels {
		lls[i] = ictChartLiqLevel{
			Price: ll.Price, Type: ll.Type,
			Count: ll.Count, Swept: ll.Swept,
		}
	}

	input := ictChartInput{
		Symbol:          symbol,
		Timeframe:       timeframe,
		Bars:            chartBars,
		FVGZones:        fvgZones,
		OrderBlocks:     obs,
		Structure:       structs,
		Sweeps:          sweeps,
		OTE:             otes,
		Equilibrium:     result.Equilibrium,
		PremiumZone:     result.PremiumZone,
		DiscountZone:    result.DiscountZone,
		CurrentPrice:    result.CurrentPrice,
		LiquidityLevels: lls,
	}

	// Convert Killzone Boxes.
	for _, kz := range result.KillzoneBoxes {
		input.KillzoneBoxes = append(input.KillzoneBoxes, ictChartKZBox{
			Name:      kz.Name,
			StartUTC:  kz.StartUTC,
			EndUTC:    kz.EndUTC,
			High:      kz.High,
			Low:       kz.Low,
			Mitigated: kz.Mitigated,
			Date:      kz.Date.Format("2006-01-02"),
		})
	}

	// Convert DWM Pivots.
	for _, p := range result.DWMPivots {
		input.DWMPivots = append(input.DWMPivots, ictChartDWMPivot{
			Type:        p.Type,
			Level:       p.Level,
			PeriodStart: p.PeriodStart.Format("2006-01-02"),
			PeriodEnd:   p.PeriodEnd.Format("2006-01-02"),
			Broken:      p.Broken,
		})
	}

	// Convert Silver Bullets.
	for _, sb := range result.SilverBullets {
		input.SilverBullets = append(input.SilverBullets, ictChartSB{
			Window:   sb.Window,
			StartUTC: sb.StartUTC,
			EndUTC:   sb.EndUTC,
			Active:   sb.Active,
			FVGIndex: sb.FVGIndex,
			Date:     sb.Date.Format("2006-01-02"),
			High:     sb.High,
			Low:      sb.Low,
		})
	}

	// Convert AMD phases.
	for _, amd := range result.AMD {
		input.AMD = append(input.AMD, ictChartAMD{
			Session:   amd.Session,
			TF:        amd.TF,
			Phase:     amd.Phase,
			Direction: amd.Direction,
			RangeHigh: amd.RangeHigh,
			RangeLow:  amd.RangeLow,
			AccHigh:   amd.AccHigh,
			AccLow:    amd.AccLow,
		})
	}

	// Convert Relevant Anchors.
	if result.RelevantHigh != nil {
		input.RelevantHigh = &ictChartAnchor{
			Type:        result.RelevantHigh.Type,
			Level:       result.RelevantHigh.Level,
			BarIndex:    result.RelevantHigh.BarIndex,
			ATRMultiple: result.RelevantHigh.ATRMultiple,
		}
	}
	if result.RelevantLow != nil {
		input.RelevantLow = &ictChartAnchor{
			Type:        result.RelevantLow.Type,
			Level:       result.RelevantLow.Level,
			BarIndex:    result.RelevantLow.BarIndex,
			ATRMultiple: result.RelevantLow.ATRMultiple,
		}
	}

	return runICTChartScript(ctx, input)
}

// runICTChartScript marshals input to JSON, runs ict_chart.py, and returns PNG bytes.
func runICTChartScript(ctx context.Context, input ictChartInput) ([]byte, error) {
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal ICT chart input: %w", err)
	}

	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("ict_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("ict_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write ICT chart input: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	scriptPath, err := findICTChartScript()
	if err != nil {
		return nil, err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Str("stderr", stderr.String()).Msg("ICT chart renderer failed")
		return nil, fmt.Errorf("ICT chart renderer failed: %w", err)
	}

	pngData, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read ICT chart output: %w", err)
	}
	if len(pngData) == 0 {
		return nil, fmt.Errorf("ICT chart renderer produced empty output")
	}

	return pngData, nil
}

// findICTChartScript locates the ict_chart.py script.
func findICTChartScript() (string, error) {
	candidates := []string{
		"scripts/ict_chart.py",
		"../scripts/ict_chart.py",
	}
	if d := os.Getenv("SCRIPTS_DIR"); d != "" {
		candidates = append([]string{filepath.Join(d, "ict_chart.py")}, candidates...)
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}

	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		rel := filepath.Join(execDir, "scripts", "ict_chart.py")
		if _, err := os.Stat(rel); err == nil {
			return rel, nil
		}
		rel = filepath.Join(execDir, "..", "scripts", "ict_chart.py")
		if _, err := os.Stat(rel); err == nil {
			abs, _ := filepath.Abs(rel)
			return abs, nil
		}
	}

	return "", fmt.Errorf("ict_chart.py not found (searched: %v)", candidates)
}
