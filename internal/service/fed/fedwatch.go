// Package fed provides Fed-related market data integrations.
// Currently supported:
//   - CME FedWatch implied FOMC rate probabilities via Firecrawl JSON extraction
//
// If FIRECRAWL_API_KEY is not set, FedWatch data is skipped gracefully.
// Cache TTL is 4 hours — FOMC probabilities shift on data releases and Fed speeches.
package fed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("fed")

// FedWatchData holds CME FedWatch market-implied FOMC rate probabilities.
// These represent what the Fed Funds futures market is pricing for upcoming
// FOMC meetings — a key driver of USD directionality.
type FedWatchData struct {
	NextMeetingDate    string    // "2026-05-07" — ISO date of next FOMC
	HoldProbability    float64   // % probability of no rate change
	Cut25Probability   float64   // % probability of 25bp cut
	Cut50Probability   float64   // % probability of 50bp cut
	Hike25Probability  float64   // % probability of 25bp hike
	ImpliedYearEndRate float64   // implied year-end rate in bps from Dec futures
	MeetingCount       int       // number of FOMC meetings through year-end
	Available          bool      // false when fetch failed or API key missing
	FetchedAt          time.Time // when this data was fetched
}

// fedWatchCache is an in-memory cache for FedWatch data with a 4-hour TTL.
var fedWatchCache struct {
	sync.Mutex
	data      *FedWatchData
	fetchedAt time.Time
}

const fedWatchCacheTTL = 4 * time.Hour

// FetchFedWatch fetches CME FedWatch market-implied FOMC probabilities via Firecrawl.
// Returns a populated FedWatchData on success. If FIRECRAWL_API_KEY is not set,
// returns a stub with Available=false and no error. Results are cached for 4 hours.
func FetchFedWatch(ctx context.Context) (*FedWatchData, error) {
	// Check cache first.
	fedWatchCache.Lock()
	if fedWatchCache.data != nil && time.Since(fedWatchCache.fetchedAt) < fedWatchCacheTTL {
		cached := fedWatchCache.data
		fedWatchCache.Unlock()
		log.Debug().Msg("FedWatch: returning cached data")
		return cached, nil
	}
	fedWatchCache.Unlock()

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("FedWatch: skipping — FIRECRAWL_API_KEY not set")
		return &FedWatchData{FetchedAt: time.Now()}, nil
	}

	result, err := fetchFedWatchFromFirecrawl(ctx, apiKey)
	if err != nil {
		log.Warn().Err(err).Msg("FedWatch: fetch failed, returning unavailable stub")
		return &FedWatchData{FetchedAt: time.Now()}, nil
	}

	// Update cache on success.
	if result.Available {
		fedWatchCache.Lock()
		fedWatchCache.data = result
		fedWatchCache.fetchedAt = result.FetchedAt
		fedWatchCache.Unlock()
	}

	return result, nil
}

// fetchFedWatchFromFirecrawl does the actual Firecrawl JSON extraction call.
func fetchFedWatchFromFirecrawl(ctx context.Context, apiKey string) (*FedWatchData, error) {
	result := &FedWatchData{FetchedAt: time.Now()}

	type fcJSONOpts struct {
		Prompt string          `json:"prompt"`
		Schema json.RawMessage `json:"schema"`
	}
	type fcReq struct {
		URL         string      `json:"url"`
		Formats     []string    `json:"formats"`
		WaitFor     int         `json:"waitFor"`
		JSONOptions *fcJSONOpts `json:"jsonOptions,omitempty"`
	}

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"next_meeting_date":     {"type": "string"},
			"hold_probability":      {"type": "number"},
			"cut_25bp_probability":  {"type": "number"},
			"cut_50bp_probability":  {"type": "number"},
			"hike_25bp_probability": {"type": "number"},
			"implied_year_end_rate": {"type": "number"},
			"meeting_count":         {"type": "integer"}
		}
	}`)

	reqBody := fcReq{
		URL:     "https://www.cmegroup.com/markets/interest-rates/cme-fedwatch-tool.html",
		Formats: []string{"json"},
		WaitFor: 5000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract from the CME FedWatch Tool page: " +
				"(1) the date of the next FOMC meeting in ISO format (YYYY-MM-DD), " +
				"(2) the probability percentages for: hold (no change), 25bp cut, 50bp cut, and 25bp hike at the next meeting, " +
				"(3) the year-end implied rate in basis points from December futures, " +
				"(4) the total number of FOMC meetings remaining through year-end. " +
				"Return all probabilities as plain numbers 0-100 (not as decimals).",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal Firecrawl request: %w", err)
	}

	fcClient := &http.Client{Timeout: 45 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build Firecrawl request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := fcClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Firecrawl request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Firecrawl non-2xx status: %d", resp.StatusCode)
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				NextMeetingDate    string  `json:"next_meeting_date"`
				HoldProbability    float64 `json:"hold_probability"`
				Cut25Probability   float64 `json:"cut_25bp_probability"`
				Cut50Probability   float64 `json:"cut_50bp_probability"`
				Hike25Probability  float64 `json:"hike_25bp_probability"`
				ImpliedYearEndRate float64 `json:"implied_year_end_rate"`
				MeetingCount       int     `json:"meeting_count"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		return nil, fmt.Errorf("decode Firecrawl response: %w", err)
	}

	if !fcResp.Success {
		return nil, fmt.Errorf("Firecrawl returned unsuccessful")
	}

	d := fcResp.Data.JSON
	result.NextMeetingDate = d.NextMeetingDate
	result.HoldProbability = d.HoldProbability
	result.Cut25Probability = d.Cut25Probability
	result.Cut50Probability = d.Cut50Probability
	result.Hike25Probability = d.Hike25Probability
	result.ImpliedYearEndRate = d.ImpliedYearEndRate
	result.MeetingCount = d.MeetingCount

	// Mark available only if we got at least some probability data.
	if result.HoldProbability+result.Cut25Probability+result.Cut50Probability+result.Hike25Probability > 0 {
		result.Available = true
		log.Debug().
			Str("next_meeting", result.NextMeetingDate).
			Float64("hold", result.HoldProbability).
			Float64("cut25", result.Cut25Probability).
			Float64("cut50", result.Cut50Probability).
			Float64("hike25", result.Hike25Probability).
			Float64("implied_year_end_bps", result.ImpliedYearEndRate).
			Int("meetings_remaining", result.MeetingCount).
			Msg("FedWatch fetched via Firecrawl")
	} else {
		log.Warn().Msg("FedWatch: Firecrawl returned empty probabilities — check page structure")
	}

	return result, nil
}

// ImpliedCutsText returns a human-readable summary of year-end implied rate cuts.
// E.g. "~1.5 cuts by year-end" based on meeting count and probabilities.
func ImpliedCutsText(d *FedWatchData) string {
	if d == nil || !d.Available || d.MeetingCount == 0 {
		return ""
	}
	// Rough implied cut count: each 25bp cut probability contributes to total easing.
	// This is a simplification; real FedWatch uses full probability path.
	totalCutProb := d.Cut25Probability + d.Cut50Probability*2 // weight 50bp as 2 cuts
	impliedCuts := (totalCutProb / 100.0) * float64(d.MeetingCount)
	if impliedCuts < 0.05 {
		return "Market implies no cuts by year-end"
	}
	return fmt.Sprintf("Market implies ~%.1f cut(s) by year-end (%d meetings)", impliedCuts, d.MeetingCount)
}
