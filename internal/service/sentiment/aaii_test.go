package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetchAAIISentiment_Success tests successful Firecrawl response parsing.
func TestFetchAAIISentiment_Success(t *testing.T) {
	// Create mock Firecrawl server
	mockFirecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

		// Verify request body structure
		body, _ := io.ReadAll(r.Body)
		var reqBody fcReqAAII
		err := json.Unmarshal(body, &reqBody)
		require.NoError(t, err)
		assert.Equal(t, "https://www.aaii.com/sentimentsurvey", reqBody.URL)
		assert.Contains(t, reqBody.Formats, "json")
		assert.Equal(t, 5000, reqBody.WaitFor)
		assert.NotNil(t, reqBody.JSONOptions)

		// Return mock response
		response := aaiiFCResponse{
			Success: true,
			Data: struct {
				JSON struct {
					LatestWeek  string  `json:"latest_week"`
					BullishPct  float64 `json:"bullish_pct"`
					NeutralPct  float64 `json:"neutral_pct"`
					BearishPct  float64 `json:"bearish_pct"`
				} `json:"json"`
			}{
				JSON: struct {
					LatestWeek  string  `json:"latest_week"`
					BullishPct  float64 `json:"bullish_pct"`
					NeutralPct  float64 `json:"neutral_pct"`
					BearishPct  float64 `json:"bearish_pct"`
				}{
					LatestWeek: "3/18/2026",
					BullishPct: 45.2,
					NeutralPct: 25.3,
					BearishPct: 29.5,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockFirecrawl.Close()

	// Set environment variable
	os.Setenv("FIRECRAWL_API_KEY", "test-api-key")
	defer os.Unsetenv("FIRECRAWL_API_KEY")

	// Temporarily override the firecrawl URL (via test hook)
	ctx := context.Background()
	result := testFetchAAIISentimentWithURL(ctx, mockFirecrawl.URL)

	// Verify results
	require.NotNil(t, result)
	assert.True(t, result.Available)
	require.NotNil(t, result.Data)
	assert.InDelta(t, 45.2, result.Data.Bullish, 0.01)
	assert.InDelta(t, 25.3, result.Data.Neutral, 0.01)
	assert.InDelta(t, 29.5, result.Data.Bearish, 0.01)
	assert.Equal(t, "3/18/2026", result.WeekDate)
	assert.InDelta(t, 45.2/29.5, result.BullBear, 0.01)
	assert.Equal(t, "aaii", result.Data.Source)
	assert.WithinDuration(t, time.Now(), result.Data.FetchedAt, 5*time.Second)

	// Date should be parsed
	expectedDate, _ := time.Parse("1/2/2006", "3/18/2026")
	assert.Equal(t, expectedDate, result.Data.Date)
}

// TestFetchAAIISentiment_NoAPIKey tests behavior when API key is missing.
func TestFetchAAIISentiment_NoAPIKey(t *testing.T) {
	os.Unsetenv("FIRECRAWL_API_KEY")

	ctx := context.Background()
	result := FetchAAIISentiment(ctx)

	require.NotNil(t, result)
	assert.False(t, result.Available)
	assert.NotNil(t, result.Data)
	assert.Zero(t, result.Data.Bullish)
	assert.Zero(t, result.Data.Bearish)
	assert.Zero(t, result.Data.Neutral)
}

// TestFetchAAIISentiment_EmptyResponse tests handling of empty Firecrawl data.
func TestFetchAAIISentiment_EmptyResponse(t *testing.T) {
	mockFirecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := aaiiFCResponse{
			Success: true,
			Data: struct {
				JSON struct {
					LatestWeek  string  `json:"latest_week"`
					BullishPct  float64 `json:"bullish_pct"`
					NeutralPct  float64 `json:"neutral_pct"`
					BearishPct  float64 `json:"bearish_pct"`
				} `json:"json"`
			}{
				JSON: struct {
					LatestWeek  string  `json:"latest_week"`
					BullishPct  float64 `json:"bullish_pct"`
					NeutralPct  float64 `json:"neutral_pct"`
					BearishPct  float64 `json:"bearish_pct"`
				}{
					LatestWeek: "",
					BullishPct: 0,
					NeutralPct: 0,
					BearishPct: 0,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockFirecrawl.Close()

	os.Setenv("FIRECRAWL_API_KEY", "test-api-key")
	defer os.Unsetenv("FIRECRAWL_API_KEY")

	ctx := context.Background()
	result := testFetchAAIISentimentWithURL(ctx, mockFirecrawl.URL)

	require.NotNil(t, result)
	assert.False(t, result.Available)
}

// TestFetchAAIISentiment_FirecrawlError tests handling of Firecrawl error response.
func TestFetchAAIISentiment_FirecrawlError(t *testing.T) {
	mockFirecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Internal server error",
		})
	}))
	defer mockFirecrawl.Close()

	os.Setenv("FIRECRAWL_API_KEY", "test-api-key")
	defer os.Unsetenv("FIRECRAWL_API_KEY")

	ctx := context.Background()
	result := testFetchAAIISentimentWithURL(ctx, mockFirecrawl.URL)

	require.NotNil(t, result)
	assert.False(t, result.Available)
}

// TestFetchAAIISentiment_UnsuccessfulResponse tests handling of unsuccessful Firecrawl response.
func TestFetchAAIISentiment_UnsuccessfulResponse(t *testing.T) {
	mockFirecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := aaiiFCResponse{
			Success: false,
			Data:    struct {
				JSON struct {
					LatestWeek  string  `json:"latest_week"`
					BullishPct  float64 `json:"bullish_pct"`
					NeutralPct  float64 `json:"neutral_pct"`
					BearishPct  float64 `json:"bearish_pct"`
				} `json:"json"`
			}{},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockFirecrawl.Close()

	os.Setenv("FIRECRAWL_API_KEY", "test-api-key")
	defer os.Unsetenv("FIRECRAWL_API_KEY")

	ctx := context.Background()
	result := testFetchAAIISentimentWithURL(ctx, mockFirecrawl.URL)

	require.NotNil(t, result)
	assert.False(t, result.Available)
}

// TestFetchAAIISentiment_DateParsing tests date parsing with various formats.
func TestFetchAAIISentiment_DateParsing(t *testing.T) {
	testCases := []struct {
		name       string
		weekDate   string
		expectZero bool
	}{
		{"Format_M_D_YYYY", "3/18/2026", false},
		{"Format_MM_DD_YYYY", "03/18/2026", false},
		{"Format_M_DD_YYYY", "3/18/2026", false},
		{"Format_MM_D_YYYY", "03/8/2026", false},
		{"Empty", "", true},
		{"Invalid", "not-a-date", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockFirecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := aaiiFCResponse{
					Success: true,
					Data: struct {
						JSON struct {
							LatestWeek  string  `json:"latest_week"`
							BullishPct  float64 `json:"bullish_pct"`
							NeutralPct  float64 `json:"neutral_pct"`
							BearishPct  float64 `json:"bearish_pct"`
						} `json:"json"`
					}{
						JSON: struct {
							LatestWeek  string  `json:"latest_week"`
							BullishPct  float64 `json:"bullish_pct"`
							NeutralPct  float64 `json:"neutral_pct"`
							BearishPct  float64 `json:"bearish_pct"`
						}{
							LatestWeek: tc.weekDate,
							BullishPct: 40.0,
							NeutralPct: 30.0,
							BearishPct: 30.0,
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			}))
			defer mockFirecrawl.Close()

			os.Setenv("FIRECRAWL_API_KEY", "test-api-key")
			defer os.Unsetenv("FIRECRAWL_API_KEY")

			ctx := context.Background()
			result := testFetchAAIISentimentWithURL(ctx, mockFirecrawl.URL)

			require.NotNil(t, result)
			assert.True(t, result.Available)
			if tc.expectZero {
				assert.True(t, result.Data.Date.IsZero())
			} else {
				assert.False(t, result.Data.Date.IsZero())
			}
		})
	}
}

// TestIntegrateAAIIIntoSentiment tests the integration function.
func TestIntegrateAAIIIntoSentiment(t *testing.T) {
	t.Run("Valid data integration", func(t *testing.T) {
		sd := &SentimentData{}
		aaii := &AAIIResponseData{
			Available: true,
			Data: &AAIISentiment{
				Bullish: 45.0,
				Neutral: 25.0,
				Bearish: 30.0,
			},
			WeekDate: "3/18/2026",
			BullBear: 1.5,
		}

		IntegrateAAIIIntoSentiment(sd, aaii)

		assert.True(t, sd.AAIIAvailable)
		assert.InDelta(t, 45.0, sd.AAIIBullish, 0.01)
		assert.InDelta(t, 25.0, sd.AAIINeutral, 0.01)
		assert.InDelta(t, 30.0, sd.AAIIBearish, 0.01)
		assert.Equal(t, "3/18/2026", sd.AAIIWeekDate)
		assert.InDelta(t, 1.5, sd.AAIIBullBear, 0.01)
	})

	t.Run("Nil SentimentData", func(t *testing.T) {
		aaii := &AAIIResponseData{
			Available: true,
			Data:      &AAIISentiment{Bullish: 45.0},
		}

		// Should not panic
		IntegrateAAIIIntoSentiment(nil, aaii)
	})

	t.Run("Nil AAII data", func(t *testing.T) {
		sd := &SentimentData{}

		IntegrateAAIIIntoSentiment(sd, nil)

		assert.False(t, sd.AAIIAvailable)
	})

	t.Run("Unavailable AAII data", func(t *testing.T) {
		sd := &SentimentData{}
		aaii := &AAIIResponseData{
			Available: false,
			Data:      &AAIISentiment{Bullish: 45.0},
		}

		IntegrateAAIIIntoSentiment(sd, aaii)

		assert.False(t, sd.AAIIAvailable)
	})

	t.Run("Nil Data field", func(t *testing.T) {
		sd := &SentimentData{}
		aaii := &AAIIResponseData{
			Available: true,
			Data:      nil,
		}

		IntegrateAAIIIntoSentiment(sd, aaii)

		assert.False(t, sd.AAIIAvailable)
	})
}

// TestClassifyAAIISignal tests the signal classification function.
func TestClassifyAAIISignal(t *testing.T) {
	testCases := []struct {
		name        string
		bullish     float64
		bearish     float64
		wantSignal  string
		wantDesc    string
	}{
		{
			name:       "Extreme Greed - Very high bullish",
			bullish:    55.0,
			bearish:    20.0,
			wantSignal: "EXTREME GREED",
			wantDesc:   "Very high retail bullishness — contrarian bearish warning",
		},
		{
			name:       "Greed - Elevated bullish",
			bullish:    45.0,
			bearish:    25.0,
			wantSignal: "GREED",
			wantDesc:   "Elevated bullish sentiment — mild contrarian bearish",
		},
		{
			name:       "Extreme Fear - Very high bearish",
			bullish:    25.0,
			bearish:    50.0,
			wantSignal: "EXTREME FEAR",
			wantDesc:   "Very high bearishness — strong contrarian bullish signal",
		},
		{
			name:       "Fear - Elevated bearish",
			bullish:    30.0,
			bearish:    40.0,
			wantSignal: "FEAR",
			wantDesc:   "Elevated bearish sentiment — contrarian bullish",
		},
		{
			name:       "Neutral - Balanced",
			bullish:    35.0,
			bearish:    35.0,
			wantSignal: "NEUTRAL",
			wantDesc:   "Normal sentiment balance",
		},
		{
			name:       "No data",
			bullish:    0,
			bearish:    0,
			wantSignal: "UNKNOWN",
			wantDesc:   "Insufficient data",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signal, desc := ClassifyAAIISignal(tc.bullish, tc.bearish)
			assert.Equal(t, tc.wantSignal, signal)
			assert.Equal(t, tc.wantDesc, desc)
		})
	}
}

