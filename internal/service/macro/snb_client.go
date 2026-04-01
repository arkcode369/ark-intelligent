package macro

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/errs"
)

// snbHTTPTimeout for SNB API requests.
const snbHTTPTimeout = 20 * time.Second

// snbCacheTTL for SNB data (monthly updates, cache 24h).
const snbCacheTTL = 24 * time.Hour

// snbAPIBase is the SNB REST API base URL.
const snbAPIBase = "https://data.snb.ch/api/cube"

// snbFXAlertThreshold is the month-over-month FX reserve change (in billions CHF)
// that triggers an intervention alert.
const snbFXAlertThreshold = 5.0 // CHF billions

// snbCubeID is the SNB Balance Sheet cube identifier.
const snbCubeID = "snbbipo"

// SNB balance sheet position codes in cube snbbipo.
// These dimension keys correspond to the SNB's published balance sheet items.
const (
	snbPosFXInvestments  = "A2"  // Foreign currency investments
	snbPosSightDeposits  = "L4"  // Sight deposits (commercial banks & govt)
	snbPosGold           = "A1"  // Gold holdings
	snbPosBanknotes      = "L1"  // Banknotes in circulation
)

// SNBData holds the latest SNB balance sheet data.
type SNBData struct {
	FetchedAt time.Time

	// Foreign currency investments (billions CHF) — FX intervention proxy
	FXInvestmentsDate  time.Time
	FXInvestments      float64 // CHF billions
	FXInvestmentsPrev  float64 // previous month
	FXInvestmentsMoM   float64 // month-over-month change

	// Sight deposits (billions CHF) — excess liquidity indicator
	SightDepositsDate  time.Time
	SightDeposits      float64

	// Gold holdings (billions CHF)
	GoldDate  time.Time
	Gold      float64

	// FX intervention alert
	InterventionAlert bool
}

// IsZero returns true if no SNB data has been fetched.
func (d *SNBData) IsZero() bool {
	return d == nil || d.FetchedAt.IsZero()
}

// SNBClient fetches balance sheet data from the SNB REST API.
// No API key required — free public data.
type SNBClient struct {
	mu       sync.Mutex
	cached   *SNBData
	cachedAt time.Time
	hc       *http.Client
}

// NewSNBClient creates a new SNBClient.
func NewSNBClient() *SNBClient {
	return &SNBClient{
		hc: &http.Client{Timeout: snbHTTPTimeout},
	}
}

// GetData returns the latest SNB balance sheet data, using cache if fresh.
func (c *SNBClient) GetData(ctx context.Context) (*SNBData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.cachedAt) < snbCacheTTL {
		return c.cached, nil
	}

	data, err := c.fetchAll(ctx)
	if err != nil {
		if c.cached != nil {
			log.Warn().Err(err).Msg("SNB fetch failed, returning stale cache")
			return c.cached, nil
		}
		return nil, err
	}

	c.cached = data
	c.cachedAt = time.Now()
	return data, nil
}

// snbObservation holds a single time-series observation.
type snbObservation struct {
	date time.Time
	val  float64
}

// fetchAll fetches SNB balance sheet positions concurrently.
func (c *SNBClient) fetchAll(ctx context.Context) (*SNBData, error) {
	type result struct {
		pos  string
		obs  []snbObservation
		err  error
	}

	positions := []string{snbPosFXInvestments, snbPosSightDeposits, snbPosGold}

	results := make(chan result, len(positions))
	for _, pos := range positions {
		pos := pos
		go func() {
			obs, err := c.fetchSeries(ctx, pos)
			results <- result{pos: pos, obs: obs, err: err}
		}()
	}

	data := &SNBData{FetchedAt: time.Now()}
	var fetchErrs []string

	for range positions {
		r := <-results
		if r.err != nil {
			fetchErrs = append(fetchErrs, fmt.Sprintf("%s: %v", r.pos, r.err))
			continue
		}
		if len(r.obs) == 0 {
			fetchErrs = append(fetchErrs, fmt.Sprintf("%s: no observations", r.pos))
			continue
		}

		latest := r.obs[len(r.obs)-1]
		switch r.pos {
		case snbPosFXInvestments:
			data.FXInvestmentsDate = latest.date
			data.FXInvestments = latest.val
			if len(r.obs) >= 2 {
				data.FXInvestmentsPrev = r.obs[len(r.obs)-2].val
				data.FXInvestmentsMoM = latest.val - r.obs[len(r.obs)-2].val
			}
			data.InterventionAlert = math.Abs(data.FXInvestmentsMoM) >= snbFXAlertThreshold
		case snbPosSightDeposits:
			data.SightDepositsDate = latest.date
			data.SightDeposits = latest.val
		case snbPosGold:
			data.GoldDate = latest.date
			data.Gold = latest.val
		}
	}

	allZero := data.FXInvestments == 0 && data.SightDeposits == 0 && data.Gold == 0
	if allZero && len(fetchErrs) > 0 {
		return nil, errs.Wrapf(errs.ErrUpstream, "all SNB series failed: %s", strings.Join(fetchErrs, "; "))
	}
	if len(fetchErrs) > 0 {
		log.Warn().Strs("errors", fetchErrs).Msg("some SNB series failed, returning partial data")
	}

	return data, nil
}

