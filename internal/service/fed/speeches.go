// Package fed fetches Federal Reserve communication data from public sources.
//
// Currently supported:
//   - Fed speeches from federalreserve.gov via Firecrawl structured extraction.
//
// If FIRECRAWL_API_KEY is not set, data fetch is skipped gracefully.
// Each struct has an Available flag for callers to check.
package fed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("fed")

// FedSpeech represents a single Federal Reserve speech or remarks event.
type FedSpeech struct {
	Date    string `json:"date"`
	Speaker string `json:"speaker"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Excerpt string `json:"excerpt"`
}

// FedSpeechData holds the latest Fed speeches fetched via Firecrawl.
type FedSpeechData struct {
	Speeches  []FedSpeech
	FetchedAt time.Time
	Available bool
}

// FetchFedSpeeches scrapes the 5 most recent Fed speeches from
// federalreserve.gov/newsevents/speech/ using Firecrawl JSON extraction.
// If FIRECRAWL_API_KEY is not set, returns Available=false with no error.
func FetchFedSpeeches(ctx context.Context) (*FedSpeechData, error) {
	result := &FedSpeechData{FetchedAt: time.Now()}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("Fed speeches: skipping — FIRECRAWL_API_KEY not set")
		return result, nil
	}

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
			"speeches": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"date":    {"type": "string"},
						"speaker": {"type": "string"},
						"title":   {"type": "string"},
						"url":     {"type": "string"},
						"excerpt": {"type": "string"}
					}
				}
			}
		}
	}`)

	reqBody := fcReq{
		URL:     "https://www.federalreserve.gov/newsevents/speech/",
		Formats: []string{"json"},
		WaitFor: 3000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract the 5 most recent Fed speeches or remarks listed on this page. For each entry return: date (as shown), speaker name, title of the speech/remarks, URL (full or relative path), and a short excerpt or description if available (first paragraph or subtitle). Return as an array under the key 'speeches'.",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Debug().Err(err).Msg("Fed speeches: failed to marshal Firecrawl request")
		return result, fmt.Errorf("fed speeches: marshal request: %w", err)
	}

	fcClient := &http.Client{Timeout: 35 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(bodyBytes))
	if err != nil {
		log.Debug().Err(err).Msg("Fed speeches: failed to build Firecrawl request")
		return result, fmt.Errorf("fed speeches: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("Fed speeches: Firecrawl request failed")
		return result, fmt.Errorf("fed speeches: firecrawl request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Int("status", resp.StatusCode).Msg("Fed speeches: Firecrawl non-200 status")
		return result, fmt.Errorf("fed speeches: firecrawl HTTP %d", resp.StatusCode)
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				Speeches []FedSpeech `json:"speeches"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		log.Debug().Err(err).Msg("Fed speeches: Firecrawl decode failed")
		return result, fmt.Errorf("fed speeches: decode response: %w", err)
	}

	if !fcResp.Success {
		log.Debug().Msg("Fed speeches: Firecrawl returned unsuccessful")
		return result, nil
	}

	speeches := fcResp.Data.JSON.Speeches
	if len(speeches) == 0 {
		log.Debug().Msg("Fed speeches: no speeches extracted")
		return result, nil
	}

	// Cap at 5
	if len(speeches) > 5 {
		speeches = speeches[:5]
	}

	result.Speeches = speeches
	result.Available = true
	log.Debug().Int("count", len(speeches)).Msg("Fed speeches fetched via Firecrawl")

	return result, nil
}