// TestAAIISentiment_DataStructure tests the data structure.
func TestAAIISentiment_DataStructure(t *testing.T) {
	now := time.Now()
	sentiment := AAIISentiment{
		Date:      now,
		Bullish:   45.5,
		Neutral:   25.5,
		Bearish:   29.0,
		Source:    "aaii",
		FetchedAt: now,
	}

	assert.WithinDuration(t, now, sentiment.Date, time.Second)
	assert.InDelta(t, 45.5, sentiment.Bullish, 0.01)
	assert.InDelta(t, 25.5, sentiment.Neutral, 0.01)
	assert.InDelta(t, 29.0, sentiment.Bearish, 0.01)
	assert.Equal(t, "aaii", sentiment.Source)
	assert.WithinDuration(t, now, sentiment.FetchedAt, time.Second)
}

// TestAAIIResponse_DataStructure tests the response wrapper.
func TestAAIIResponse_DataStructure(t *testing.T) {
	data := &AAIISentiment{
		Bullish: 40.0,
		Bearish: 30.0,
	}

	response := AAIIResponseData{
		Data:      data,
		Available: true,
		BullBear:  40.0 / 30.0,
		WeekDate:  "3/18/2026",
	}

	assert.Equal(t, data, response.Data)
	assert.True(t, response.Available)
	assert.InDelta(t, 1.333, response.BullBear, 0.01)
	assert.Equal(t, "3/18/2026", response.WeekDate)
}

// testFetchAAIISentimentWithURL is a test helper that uses a custom Firecrawl URL.
// In production code, this would use the real Firecrawl endpoint.
func testFetchAAIISentimentWithURL(ctx context.Context, url string) *AAIIResponseData {
	result := &AAIIResponseData{
		Data:      &AAIISentiment{FetchedAt: time.Now(), Source: "aaii"},
		Available: false,
	}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
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

	bodyBytes, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result
	}

	var fcResp aaiiFCResponse
	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		return result
	}

	if !fcResp.Success {
		return result
	}

	j := fcResp.Data.JSON
	if j.BullishPct == 0 && j.BearishPct == 0 && j.NeutralPct == 0 {
		return result
	}

	var parsedDate time.Time
	if j.LatestWeek != "" {
		formats := []string{"1/2/2006", "01/02/2006", "1/02/2006", "01/2/2006", "2006-01-02"}
		for _, format := range formats {
			if d, err := time.Parse(format, j.LatestWeek); err == nil {
				parsedDate = d
				break
			}
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

	return result
}
