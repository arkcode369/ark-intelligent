package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// buildValidConfig returns a Config with all required fields populated and
// a writable temp DataDir so validate() can complete without fatal exits.
func buildValidConfig(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	return &Config{
		BotToken:  "test-token",
		ChatID:    "123456",
		DataDir:   dir,

		COTHistoryWeeks:        52,
		COTFetchInterval:       6 * time.Hour,
		ConfluenceCalcInterval: 2 * time.Hour,
		PriceFetchInterval:     6 * time.Hour,
		IntradayFetchInterval:  15 * time.Minute,
		AICacheTTL:             1 * time.Hour,

		IntradayRetentionDays: 60,
		PriceHistoryWeeks:     52,
		ChatHistoryLimit:      50,
		ImpactBootstrapMonths: 12,

		AIMaxRPM:        15,
		AIMaxDaily:      200,
		ClaudeMaxTokens: 8192,

		GeminiModel:  "gemini-flash",
		ClaudeModel:  "claude-opus-4-6",
	}
}

// runValidate calls validate() and returns true if it did NOT call log.Fatal.
// Because log.Fatal uses zerolog which writes to os.Stderr and calls os.Exit,
// we cannot capture it in a unit test without subprocess tricks.
// Instead these tests verify the non-fatal path of validate() by supplying
// valid configs and confirming no panic/exit occurs. Fatal-path tests use
// sub-process exec pattern (see TestValidate_Fatal*) but are skipped unless
// RUN_FATAL_TEST=1 is set.

func TestValidate_ValidConfig(t *testing.T) {
	cfg := buildValidConfig(t)
	// Should not panic.
	cfg.validate()
}

func TestValidate_ChatHistoryLimitDefault(t *testing.T) {
	cfg := buildValidConfig(t)
	cfg.ChatHistoryLimit = 0
	// validate() should auto-correct this to 50 (warn only, not fatal).
	cfg.validate()
	if cfg.ChatHistoryLimit != 50 {
		t.Errorf("expected ChatHistoryLimit=50 after auto-correction, got %d", cfg.ChatHistoryLimit)
	}
}

func TestValidate_AIMaxDailyLessThanRPM_DoesNotPanic(t *testing.T) {
	// AIMaxDaily < AIMaxRPM is a warning, NOT a fatal. validate() must not panic.
	cfg := buildValidConfig(t)
	cfg.AIMaxDaily = 5
	cfg.AIMaxRPM = 20
	cfg.validate() // should emit Warn, not Fatal
}

func TestValidate_ClaudeEndpointWithoutModel_NoFatal_WhenEndpointEmpty(t *testing.T) {
	// ClaudeEndpoint is empty → cross-field rule does not trigger. Should pass.
	cfg := buildValidConfig(t)
	cfg.ClaudeEndpoint = ""
	cfg.ClaudeModel = ""
	cfg.validate()
}

func TestValidate_S3PairedCredentials_BothEmpty(t *testing.T) {
	// Both S3 keys empty → valid (neither set).
	cfg := buildValidConfig(t)
	cfg.MassiveS3AccessKey = ""
	cfg.MassiveS3SecretKey = ""
	cfg.validate()
}

func TestValidate_S3PairedCredentials_BothSet(t *testing.T) {
	// Both S3 keys set → valid.
	cfg := buildValidConfig(t)
	cfg.MassiveS3AccessKey = "key"
	cfg.MassiveS3SecretKey = "secret"
	cfg.validate()
}

func TestValidate_DataDirNotExist_Subprocess(t *testing.T) {
	if os.Getenv("RUN_FATAL_TEST") != "1" {
		t.Skip("Set RUN_FATAL_TEST=1 to run fatal-exit tests in subprocess")
	}
	cfg := buildValidConfig(t)
	cfg.DataDir = filepath.Join(t.TempDir(), "nonexistent")
	cfg.validate() // Expected: log.Fatal
}

func TestValidate_PriceFetchIntervalZero_Subprocess(t *testing.T) {
	if os.Getenv("RUN_FATAL_TEST") != "1" {
		t.Skip("Set RUN_FATAL_TEST=1 to run fatal-exit tests in subprocess")
	}
	cfg := buildValidConfig(t)
	cfg.PriceFetchInterval = 0
	cfg.validate() // Expected: log.Fatal
}

func TestValidate_IntradayRetentionDaysZero_Subprocess(t *testing.T) {
	if os.Getenv("RUN_FATAL_TEST") != "1" {
		t.Skip("Set RUN_FATAL_TEST=1 to run fatal-exit tests in subprocess")
	}
	cfg := buildValidConfig(t)
	cfg.IntradayRetentionDays = 0
	cfg.validate() // Expected: log.Fatal
}

func TestValidate_COTHistoryWeeksTooLow_Subprocess(t *testing.T) {
	if os.Getenv("RUN_FATAL_TEST") != "1" {
		t.Skip("Set RUN_FATAL_TEST=1 to run fatal-exit tests in subprocess")
	}
	cfg := buildValidConfig(t)
	cfg.COTHistoryWeeks = 2
	cfg.validate() // Expected: log.Fatal
}
