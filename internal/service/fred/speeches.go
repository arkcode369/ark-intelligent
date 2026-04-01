package fred

// speeches.go — Federal Reserve speeches & FOMC minutes scraper via Firecrawl.
//
// Fetches the 5 most recent Fed speeches from federalreserve.gov and classifies
// each as HAWKISH, DOVISH, or NEUTRAL using keyword matching.
// Falls back gracefully when FIRECRAWL_API_KEY is not set.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// FedSpeech represents a single Federal Reserve speech or statement.
type FedSpeech struct {
	Title     string    // Speech title
	Speaker   string    // Speaker name (e.g., "Jerome H. Powell")
	Date      time.Time // Speech date
	URL       string    // Original URL on federalreserve.gov
	Tone      string    // "HAWKISH", "DOVISH", "NEUTRAL"
}

// FedSpeechData is the result set returned by FetchRecentSpeeches.
type FedSpeechData struct {
	Speeches  []FedSpeech // Last 5 speeches, most recent first
	Available bool
	FetchedAt time.Time
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

var (
	speechCache   *FedSpeechData
	speechCacheMu sync.RWMutex
	speechCacheTTL = 6 * time.Hour //nolint:gochecknoglobals
)

// GetCachedOrFetchSpeeches returns cached data within TTL or fetches fresh.
func GetCachedOrFetchSpeeches(ctx context.Context) *FedSpeechData {
	speechCacheMu.RLock()
	if speechCache != nil && time.Since(speechCache.FetchedAt) < speechCacheTTL {
		d := speechCache
		speechCacheMu.RUnlock()
		return d
	}
	speechCacheMu.RUnlock()

	fresh := FetchRecentSpeeches(ctx)

	speechCacheMu.Lock()
	speechCache = fresh
	speechCacheMu.Unlock()

	return fresh
}

// InvalidateSpeechCache forces a fresh fetch on next call.
func InvalidateSpeechCache() {
	speechCacheMu.Lock()
	speechCache = nil
	speechCacheMu.Unlock()
}

// ---------------------------------------------------------------------------
// Fetch
// ---------------------------------------------------------------------------

// firecrawlScrapeURL is the Firecrawl v1 scrape endpoint.
const firecrawlScrapeURL = "https://api.firecrawl.dev/v1/scrape"

// fedSpeechesListURL is the Fed speeches listing page.
const fedSpeechesListURL = "https://www.federalreserve.gov/newsevents/speeches.htm"

// FetchRecentSpeeches scrapes the Fed speeches listing page via Firecrawl
// and returns the 5 most recent speeches with metadata and tone classification.
// Falls back gracefully (Available=false) when FIRECRAWL_API_KEY is not set
// or the request fails.
func FetchRecentSpeeches(ctx context.Context) *FedSpeechData {
	result := &FedSpeechData{FetchedAt: time.Now()}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("FedSpeeches: skipping — FIRECRAWL_API_KEY not set")
		return result
	}

	// Build Firecrawl structured extraction request.
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"speeches": {
				"type": "array",
				"maxItems": 5,
				"items": {
					"type": "object",
					"properties": {
						"title":   {"type": "string"},
						"speaker": {"type": "string"},
						"date":    {"type": "string"},
						"url":     {"type": "string"}
					},
					"required": ["title", "speaker", "date"]
				}
			}
		},
		"required": ["speeches"]
	}`)

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

	reqBody := fcReq{
		URL:     fedSpeechesListURL,
		Formats: []string{"json"},
		WaitFor: 3000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract the 5 most recent Federal Reserve speeches from the listing. " +
				"For each speech, extract: title (full speech title), speaker (full name), " +
				"date (as shown, e.g. 'April 1, 2026'), and url (full URL to the speech page if available). " +
				"Return most recent first.",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Debug().Err(err).Msg("FedSpeeches: failed to marshal Firecrawl request")
		return result
	}

	fcClient := &http.Client{Timeout: 45 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlScrapeURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Debug().Err(err).Msg("FedSpeeches: failed to build Firecrawl request")
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("FedSpeeches: Firecrawl request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Debug().Int("status", resp.StatusCode).Msg("FedSpeeches: Firecrawl non-2xx response")
		return result
	}

	// Parse Firecrawl response.
	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				Speeches []struct {
					Title   string `json:"title"`
					Speaker string `json:"speaker"`
					Date    string `json:"date"`
					URL     string `json:"url"`
				} `json:"speeches"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		log.Debug().Err(err).Msg("FedSpeeches: Firecrawl decode failed")
		return result
	}

	if !fcResp.Success {
		log.Debug().Msg("FedSpeeches: Firecrawl returned unsuccessful")
		return result
	}

	raw := fcResp.Data.JSON.Speeches
	if len(raw) == 0 {
		log.Debug().Msg("FedSpeeches: no speeches extracted")
		return result
	}

	for _, s := range raw {
		if s.Title == "" && s.Speaker == "" {
			continue
		}

		speech := FedSpeech{
			Title:   s.Title,
			Speaker: s.Speaker,
			URL:     normalizeURL(s.URL),
			Tone:    ClassifySpeechTone(s.Title),
		}

		// Parse date — try common formats from the Fed site.
		speech.Date = parseFedDate(s.Date)

		result.Speeches = append(result.Speeches, speech)
	}

	if len(result.Speeches) > 0 {
		result.Available = true
		log.Debug().
			Int("count", len(result.Speeches)).
			Msg("FedSpeeches: fetched via Firecrawl")
	}

	return result
}