// fetchSeries fetches the last 2 observations for a given SNB balance sheet position.
// Returns observations in chronological order.
func (c *SNBClient) fetchSeries(ctx context.Context, posCode string) ([]snbObservation, error) {
	// SNB cube CSV endpoint with filter on the position dimension
	url := fmt.Sprintf("%s/%s/data/csv/en?lastNObservations=2", snbAPIBase, snbCubeID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/csv")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errs.Wrapf(errs.ErrUpstream, "SNB API status %d for %s", resp.StatusCode, posCode)
	}

	return parseSNBCSV(resp.Body, posCode)
}

// parseSNBCSV parses the SNB cube CSV response and extracts observations for the given position code.
// SNB CSV format: Date;D0;D1;...;Value columns, with position codes as column headers or row dimension.
func parseSNBCSV(r io.Reader, posCode string) ([]snbObservation, error) {
	reader := csv.NewReader(r)
	reader.Comma = ';'
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // variable columns

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Find date column and value column for the given position code.
	dateIdx := -1
	valueIdx := -1
	for i, h := range headers {
		h = strings.TrimSpace(h)
		if strings.EqualFold(h, "Date") || strings.EqualFold(h, "TIME_PERIOD") || h == "" && i == 0 {
			dateIdx = i
		}
		if strings.EqualFold(h, posCode) {
			valueIdx = i
		}
	}

	// If the position code is a row dimension (not a column), scan rows differently.
	// SNB may use wide format (dates as rows, positions as columns)
	// or long format (date + position + value columns).
	if valueIdx == -1 {
		// Try long format: look for "Position" / "D0" column and "Value" column
		posIdx := -1
		for i, h := range headers {
			h = strings.TrimSpace(h)
			switch strings.ToUpper(h) {
			case "D0", "POSITION", "POS", "ITEM":
				posIdx = i
			case "VALUE", "OBS_VALUE":
				valueIdx = i
			}
		}
		if posIdx == -1 || valueIdx == -1 || dateIdx == -1 {
			return nil, errs.Wrapf(errs.ErrInvalidFormat,
				"SNB CSV: cannot locate columns for position %s (headers: %v)", posCode, headers)
		}
		return parseSNBLongFormat(reader, dateIdx, posIdx, valueIdx, posCode)
	}

	if dateIdx == -1 {
		return nil, errs.Wrapf(errs.ErrInvalidFormat, "SNB CSV: no date column (headers: %v)", headers)
	}

	// Wide format: each row is a date, each column is a position.
	var obs []snbObservation
	for {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			continue
		}
		if dateIdx >= len(row) || valueIdx >= len(row) {
			continue
		}
		rawDate := strings.TrimSpace(row[dateIdx])
		rawVal := strings.TrimSpace(row[valueIdx])
		if rawVal == "" || rawVal == "." || rawVal == "NA" {
			continue
		}
		val, parseErr := strconv.ParseFloat(strings.ReplaceAll(rawVal, ",", ""), 64)
		if parseErr != nil {
			continue
		}
		date := parseSNBDate(rawDate)
		if date.IsZero() {
			continue
		}
		obs = append(obs, snbObservation{date: date, val: val / 1000.0}) // millions → billions
	}

	if len(obs) == 0 {
		return nil, errs.Wrap(errs.ErrNoData, "SNB CSV: no valid observations")
	}
	return obs, nil
}

