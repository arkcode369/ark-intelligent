package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// AAIISentiment holds the latest AAII sentiment survey data.
type AAIISentiment struct {
	Date      time.Time // Survey week ending date
	Bullish   float64   // Bullish percentage
	Neutral   float64   // Neutral percentage
	Bearish   float64   // Bearish percentage
	Available bool      // Whether data was successfully fetched
	FetchedAt time.Time
}

// AAIIFetcher provides methods to fetch and cache AAII sentiment data.
type AAIIFetcher struct {
	httpClient *http.Client
}

// NewAAIIFetcher creates a new AAII fetcher with default HTTP client.
func NewAAIIFetcher() *AAIIFetcher {
	return &AAIIFetcher{
		httpClient: httpclient.New(httpclient.WithTimeout(30 * time.Second)),
	}
}

// FirecrawlAAIIRequest represents the Firecrawl API request structure for AAII.
type FirecrawlAAIIRequest struct {
	URL         string          `json:"url"`
	Formats     []string        `json:"formats"`
	WaitFor     int             `json:"waitFor"`
	JSONOptions *FCJSONOptions  `json:"jsonOptions,omitempty"`
}

// FirecrawlAAIIResponse represents the Firecrawl API response structure for AAII.
type FirecrawlAAIIResponse struct {
	Success bool `json:"success"`
	Data    struct {
		JSON struct {
			LatestWeek   string  `json:"latest_week"`
			BullishPct   float64 `json:"bullish_pct"`
			NeutralPct   float64 `json:"neutral_pct"`
			BearishPct   float64 `json:"bearish_pct"`
		} `json:"json"`
	} `json:"data"`
}

// FCJSONOptions holds JSON extraction options for Firecrawl.
type FCJSONOptions struct {
	Prompt string          `json:"prompt"`
	Schema json.RawMessage `json:"schema"`
}

// aaiiJSONSchema defines the expected JSON schema for AAII extraction.
var aaiiJSONSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"latest_week": {"type": "string"},
		"bullish_pct": {"type": "number"},
		"neutral_pct": {"type": "number"},
		"bearish_pct": {"type": "number"}
	}
}`)

const (
	aaiiURL = "https://www.aaii.com/sentimentsurvey"
)

// Fetch retrieves AAII sentiment data via Firecrawl.
// If FIRECRAWL_API_KEY is not set, returns unavailable data.
func (f *AAIIFetcher) Fetch(ctx context.Context) *AAIISentiment {
	result := &AAIISentiment{
		FetchedAt: time.Now(),
	}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Str("source", "aaii").Msg("AAII: skipping — FIRECRAWL_API_KEY not set")
		return result
	}

	reqBody := FirecrawlAAIIRequest{
		URL:     aaiiURL,
		Formats: []string{"json"},
		WaitFor: 5000,
		JSONOptions: &FCJSONOptions{
			Prompt: "Extract the latest AAII sentiment survey data: latest week ending date, bullish %, neutral %, and bearish %.",
			Schema: aaiiJSONSchema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Warn().Str("source", "aaii").Err(err).Msg("AAII: failed to marshal Firecrawl request")
		return result
	}

	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlScrapeURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Warn().Str("source", "aaii").Err(err).Msg("AAII: failed to build Firecrawl request")
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := f.httpClient.Do(req)
	if err != nil {
		log.Warn().Str("source", "aaii").Err(err).Msg("AAII: Firecrawl request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Str("source", "aaii").Int("status", resp.StatusCode).Msg("AAII: Firecrawl non-2xx response")
		return result
	}

	var fcResp FirecrawlAAIIResponse
	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		log.Warn().Str("source", "aaii").Err(err).Msg("AAII: Firecrawl decode failed")
		return result
	}

	if !fcResp.Success || fcResp.Data.JSON.BullishPct == 0 {
		log.Warn().Str("source", "aaii").Msg("AAII: Firecrawl returned empty or failed result")
		return result
	}

	j := fcResp.Data.JSON
	result.Bullish = j.BullishPct
	result.Neutral = j.NeutralPct
	result.Bearish = j.BearishPct
	result.Available = true

	// Parse date from string
	if j.LatestWeek != "" {
		if parsed, err := time.Parse("2006-01-02", j.LatestWeek); err == nil {
			result.Date = parsed
		}
	}

	log.Debug().
		Float64("bullish", result.Bullish).
		Float64("bearish", result.Bearish).
		Float64("neutral", result.Neutral).
		Str("week", j.LatestWeek).
		Msg("AAII fetched via Firecrawl")

	return result
}

// FetchAAII is a package-level convenience function that uses the default fetcher.
func FetchAAII(ctx context.Context) *AAIISentiment {
	return NewAAIIFetcher().Fetch(ctx)
}

// BullBearRatio calculates the bull-to-bear ratio.
// Returns 0 if bearish is 0 to avoid division by zero.
func (a *AAIISentiment) BullBearRatio() float64 {
	if a.Bearish == 0 {
		return 0
	}
	return a.Bullish / a.Bearish
}

// IsValid returns true if the sentiment data is available and percentages sum to approximately 100.
func (a *AAIISentiment) IsValid() bool {
	if !a.Available {
		return false
	}
	total := a.Bullish + a.Neutral + a.Bearish
	// Allow for some rounding error (should sum to ~100)
	return total >= 99 && total <= 101
}

// SentimentSignal returns a contrarian interpretation of AAII sentiment.
// High bullish sentiment is a bearish contrarian signal (retail is euphoric).
// High bearish sentiment is a bullish contrarian signal (retal is fearful).
func (a *AAIISentiment) SentimentSignal() (signal, description string) {
	switch {
	case a.Bullish >= 55:
		return "EXTREME OPTIMISM", "Very high bullish sentiment — contrarian bearish warning (retail euphoria)"
	case a.Bullish >= 45:
		return "OPTIMISM", "Above-average bullish sentiment — mild contrarian bearish"
	case a.Bearish >= 45:
		return "EXTREME FEAR", "Very high bearish sentiment — strong contrarian bullish signal"
	case a.Bearish >= 35:
		return "FEAR", "Above-average bearish sentiment — contrarian bullish"
	default:
		return "NEUTRAL", "Balanced sentiment — no strong contrarian signal"
	}
}

// IntegrateIntoSentimentData merges AAII data into the main SentimentData struct.
func IntegrateAAIIIntoSentiment(sd *SentimentData, aaii *AAIISentiment) {
	if sd == nil || aaii == nil || !aaii.Available {
		return
	}
	sd.AAIIBullish = aaii.Bullish
	sd.AAIINeutral = aaii.Neutral
	sd.AAIIBearish = aaii.Bearish
	sd.AAIIWeekDate = aaii.Date.Format("2006-01-02")
	sd.AAIIBullBear = aaii.BullBearRatio()
	sd.AAIIAvailable = true
}
