package price

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// --- Twelve Data JSON Parsing ---

func TestParseTwelveDataResponse(t *testing.T) {
	raw := `{
		"meta": {"symbol": "EUR/USD", "interval": "1week", "type": "Forex"},
		"values": [
			{"datetime": "2025-01-06", "open": "1.02850", "high": "1.03420", "low": "1.02100", "close": "1.02450"},
			{"datetime": "2024-12-30", "open": "1.04200", "high": "1.04500", "low": "1.02700", "close": "1.02850"},
			{"datetime": "2024-12-23", "open": "1.04300", "high": "1.04450", "low": "1.03800", "close": "1.04200"}
		],
		"status": "ok"
	}`

	var resp twelveDataResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
	if len(resp.Values) != 3 {
		t.Fatalf("values count = %d, want 3", len(resp.Values))
	}
	if resp.Values[0].Close != "1.02450" {
		t.Errorf("first close = %q, want 1.02450", resp.Values[0].Close)
	}
}

func TestParseTwelveDataError(t *testing.T) {
	raw := `{"code": 429, "message": "Too many requests", "status": "error"}`

	var resp twelveDataResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
	if resp.Code != 429 {
		t.Errorf("code = %d, want 429", resp.Code)
	}
}

// --- Alpha Vantage FX Weekly Parsing ---

func TestParseAVFXWeeklyResponse(t *testing.T) {
	raw := `{
		"Meta Data": {
			"1. Information": "Forex Weekly Prices",
			"2. From Symbol": "EUR",
			"3. To Symbol": "USD"
		},
		"Time Series FX (Weekly)": {
			"2025-01-10": {"1. open": "1.0285", "2. high": "1.0342", "3. low": "1.0210", "4. close": "1.0245"},
			"2025-01-03": {"1. open": "1.0420", "2. high": "1.0450", "3. low": "1.0270", "4. close": "1.0285"}
		}
	}`

	var resp avFXWeeklyResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.TimeSeries) != 2 {
		t.Fatalf("time series count = %d, want 2", len(resp.TimeSeries))
	}
	ohlc, ok := resp.TimeSeries["2025-01-10"]
	if !ok {
		t.Fatal("missing 2025-01-10 entry")
	}
	if ohlc.Close != "1.0245" {
		t.Errorf("close = %q, want 1.0245", ohlc.Close)
	}
}

func TestParseAVFXWeeklyRecords(t *testing.T) {
	raw := `{
		"Meta Data": {},
		"Time Series FX (Weekly)": {
			"2025-01-10": {"1. open": "1.0285", "2. high": "1.0342", "3. low": "1.0210", "4. close": "1.0245"},
			"2025-01-03": {"1. open": "1.0420", "2. high": "1.0450", "3. low": "1.0270", "4. close": "1.0285"},
			"2024-12-27": {"1. open": "1.0430", "2. high": "1.0445", "3. low": "1.0380", "4. close": "1.0420"}
		}
	}`

	f := &Fetcher{}
	mapping := domain.PriceSymbolMapping{
		ContractCode: "099741",
		Currency:     "EUR",
		TwelveData:   "EUR/USD",
	}

	records, err := f.parseAVFXWeekly([]byte(raw), mapping, 2)
	if err != nil {
		t.Fatalf("parseAVFXWeekly: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("records count = %d, want 2 (limited by weeks)", len(records))
	}
	// Should be sorted newest-first
	if len(records) >= 2 && records[0].Date.Before(records[1].Date) {
		t.Errorf("records not sorted newest-first: %s before %s", records[0].Date, records[1].Date)
	}
	if records[0].Source != "alphavantage" {
		t.Errorf("source = %q, want alphavantage", records[0].Source)
	}
}

func TestParseAVRateLimited(t *testing.T) {
	raw := `{"Note": "Thank you for using Alpha Vantage! Our standard API call frequency is 25 calls per day."}`

	f := &Fetcher{}
	mapping := domain.PriceSymbolMapping{ContractCode: "099741"}
	_, err := f.parseAVFXWeekly([]byte(raw), mapping, 10)
	if err == nil {
		t.Fatal("expected error for rate limited response")
	}
}

// --- Alpha Vantage Commodity Parsing ---

func TestParseAVCommodityResponse(t *testing.T) {
	raw := `{
		"name": "West Texas Intermediate",
		"interval": "weekly",
		"unit": "dollars per barrel",
		"data": [
			{"date": "2025-01-10", "value": "73.56"},
			{"date": "2025-01-03", "value": "72.89"},
			{"date": "2024-12-27", "value": "."},
			{"date": "2024-12-20", "value": "71.45"}
		]
	}`

	f := &Fetcher{}
	mapping := domain.PriceSymbolMapping{
		ContractCode: "067651",
		Currency:     "OIL",
		AlphaVantage: domain.AlphaVantageSpec{Function: "WTI"},
	}

	records, err := f.parseAVCommodity([]byte(raw), mapping, 10)
	if err != nil {
		t.Fatalf("parseAVCommodity: %v", err)
	}
	// "." value should be skipped
	if len(records) != 3 {
		t.Errorf("records count = %d, want 3 (skipping '.' entry)", len(records))
	}
	// Commodity records have same value for OHLC
	if records[0].Open != records[0].Close {
		t.Errorf("commodity OHLC mismatch: open=%f close=%f", records[0].Open, records[0].Close)
	}
}

// --- Yahoo Finance Parsing ---

func TestParseYahooResponse(t *testing.T) {
	raw := `{
		"chart": {
			"result": [{
				"meta": {"symbol": "EURUSD=X", "currency": "USD"},
				"timestamp": [1704067200, 1704672000, 1705276800],
				"indicators": {
					"quote": [{
						"open": [1.028, 1.042, 1.043],
						"high": [1.034, 1.045, 1.044],
						"low": [1.021, 1.027, 1.038],
						"close": [1.024, 1.028, 1.042],
						"volume": [null, null, null]
					}]
				}
			}],
			"error": null
		}
	}`

	var resp yahooChartResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Chart.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Chart.Error.Code)
	}
	if len(resp.Chart.Result) != 1 {
		t.Fatalf("result count = %d, want 1", len(resp.Chart.Result))
	}
	result := resp.Chart.Result[0]
	if len(result.Timestamp) != 3 {
		t.Errorf("timestamp count = %d, want 3", len(result.Timestamp))
	}
	quote := result.Indicators.Quote[0]
	if len(quote.Close) != 3 {
		t.Errorf("close count = %d, want 3", len(quote.Close))
	}
}