// parseSNBLongFormat handles SNB CSV in long format (date, position, value columns).
func parseSNBLongFormat(reader *csv.Reader, dateIdx, posIdx, valueIdx int, posCode string) ([]snbObservation, error) {
	var obs []snbObservation
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		maxIdx := dateIdx
		if posIdx > maxIdx {
			maxIdx = posIdx
		}
		if valueIdx > maxIdx {
			maxIdx = valueIdx
		}
		if maxIdx >= len(row) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(row[posIdx]), posCode) {
			continue
		}
		rawDate := strings.TrimSpace(row[dateIdx])
		rawVal := strings.TrimSpace(row[valueIdx])
		if rawVal == "" || rawVal == "." || rawVal == "NA" {
			continue
		}
		val, parseErr := strconv.ParseFloat(strings.ReplaceAll(rawVal, ",", ""), 64)
		if parseErr != nil {
			continue
		}
		date := parseSNBDate(rawDate)
		if date.IsZero() {
			continue
		}
		obs = append(obs, snbObservation{date: date, val: val / 1000.0}) // millions → billions
	}
	if len(obs) == 0 {
		return nil, errs.Wrap(errs.ErrNoData, "SNB CSV long format: no observations for "+posCode)
	}
	return obs, nil
}

// parseSNBDate parses SNB date strings: "YYYY-MM" or "YYYY-MM-DD".
func parseSNBDate(s string) time.Time {
	if t, err := time.Parse("2006-01", s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return time.Time{}
}

// ---------------------------------------------------------------------------
// Package-level singleton
// ---------------------------------------------------------------------------

var defaultSNBClient = NewSNBClient()

// GetSNBData returns the latest SNB balance sheet data using the package-level client.
func GetSNBData(ctx context.Context) (*SNBData, error) {
	return defaultSNBClient.GetData(ctx)
}

// FormatSNBData formats SNB balance sheet data for Telegram HTML display.
func FormatSNBData(d *SNBData) string {
	if d == nil || d.IsZero() {
		return "❌ SNB data tidak tersedia."
	}

	var sb strings.Builder
	sb.WriteString("🇨🇭 <b>SNB Balance Sheet — FX Intervention Monitor</b>\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// FX Intervention Alert
	if d.InterventionAlert {
		direction := "↑"
		if d.FXInvestmentsMoM < 0 {
			direction = "↓"
		}
		sb.WriteString(fmt.Sprintf("🚨 <b>ALERT: Potential FX Intervention Detected!</b>\n"))
		sb.WriteString(fmt.Sprintf("   MoM Change: <b>%+.1fB CHF %s</b>\n\n", d.FXInvestmentsMoM, direction))
	}

	// Foreign Currency Investments
	if d.FXInvestments != 0 {
		momStr := ""
		if d.FXInvestmentsMoM != 0 {
			sign := "+"
			if d.FXInvestmentsMoM < 0 {
				sign = ""
			}
			momStr = fmt.Sprintf(" (%s%.1fB MoM)", sign, d.FXInvestmentsMoM)
		}
		sb.WriteString("💱 <b>Foreign Currency Investments</b>\n")
		sb.WriteString(fmt.Sprintf("   <b>%.1fB CHF</b>%s", d.FXInvestments, momStr))
		if !d.FXInvestmentsDate.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", d.FXInvestmentsDate.Format("Jan 2006")))
		}
		sb.WriteString("\n")
		sb.WriteString("   <i>↑ Increase = SNB buying FX (CHF selling / weakening)</i>\n\n")
	}

	// Sight Deposits
	if d.SightDeposits != 0 {
		sb.WriteString("🏦 <b>Sight Deposits</b>\n")
		sb.WriteString(fmt.Sprintf("   <b>%.1fB CHF</b>", d.SightDeposits))
		if !d.SightDepositsDate.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", d.SightDepositsDate.Format("Jan 2006")))
		}
		sb.WriteString("\n")
		sb.WriteString("   <i>Excess liquidity at SNB (bank deposits)</i>\n\n")
	}

	// Gold Holdings
	if d.Gold != 0 {
		sb.WriteString("🥇 <b>Gold Holdings</b>\n")
		sb.WriteString(fmt.Sprintf("   <b>%.1fB CHF</b>", d.Gold))
		if !d.GoldDate.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", d.GoldDate.Format("Jan 2006")))
		}
		sb.WriteString("\n\n")
	}

	// CHF Context
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📝 <b>CHF Interpretation Guide</b>\n")
	sb.WriteString("• FX Inv ↑ = SNB selling CHF → CHF bearish pressure\n")
	sb.WriteString("• FX Inv ↓ = SNB buying CHF → CHF bullish / deflation risk\n")
	sb.WriteString("• Sight Deposits ↑ = Loose monetary conditions\n\n")
	sb.WriteString("<i>Sumber: SNB Data API (data.snb.ch) — data bulanan</i>")

	return sb.String()
}
