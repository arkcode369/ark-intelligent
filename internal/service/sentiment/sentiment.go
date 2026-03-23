// Package sentiment fetches investor sentiment survey data from public sources.
//
// Currently supported:
//   - CNN Fear & Greed Index (daily, 0-100 scale)
//   - AAII Investor Sentiment Survey (weekly, bull/bear/neutral %)
//
// Neither source offers a stable, documented API, so this package uses
// lightweight HTTP scraping with graceful degradation: if a source is
// unreachable or changes format, the corresponding Available flag is false
// and the rest of the system continues to work.
package sentiment

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("sentiment")

// SentimentData holds the latest readings from all sentiment sources.
type SentimentData struct {
	// AAII Investor Sentiment Survey
	AAIIBullish   float64 // % bullish
	AAIIBearish   float64 // % bearish
	AAIINeutral   float64 // % neutral
	AAIIBullBear  float64 // Bull/Bear ratio (>1 = bullish sentiment)
	AAIIAvailable bool

	// CNN Fear & Greed Index
	CNNFearGreed      float64 // 0-100 (0=Extreme Fear, 100=Extreme Greed)
	CNNFearGreedLabel string  // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
	CNNAvailable      bool

	FetchedAt time.Time
}

// FetchSentiment fetches sentiment data from all supported sources.
// Individual source failures are logged but do not cause an overall error;
// callers should check the Available flags on the returned data.
func FetchSentiment(ctx context.Context) (*SentimentData, error) {
	data := &SentimentData{FetchedAt: time.Now()}
	client := &http.Client{Timeout: 15 * time.Second}

	// Fetch CNN Fear & Greed
	fetchCNNFearGreed(ctx, client, data)

	// Fetch AAII Sentiment
	fetchAAIISentiment(ctx, client, data)

	return data, nil
}

// ---------------------------------------------------------------------------
// CNN Fear & Greed Index
// ---------------------------------------------------------------------------

// cnnFearGreedURL is the public JSON endpoint for the CNN Fear & Greed data.
const cnnFearGreedURL = "https://production.dataviz.cnn.io/index/fearandgreed/graphdata"

// cnnResponse models the relevant portion of the CNN Fear & Greed JSON response.
type cnnResponse struct {
	FearAndGreed struct {
		Score       float64 `json:"score"`
		Rating      string  `json:"rating"`
		Timestamp   string  `json:"timestamp"`
		PreviousClose float64 `json:"previous_close"`
		Previous1Week float64 `json:"previous_1_week"`
		Previous1Month float64 `json:"previous_1_month"`
		Previous1Year  float64 `json:"previous_1_year"`
	} `json:"fear_and_greed"`
}

func fetchCNNFearGreed(ctx context.Context, client *http.Client, data *SentimentData) {
	req, err := http.NewRequestWithContext(ctx, "GET", cnnFearGreedURL, nil)
	if err != nil {
		log.Warn().Err(err).Msg("CNN F&G: failed to build request")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ArkIntelligent/1.0)")
	req.Header.Set("Referer", "https://www.cnn.com/markets/fear-and-greed")

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("CNN F&G: request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Msg("CNN F&G: non-2xx response")
		return
	}

	var result cnnResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Warn().Err(err).Msg("CNN F&G: decode failed")
		return
	}

	data.CNNFearGreed = result.FearAndGreed.Score
	data.CNNFearGreedLabel = normalizeFearGreedLabel(result.FearAndGreed.Rating)
	data.CNNAvailable = true

	log.Debug().
		Float64("score", data.CNNFearGreed).
		Str("label", data.CNNFearGreedLabel).
		Msg("CNN F&G fetched")
}

// normalizeFearGreedLabel normalizes the CNN rating string to a display label.
func normalizeFearGreedLabel(rating string) string {
	switch strings.ToLower(strings.TrimSpace(rating)) {
	case "extreme fear":
		return "Extreme Fear"
	case "fear":
		return "Fear"
	case "neutral":
		return "Neutral"
	case "greed":
		return "Greed"
	case "extreme greed":
		return "Extreme Greed"
	default:
		if rating != "" {
			return rating
		}
		return "Unknown"
	}
}

// ---------------------------------------------------------------------------
// AAII Investor Sentiment Survey
// ---------------------------------------------------------------------------

func fetchAAIISentiment(_ context.Context, _ *http.Client, data *SentimentData) {
	// AAII is behind Imperva bot protection and cannot be scraped with
	// simple HTTP requests. Browser automation would be required.
	log.Debug().Msg("AAII: skipping — site is behind Imperva bot protection and requires browser automation")
	data.AAIIAvailable = false
}