func TestParseYahooNullValues(t *testing.T) {
	raw := `{
		"chart": {
			"result": [{
				"meta": {"symbol": "EURUSD=X"},
				"timestamp": [1704067200, 1704672000],
				"indicators": {
					"quote": [{
						"open": [1.028, null],
						"high": [1.034, null],
						"low": [1.021, null],
						"close": [1.024, null],
						"volume": [null, null]
					}]
				}
			}],
			"error": null
		}
	}`

	var resp yahooChartResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	quote := resp.Chart.Result[0].Indicators.Quote[0]
	if quote.Close[1] != nil {
		t.Error("expected nil for second close value")
	}
	if quote.Open[0] == nil {
		t.Error("expected non-nil for first open value")
	}
}

func TestParseYahooError(t *testing.T) {
	raw := `{"chart": {"result": null, "error": {"code": "Not Found", "description": "No data found"}}}`

	var resp yahooChartResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Chart.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Chart.Error.Code != "Not Found" {
		t.Errorf("error code = %q, want 'Not Found'", resp.Chart.Error.Code)
	}
}

// --- Helpers ---

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"1.0245", 1.0245},
		{"73.56", 73.56},
		{"0", 0},
		{"", 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		got := parseFloat(tt.input)
		if got != tt.want {
			t.Errorf("parseFloat(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestDerefFloat(t *testing.T) {
	v := 1.5
	if got := derefFloat(&v); got != 1.5 {
		t.Errorf("derefFloat(&1.5) = %f", got)
	}
	if got := derefFloat(nil); got != 0 {
		t.Errorf("derefFloat(nil) = %f, want 0", got)
	}
}

func TestSafeIndex(t *testing.T) {
	v := 1.0
	s := []*float64{&v, nil}

	if got := safeIndex(s, 0); got != &v {
		t.Error("safeIndex(s, 0) should return &v")
	}
	if got := safeIndex(s, 1); got != nil {
		t.Error("safeIndex(s, 1) should return nil (nil element)")
	}
	if got := safeIndex(s, 5); got != nil {
		t.Error("safeIndex(s, 5) should return nil (out of bounds)")
	}
}

func TestSortRecordsByDate(t *testing.T) {
	records := []domain.PriceRecord{
		{Date: mustParseDate("2025-01-01")},
		{Date: mustParseDate("2025-01-15")},
		{Date: mustParseDate("2025-01-08")},
	}
	sortRecordsByDate(records)
	if records[0].Date.After(records[1].Date) && records[1].Date.After(records[2].Date) {
		// Good — newest first
	} else {
		t.Errorf("not sorted newest-first: %s, %s, %s",
			records[0].Date.Format("2006-01-02"),
			records[1].Date.Format("2006-01-02"),
			records[2].Date.Format("2006-01-02"),
		)
	}
}

func mustParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
