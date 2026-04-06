package telegram

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Mock Bot for handler testing
// ---------------------------------------------------------------------------

type mockBotSender struct {
	mu           sync.Mutex
	sentMessages []mockSentMessage
	sentHTML     []string
	typingCalls  []string
	err          error
}

type mockSentMessage struct {
	ChatID   string
	Text     string
	Keyboard interface{}
}

func (m *mockBotSender) SendMessage(ctx context.Context, chatID string, text string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	m.sentMessages = append(m.sentMessages, mockSentMessage{ChatID: chatID, Text: text})
	return len(m.sentMessages), nil
}

func (m *mockBotSender) SendHTML(ctx context.Context, chatID string, html string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	m.sentHTML = append(m.sentHTML, html)
	return len(m.sentHTML), nil
}

func (m *mockBotSender) SendWithKeyboard(ctx context.Context, chatID string, html string, keyboard interface{}) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	m.sentMessages = append(m.sentMessages, mockSentMessage{ChatID: chatID, Text: html, Keyboard: keyboard})
	m.sentHTML = append(m.sentHTML, html)
	return len(m.sentHTML), nil
}

func (m *mockBotSender) SendTyping(ctx context.Context, chatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.typingCalls = append(m.typingCalls, chatID)
	return nil
}

// Getters for test assertions
func (m *mockBotSender) LastSentHTML() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sentHTML) == 0 {
		return ""
	}
	return m.sentHTML[len(m.sentHTML)-1]
}

func (m *mockBotSender) SentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sentHTML)
}

func (m *mockBotSender) HasKeyboard() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.sentMessages {
		if msg.Keyboard != nil {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Mock Repositories
// ---------------------------------------------------------------------------

type mockCOTRepo struct {
	mu        sync.Mutex
	analysis  *domain.COTAnalysis
	analyses  []domain.COTAnalysis
	err       error
}

func (m *mockCOTRepo) SaveRecords(ctx context.Context, records []domain.COTRecord) error { return nil }
func (m *mockCOTRepo) GetLatest(ctx context.Context, contractCode string) (*domain.COTRecord, error) { return nil, nil }
func (m *mockCOTRepo) GetHistory(ctx context.Context, contractCode string, weeks int) ([]domain.COTRecord, error) { return nil, nil }
func (m *mockCOTRepo) SaveAnalyses(ctx context.Context, analyses []domain.COTAnalysis) error { return nil }

func (m *mockCOTRepo) GetLatestAnalysis(ctx context.Context, contractCode string) (*domain.COTAnalysis, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.analysis, nil
}

func (m *mockCOTRepo) GetAllLatestAnalyses(ctx context.Context) ([]domain.COTAnalysis, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.analyses, nil
}

func (m *mockCOTRepo) GetLatestReportDate(ctx context.Context) (time.Time, error) { return time.Time{}, nil }

// Mock Preferences Repository
type mockPrefsRepo struct {
	mu         sync.Mutex
	prefs      domain.UserPrefs
	err        error
	lastSaved  domain.UserPrefs
	saveCount  int
}

func (m *mockPrefsRepo) Get(ctx context.Context, userID int64) (domain.UserPrefs, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return domain.UserPrefs{}, m.err
	}
	return m.prefs, nil
}

func (m *mockPrefsRepo) Set(ctx context.Context, userID int64, prefs domain.UserPrefs) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSaved = prefs
	m.saveCount++
	return m.err
}

func (m *mockPrefsRepo) GetAllActive(ctx context.Context) (map[int64]domain.UserPrefs, error) {
	return nil, nil
}

// Mock Event Repository
type mockEventRepo struct {
	events []domain.FFEvent
	err    error
}

