package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// MyfxbookPairSentiment holds retail positioning for a single forex pair.
type MyfxbookPairSentiment struct {
	Symbol   string  // e.g. "EURUSD"
	LongPct  float64 // % of retail traders long (e.g. 32.5)
	ShortPct float64 // % of retail traders short (e.g. 67.5)
	Signal   string  // "CONTRARIAN_BULLISH", "CONTRARIAN_BEARISH", or "NEUTRAL"
}

// MyfxbookData holds retail positioning data for multiple pairs.
type MyfxbookData struct {
	Pairs     []MyfxbookPairSentiment
	Available bool
	FetchedAt time.Time
}

// ContrarianSignal classifies the contrarian signal based on retail positioning.
// Returns "CONTRARIAN_BULLISH" if most retail traders are short (>65%),
// "CONTRARIAN_BEARISH" if most are long (>65%), or "NEUTRAL" otherwise.
func ContrarianSignal(longPct float64) string {
	const threshold = 65.0
	switch {
	case longPct < 100-threshold: // most retail short → contrarian buy
		return "CONTRARIAN_BULLISH"
	case longPct > threshold: // most retail long → contrarian sell
		return "CONTRARIAN_BEARISH"
	default:
		return "NEUTRAL"
	}
}

// FetchMyfxbook scrapes Myfxbook Community Outlook via Firecrawl and returns
// retail positioning data for major forex pairs.
//
// Requires FIRECRAWL_API_KEY environment variable.
// Returns a result with Available=false if the key is missing or scraping fails.
func FetchMyfxbook(ctx context.Context) *MyfxbookData {
	result := &MyfxbookData{FetchedAt: time.Now()}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("Myfxbook: skipping — FIRECRAWL_API_KEY not set")
		return result
	}

	// Firecrawl request structure (same as cboe.go)
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

	// Schema: array of pair objects with symbol, long_pct, short_pct
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pairs": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"symbol":    {"type": "string"},
						"long_pct":  {"type": "number"},
						"short_pct": {"type": "number"}
					}
				}
			}
		}
	}`)

	reqBody := fcReq{
		URL:     "https://www.myfxbook.com/community/outlook",
		Formats: []string{"json"},
		WaitFor: 8000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract the community outlook retail positioning for forex pairs. For each pair (like EURUSD, GBPUSD, USDJPY, AUDUSD, USDCAD, USDCHF, NZDUSD, XAUUSD), extract the symbol name, the long percentage (%), and the short percentage (%). Return as a list called 'pairs'.",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Debug().Err(err).Msg("Myfxbook: failed to marshal Firecrawl request")
		return result
	}

	fcClient := &http.Client{Timeout: 35 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(bodyBytes))
	if err != nil {
		log.Debug().Err(err).Msg("Myfxbook: failed to build Firecrawl request")
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("Myfxbook: Firecrawl request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Int("status", resp.StatusCode).Msg("Myfxbook: Firecrawl non-200 response")
		return result
	}

	// Parse Firecrawl JSON extraction response
	type fcRespPair struct {
		Symbol   string  `json:"symbol"`
		LongPct  float64 `json:"long_pct"`
		ShortPct float64 `json:"short_pct"`
	}
	type fcRespData struct {
		Pairs []fcRespPair `json:"pairs"`
	}
	type fcResp struct {
		Success bool       `json:"success"`
		Data    *fcRespData `json:"data"`
	}

	var parsed fcResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		log.Debug().Err(err).Msg("Myfxbook: failed to parse Firecrawl response")
		return result
	}

	if !parsed.Success || parsed.Data == nil || len(parsed.Data.Pairs) == 0 {
		log.Debug().Msg("Myfxbook: no pairs in response")
		return result
	}

	// Build result
	for _, p := range parsed.Data.Pairs {
		symbol := strings.ToUpper(strings.TrimSpace(p.Symbol))
		if symbol == "" {
			continue
		}

		// Normalize: if only one pct given, compute the other
		longPct := p.LongPct
		shortPct := p.ShortPct
		if longPct == 0 && shortPct > 0 {
			longPct = 100 - shortPct
		} else if shortPct == 0 && longPct > 0 {
			shortPct = 100 - longPct
		}

		if longPct <= 0 || shortPct <= 0 {
			continue
		}

		result.Pairs = append(result.Pairs, MyfxbookPairSentiment{
			Symbol:   symbol,
			LongPct:  longPct,
			ShortPct: shortPct,
			Signal:   ContrarianSignal(longPct),
		})
	}

	if len(result.Pairs) > 0 {
		result.Available = true
	}

	return result
}
