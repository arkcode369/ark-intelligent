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

// AAIISentiment holds the latest AAII Investor Sentiment Survey data.
type AAIISentiment struct {
	Date      time.Time // Survey week ending date
	Bullish   float64   // percentage
	Neutral   float64   // percentage
	Bearish   float64   // percentage
	Source    string    // "aaii"
	FetchedAt time.Time
}

// AAIIResponseData wraps AAIISentiment with availability flag for service integration.
type AAIIResponseData struct {
	Data        *AAIISentiment
	Available   bool
	BullBear    float64 // Bull/Bear ratio (>1 = bullish sentiment)
	WeekDate    string  // Survey week ending date (e.g. "3/18/2026")
}

// fcJSONOptsAAII defines the JSON extraction options for Firecrawl.
type fcJSONOptsAAII struct {
	Prompt string          `json:"prompt"`
	Schema json.RawMessage `json:"schema"`
}

// fcReqAAII is the Firecrawl scrape request body.
type fcReqAAII struct {
	URL         string          `json:"url"`
	Formats     []string        `json:"formats"`
	WaitFor     int             `json:"waitFor"`
	JSONOptions *fcJSONOptsAAII `json:"jsonOptions,omitempty"`
}

// aaiiFCResponse models the Firecrawl scrape response for AAII data.
type aaiiFCResponse struct {
	Success bool `json:"success"`
	Data    struct {
		JSON struct {
			LatestWeek  string  `json:"latest_week"`
			BullishPct  float64 `json:"bullish_pct"`
			NeutralPct  float64 `json:"neutral_pct"`
			BearishPct  float64 `json:"bearish_pct"`
		} `json:"json"`
	} `json:"data"`
}

// aaiiFCSchema is the JSON schema for Firecrawl extraction.
var aaiiFCSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"latest_week":  {"type": "string"},
		"bullish_pct":  {"type": "number"},
		"neutral_pct":  {"type": "number"},
		"bearish_pct":  {"type": "number"}
	}
}`)

// FetchAAIISentiment fetches the latest AAII Investor Sentiment Survey via Firecrawl.
// AAII data is behind Imperva bot protection and requires Firecrawl to scrape.
// If FIRECRAWL_API_KEY is not set, returns unavailable.
// Returns data with 24-hour caching recommendation.
func FetchAAIISentiment(ctx context.Context) *AAIIResponseData {
	result := &AAIIResponseData{
		Data:      &AAIISentiment{FetchedAt: time.Now(), Source: "aaii"},
		Available: false,
	}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("AAII: skipping — FIRECRAWL_API_KEY not set")
		return result
	}

	reqBody := fcReqAAII{
		URL:     "https://www.aaii.com/sentimentsurvey",
		Formats: []string{"json"},
		WaitFor: 5000,
		JSONOptions: &fcJSONOptsAAII{
			Prompt: "Extract the latest AAII sentiment survey data: latest week ending date, bullish %, neutral %, and bearish %. Return the week date as a string (e.g., '3/18/2026') and percentages as numbers.",
			Schema: aaiiFCSchema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Warn().Err(err).Msg("AAII: failed to marshal Firecrawl request")
		return result
	}

	// Use a longer timeout for Firecrawl (it needs to render the page)
	fcClient := httpclient.New(httpclient.WithTimeout(30 * time.Second))
	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlScrapeURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Warn().Err(err).Msg("AAII: failed to build Firecrawl request")
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("AAII: Firecrawl request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Msg("AAII: Firecrawl non-2xx response")
		return result
	}

	var fcResp aaiiFCResponse
	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		log.Warn().Err(err).Msg("AAII: Firecrawl decode failed")
		return result
	}

	if !fcResp.Success {
		log.Warn().Msg("AAII: Firecrawl returned unsuccessful")
		return result
	}

	j := fcResp.Data.JSON
	if j.BullishPct == 0 && j.BearishPct == 0 && j.NeutralPct == 0 {
		log.Warn().Msg("AAII: Firecrawl returned empty data")
		return result
	}

	// Parse the week date
	var parsedDate time.Time
	if j.LatestWeek != "" {
		// Try multiple date formats
		formats := []string{"1/2/2006", "01/02/2006", "1/02/2006", "01/2/2006", "2006-01-02"}
		for _, format := range formats {
			if d, err := time.Parse(format, j.LatestWeek); err == nil {
				parsedDate = d
				break
			}
		}
		if parsedDate.IsZero() {
			log.Debug().Str("week_date", j.LatestWeek).Msg("AAII: could not parse week date")
		}
	}

	result.Data.Date = parsedDate
	result.Data.Bullish = j.BullishPct
	result.Data.Neutral = j.NeutralPct
	result.Data.Bearish = j.BearishPct
	result.WeekDate = j.LatestWeek
	if j.BearishPct > 0 {
		result.BullBear = j.BullishPct / j.BearishPct
	}
	result.Available = true

	log.Debug().
		Float64("bullish", result.Data.Bullish).
		Float64("bearish", result.Data.Bearish).
		Float64("neutral", result.Data.Neutral).
		Str("week", result.WeekDate).
		Msg("AAII sentiment fetched via Firecrawl")

	return result
}

// IntegrateAAIIIntoSentiment merges AAII sentiment data into SentimentData.
func IntegrateAAIIIntoSentiment(sd *SentimentData, aaii *AAIIResponseData) {
	if sd == nil || aaii == nil || !aaii.Available || aaii.Data == nil {
		return
	}
	sd.AAIIBullish = aaii.Data.Bullish
	sd.AAIINeutral = aaii.Data.Neutral
	sd.AAIIBearish = aaii.Data.Bearish
	sd.AAIIWeekDate = aaii.WeekDate
	if aaii.BullBear > 0 {
		sd.AAIIBullBear = aaii.BullBear
	}
	sd.AAIIAvailable = true
}

// ClassifyAAIISignal returns a contrarian interpretation of AAII sentiment.
// High bullish sentiment is contrarian bearish (retail is often wrong at extremes).
func ClassifyAAIISignal(bullish, bearish float64) (signal, description string) {
	if bullish == 0 && bearish == 0 {
		return "UNKNOWN", "Insufficient data"
	}

	// Historical AAII extremes (approximate)
	switch {
	case bullish >= 55:
		return "EXTREME GREED", "Very high retail bullishness — contrarian bearish warning"
	case bullish >= 45:
		return "GREED", "Elevated bullish sentiment — mild contrarian bearish"
	case bearish >= 50:
		return "EXTREME FEAR", "Very high bearishness — strong contrarian bullish signal"
	case bearish >= 40:
		return "FEAR", "Elevated bearish sentiment — contrarian bullish"
	default:
		return "NEUTRAL", "Normal sentiment balance"
	}
}
