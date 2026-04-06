package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// makeValidConfig returns a Config pre-populated with values that pass all
// validation checks. Tests should modify individual fields to trigger errors.
func makeValidConfig(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	return &Config{
		BotToken:               "test-token",
		ChatID:                 "123456",
		DataDir:                dir,
		GeminiModel:            "gemini-test",
		COTFetchInterval:       6 * time.Hour,
		COTHistoryWeeks:        52,
		ConfluenceCalcInterval: 2 * time.Hour,
		PriceFetchInterval:     6 * time.Hour,
		PriceHistoryWeeks:      52,
		IntradayFetchInterval:  15 * time.Minute,
		IntradayRetentionDays:  60,
		AICacheTTL:             1 * time.Hour,
		AIMaxRPM:               15,
		AIMaxDaily:             200,
		ChatHistoryLimit:       50,
		ImpactBootstrapMonths:  12,
		ClaudeMaxTokens:        8192,
		ClaudeThinkingBudget:   0,
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := makeValidConfig(t)
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}
}

func TestValidate_COTHistoryWeeks(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.COTHistoryWeeks = 3
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for COTHistoryWeeks < 4")
	}
}

func TestValidate_COTFetchInterval(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.COTFetchInterval = 30 * time.Second
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for COTFetchInterval < 1m")
	}
}

func TestValidate_ConfluenceCalcInterval(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ConfluenceCalcInterval = 30 * time.Second
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for ConfluenceCalcInterval < 1m")
	}
}

func TestValidate_PriceFetchInterval(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.PriceFetchInterval = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for PriceFetchInterval <= 0")
	}
}

func TestValidate_PriceHistoryWeeks(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.PriceHistoryWeeks = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for PriceHistoryWeeks < 1")
	}
}

func TestValidate_IntradayFetchInterval(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.IntradayFetchInterval = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for IntradayFetchInterval <= 0")
	}
}

func TestValidate_IntradayRetentionDays(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.IntradayRetentionDays = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for IntradayRetentionDays < 1")
	}
}

func TestValidate_AICacheTTL(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.AICacheTTL = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for AICacheTTL <= 0")
	}
}

func TestValidate_AIMaxRPM(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.AIMaxRPM = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for AIMaxRPM <= 0")
	}
}

func TestValidate_AIMaxDaily(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.AIMaxDaily = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for AIMaxDaily <= 0")
	}
}

func TestValidate_ChatHistoryLimitDefault(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ChatHistoryLimit = 0
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error for ChatHistoryLimit=0 (auto-default), got: %v", err)
	}
	if cfg.ChatHistoryLimit != 50 {
		t.Errorf("expected ChatHistoryLimit reset to 50, got %d", cfg.ChatHistoryLimit)
	}
}

func TestValidate_AIMaxDailyLessThanRPM(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.AIMaxDaily = 5
	cfg.AIMaxRPM = 10
	// Advisory warning only — should not return an error.
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error for AIMaxDaily < AIMaxRPM (warn only), got: %v", err)
	}
}

func TestValidate_ImpactBootstrapMonths(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ImpactBootstrapMonths = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for ImpactBootstrapMonths < 1")
	}
}

func TestValidate_ClaudeModelRequired(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ClaudeEndpoint = "http://example.com"
	cfg.ClaudeModel = ""
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when ClaudeEndpoint set without ClaudeModel")
	}
}

func TestValidate_ClaudeMaxTokensRequired(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ClaudeEndpoint = "http://example.com"
	cfg.ClaudeModel = "claude-test"
	cfg.ClaudeMaxTokens = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for ClaudeMaxTokens <= 0 when Claude enabled")
	}
}

func TestValidate_ClaudeThinkingBudgetNegative(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.ClaudeThinkingBudget = -1
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for ClaudeThinkingBudget < 0")
	}
}

func TestValidate_MassiveS3Mismatch(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.MassiveS3AccessKey = "access-key"
	cfg.MassiveS3SecretKey = ""
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for mismatched Massive S3 credentials")
	}
}