// ---------------------------------------------------------------------------
// Tone classification
// ---------------------------------------------------------------------------

// hawkishKeywords indicate a tightening/inflation-fighting bias.
var hawkishKeywords = []string{
	"inflation remains elevated",
	"inflation is too high",
	"further tightening",
	"higher for longer",
	"restrictive",
	"rate hike",
	"price stability",
	"reducing inflation",
	"inflation expectations",
	"firm commitment",
	"tighter",
}

// dovishKeywords indicate an easing/growth-supportive bias.
var dovishKeywords = []string{
	"labor market softening",
	"rate cuts",
	"rate cut",
	"easing",
	"accommodate",
	"supporting growth",
	"slowing inflation",
	"inflation returning",
	"balanced",
	"downside risks",
	"softening",
	"appropriate to reduce",
	"policy adjustment",
}

// ClassifySpeechTone classifies a speech title as HAWKISH, DOVISH, or NEUTRAL
// based on keyword matching. Case-insensitive.
func ClassifySpeechTone(title string) string {
	lower := strings.ToLower(title)

	hawkScore := 0
	for _, kw := range hawkishKeywords {
		if strings.Contains(lower, kw) {
			hawkScore++
		}
	}

	dovishScore := 0
	for _, kw := range dovishKeywords {
		if strings.Contains(lower, kw) {
			dovishScore++
		}
	}

	switch {
	case hawkScore > dovishScore:
		return "HAWKISH"
	case dovishScore > hawkScore:
		return "DOVISH"
	default:
		return "NEUTRAL"
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseFedDate attempts to parse common date formats used on federalreserve.gov.
func parseFedDate(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{
		"January 2, 2006",
		"January 02, 2006",
		"Jan. 2, 2006",
		"Jan 2, 2006",
		"2006-01-02",
		"1/2/2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	// Return zero time if unparseable; callers should check.
	return time.Time{}
}

// normalizeURL ensures URL is absolute (prefixes the Fed base URL if relative).
func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "http") {
		return u
	}
	return "https://www.federalreserve.gov" + u
}

// ---------------------------------------------------------------------------
// Prompt integration helpers
// ---------------------------------------------------------------------------

// ToneEmoji returns an emoji for a tone label.
func ToneEmoji(tone string) string {
	switch tone {
	case "HAWKISH":
		return "🦅"
	case "DOVISH":
		return "🕊️"
	default:
		return "⚖️"
	}
}

// OverallFedStance summarises the overall Fed tone from a slice of speeches.
// Returns a short description for use in AI prompts.
func OverallFedStance(speeches []FedSpeech) string {
	if len(speeches) == 0 {
		return "unknown"
	}

	hawk, dove := 0, 0
	for _, s := range speeches {
		switch s.Tone {
		case "HAWKISH":
			hawk++
		case "DOVISH":
			dove++
		}
	}

	total := len(speeches)
	switch {
	case hawk >= total-1 && hawk > dove:
		return "strongly hawkish — inflation remains top priority"
	case hawk > dove:
		return "moderately hawkish — tightening bias persists"
	case dove >= total-1 && dove > hawk:
		return "strongly dovish — easing cycle likely underway"
	case dove > hawk:
		return "moderately dovish — rate cuts on the horizon"
	default:
		return "neutral/mixed — no clear directional bias"
	}
}