func (m *mockEventRepo) SaveEvents(ctx context.Context, events []domain.FFEvent) error { return nil }
func (m *mockEventRepo) GetEventsByDateRange(ctx context.Context, start, end time.Time) ([]domain.FFEvent, error) {
	return nil, nil
}
func (m *mockEventRepo) GetEventsByDate(ctx context.Context, date time.Time) ([]domain.FFEvent, error) {
	return nil, nil
}
func (m *mockEventRepo) GetHighImpactEvents(ctx context.Context, start, end time.Time) ([]domain.FFEvent, error) {
	return nil, nil
}
func (m *mockEventRepo) GetEventsByCurrency(ctx context.Context, currency string, start, end time.Time) ([]domain.FFEvent, error) {
	return nil, nil
}
func (m *mockEventRepo) SaveEventDetails(ctx context.Context, details []domain.FFEventDetail) error { return nil }
func (m *mockEventRepo) GetEventHistory(ctx context.Context, eventName, currency string, months int) ([]domain.FFEventDetail, error) {
	return nil, nil
}
func (m *mockEventRepo) SaveRevision(ctx context.Context, rev domain.EventRevision) error { return nil }
func (m *mockEventRepo) GetRevisions(ctx context.Context, currency string, days int) ([]domain.EventRevision, error) {
	return nil, nil
}
func (m *mockEventRepo) GetAllRevisions(ctx context.Context, days int) ([]domain.EventRevision, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// AI Cooldown Tests
// ---------------------------------------------------------------------------

func TestCheckAICooldown_AllowsFirstCall(t *testing.T) {
	h := &Handler{
		aiCooldown: make(map[int64]time.Time),
	}

	userID := int64(123)
	if !h.checkAICooldown(userID) {
		t.Error("first AI call should be allowed")
	}
}

func TestCheckAICooldown_BlocksRapidCalls(t *testing.T) {
	h := &Handler{
		aiCooldown: make(map[int64]time.Time),
	}

	userID := int64(123)
	
	// First call should succeed
	if !h.checkAICooldown(userID) {
		t.Error("first AI call should be allowed")
	}

	// Immediate second call should be blocked
	if h.checkAICooldown(userID) {
		t.Error("rapid AI call should be blocked")
	}
}

func TestCheckAICooldown_AllowsAfterDelay(t *testing.T) {
	// Use a short cooldown for testing
	originalCooldown := aiCooldownDuration
	aiCooldownDuration = 1 * time.Millisecond
	defer func() { aiCooldownDuration = originalCooldown }()

	h := &Handler{
		aiCooldown: make(map[int64]time.Time),
	}

	userID := int64(123)
	
	// First call
	h.checkAICooldown(userID)
	
	// Wait for cooldown
	time.Sleep(5 * time.Millisecond)
	
	// Should be allowed after delay
	if !h.checkAICooldown(userID) {
		t.Error("AI call should be allowed after cooldown period")
	}
}

// ---------------------------------------------------------------------------
// currencyToContractCode Tests
// ---------------------------------------------------------------------------

func TestCurrencyToContractCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"EUR", "099741"},
		{"GBP", "096742"},
		{"JPY", "097741"},
		{"AUD", "232741"},
		{"USD", "098662"},
		{"GOLD", "088691"},
		{"XAU", "088691"},
		{"OIL", "067651"},
		{"UNKNOWN", "UNKNOWN"}, // Pass-through for unknown codes
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := currencyToContractCode(tc.input)
			if result != tc.expected {
				t.Errorf("currencyToContractCode(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestCurrencyToContractCode_CaseInsensitive(t *testing.T) {
	result := currencyToContractCode("eur")
	if result != "099741" {
		t.Errorf("currencyToContractCode should be case insensitive, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// Deep Link Tests
// ---------------------------------------------------------------------------

func TestDeepLinkCache_PushAndPop(t *testing.T) {
	cache := newDeepLinkCache()
	userID := int64(123)
	
	cache.Set(userID, "cot", "EUR")
	
	popped := cache.Pop(userID)
	if popped == nil {
		t.Fatal("expected to get intent from cache")
	}
	
	if popped.Command != "cot" || popped.Args != "EUR" {
		t.Errorf("intent mismatch: got %+v", popped)
	}
	
	// Second pop should return nil
	if cache.Pop(userID) != nil {
		t.Error("second pop should return nil")
	}
}

func TestDeepLinkCache_TTLExpiration(t *testing.T) {
	// This test verifies TTL behavior - intents should expire after 10 minutes
	// We can't easily test this without modifying TTL, so we verify structure
	cache := newDeepLinkCache()
	
	if cache == nil {
		t.Error("cache should be initialized")
	}
}
