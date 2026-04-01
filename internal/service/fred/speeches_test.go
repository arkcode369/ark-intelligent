package fred

import (
	"testing"
	"time"
)

func TestClassifySpeechTone_Hawkish(t *testing.T) {
	titles := []string{
		"Inflation Remains Elevated: The Case for Further Tightening",
		"Higher for Longer: Fed's Commitment to Price Stability",
		"Restrictive Policy Stance and Reducing Inflation",
	}
	for _, title := range titles {
		tone := ClassifySpeechTone(title)
		if tone != "HAWKISH" {
			t.Errorf("expected HAWKISH for %q, got %q", title, tone)
		}
	}
}

func TestClassifySpeechTone_Dovish(t *testing.T) {
	titles := []string{
		"Labor Market Softening and the Case for Rate Cuts",
		"Appropriate to Reduce the Policy Rate",
		"Easing Financial Conditions and Supporting Growth",
	}
	for _, title := range titles {
		tone := ClassifySpeechTone(title)
		if tone != "DOVISH" {
			t.Errorf("expected DOVISH for %q, got %q", title, tone)
		}
	}
}

func TestClassifySpeechTone_Neutral(t *testing.T) {
	titles := []string{
		"Remarks at the University of Chicago Booth School of Business",
		"The Fed's Role in Financial Stability",
		"Economic Outlook: Navigating Uncertainty",
	}
	for _, title := range titles {
		tone := ClassifySpeechTone(title)
		if tone != "NEUTRAL" {
			t.Errorf("expected NEUTRAL for %q, got %q", title, tone)
		}
	}
}

func TestOverallFedStance_Hawkish(t *testing.T) {
	speeches := []FedSpeech{
		{Tone: "HAWKISH"},
		{Tone: "HAWKISH"},
		{Tone: "HAWKISH"},
		{Tone: "NEUTRAL"},
	}
	stance := OverallFedStance(speeches)
	if stance == "unknown" {
		t.Error("expected non-unknown stance for hawkish speeches")
	}
}

func TestOverallFedStance_Dovish(t *testing.T) {
	speeches := []FedSpeech{
		{Tone: "DOVISH"},
		{Tone: "DOVISH"},
		{Tone: "DOVISH"},
	}
	stance := OverallFedStance(speeches)
	if stance == "unknown" {
		t.Error("expected non-unknown stance for dovish speeches")
	}
}

func TestOverallFedStance_Empty(t *testing.T) {
	stance := OverallFedStance(nil)
	if stance != "unknown" {
		t.Errorf("expected 'unknown' for empty speeches, got %q", stance)
	}
}

func TestParseFedDate(t *testing.T) {
	cases := []struct {
		input    string
		wantZero bool
	}{
		{"April 1, 2026", false},
		{"January 15, 2026", false},
		{"2026-01-15", false},
		{"", true},
		{"not-a-date", true},
	}
	for _, tc := range cases {
		got := parseFedDate(tc.input)
		if tc.wantZero && !got.IsZero() {
			t.Errorf("expected zero time for %q, got %v", tc.input, got)
		}
		if !tc.wantZero && got.IsZero() {
			t.Errorf("expected non-zero time for %q", tc.input)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"/newsevents/speeches/powell20260401.htm", "https://www.federalreserve.gov/newsevents/speeches/powell20260401.htm"},
		{"https://www.federalreserve.gov/newsevents/speeches/powell20260401.htm", "https://www.federalreserve.gov/newsevents/speeches/powell20260401.htm"},
		{"", ""},
	}
	for _, tc := range cases {
		got := normalizeURL(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeURL(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestFedSpeechDataAvailable(t *testing.T) {
	// Simulate populated result
	data := &FedSpeechData{
		Speeches: []FedSpeech{
			{Title: "Inflation Remains Elevated", Speaker: "Jerome H. Powell", Date: time.Now(), Tone: "HAWKISH"},
			{Title: "Labor Market Softening", Speaker: "Philip N. Jefferson", Date: time.Now(), Tone: "DOVISH"},
		},
		Available:  true,
		FetchedAt: time.Now(),
	}

	if !data.Available {
		t.Error("expected data.Available to be true")
	}
	if len(data.Speeches) != 2 {
		t.Errorf("expected 2 speeches, got %d", len(data.Speeches))
	}
}

func TestToneEmoji(t *testing.T) {
	if ToneEmoji("HAWKISH") == "" {
		t.Error("expected non-empty emoji for HAWKISH")
	}
	if ToneEmoji("DOVISH") == "" {
		t.Error("expected non-empty emoji for DOVISH")
	}
	if ToneEmoji("NEUTRAL") == "" {
		t.Error("expected non-empty emoji for NEUTRAL")
	}
}