func TestValidate_DataDirNotWritable(t *testing.T) {
	cfg := makeValidConfig(t)
	cfg.DataDir = "/nonexistent/path/xyz123"
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for non-writable DataDir")
	}
}

// TestLoad_MissingRequiredEnv tests that Load() returns an error (not panic)
// when required environment variables are missing.
func TestLoad_MissingRequiredEnv(t *testing.T) {
	// Clear the required env vars
	oldBotToken := os.Getenv("BOT_TOKEN")
	oldChatID := os.Getenv("CHAT_ID")
	defer func() {
		os.Setenv("BOT_TOKEN", oldBotToken)
		os.Setenv("CHAT_ID", oldChatID)
	}()

	os.Unsetenv("BOT_TOKEN")
	os.Unsetenv("CHAT_ID")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when required env vars are missing, got nil")
	}

	// Should report both missing vars
	errStr := err.Error()
	if !strings.Contains(errStr, "BOT_TOKEN") {
		t.Errorf("error message should mention BOT_TOKEN, got: %s", errStr)
	}
	if !strings.Contains(errStr, "CHAT_ID") {
		t.Errorf("error message should mention CHAT_ID, got: %s", errStr)
	}
}

// TestLoad_MissingBotTokenOnly tests error when only BOT_TOKEN is missing.
func TestLoad_MissingBotTokenOnly(t *testing.T) {
	oldBotToken := os.Getenv("BOT_TOKEN")
	oldChatID := os.Getenv("CHAT_ID")
	defer func() {
		os.Setenv("BOT_TOKEN", oldBotToken)
		os.Setenv("CHAT_ID", oldChatID)
	}()

	os.Unsetenv("BOT_TOKEN")
	os.Setenv("CHAT_ID", "123456")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when BOT_TOKEN is missing")
	}
	if !strings.Contains(err.Error(), "BOT_TOKEN") {
		t.Errorf("error should mention BOT_TOKEN, got: %s", err.Error())
	}
}

// TestLoad_Success tests that Load() succeeds with valid required env vars.
func TestLoad_Success(t *testing.T) {
	oldBotToken := os.Getenv("BOT_TOKEN")
	oldChatID := os.Getenv("CHAT_ID")
	defer func() {
		os.Setenv("BOT_TOKEN", oldBotToken)
		os.Setenv("CHAT_ID", oldChatID)
	}()

	// Set required env vars with valid test values
	os.Setenv("BOT_TOKEN", "test-bot-token-123")
	os.Setenv("CHAT_ID", "-1001234567890")
	// Use temp dir for DATA_DIR
	t.Setenv("DATA_DIR", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error with valid env vars, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.BotToken != "test-bot-token-123" {
		t.Errorf("expected BotToken to be set, got: %s", cfg.BotToken)
	}
	if cfg.ChatID != "-1001234567890" {
		t.Errorf("expected ChatID to be set, got: %s", cfg.ChatID)
	}
}

// TestRequireEnv_Success tests the requireEnv helper with a set env var.
func TestRequireEnv_Success(t *testing.T) {
	t.Setenv("TEST_REQ_VAR", "test-value")
	val, err := requireEnv("TEST_REQ_VAR")
	if err != nil {
		t.Fatalf("expected no error for set env var, got: %v", err)
	}
	if val != "test-value" {
		t.Errorf("expected 'test-value', got: %s", val)
	}
}

// TestRequireEnv_Missing tests the requireEnv helper with a missing env var.
func TestRequireEnv_Missing(t *testing.T) {
	os.Unsetenv("TEST_REQ_VAR_MISSING")
	val, err := requireEnv("TEST_REQ_VAR_MISSING")
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
	if val != "" {
		t.Errorf("expected empty value, got: %s", val)
	}
	if !strings.Contains(err.Error(), "TEST_REQ_VAR_MISSING") {
		t.Errorf("error should mention the var name, got: %s", err.Error())
	}
}
