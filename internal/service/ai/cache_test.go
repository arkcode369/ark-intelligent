package ai

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory mock: ports.AICacheRepository
// ---------------------------------------------------------------------------

type mockCache struct {
	mu    sync.Mutex
	store map[string]string
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string]string)}
}

func (m *mockCache) Get(ctx context.Context, key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.store[key]
	return v, ok
}

func (m *mockCache) Set(ctx context.Context, key, response, cacheType, dataVersion string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = response
	return nil
}

func (m *mockCache) InvalidateByPrefix(ctx context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.store {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.store, k)
		}
	}
	return nil
}

var _ ports.AICacheRepository = (*mockCache)(nil)

// ---------------------------------------------------------------------------
// In-memory mock: ports.AIAnalyzer
// ---------------------------------------------------------------------------

type mockAnalyzer struct {
	available    bool
	callCount    int
	returnResult string
	returnErr    error
}

func (m *mockAnalyzer) IsAvailable() bool { return m.available }

func (m *mockAnalyzer) AnalyzeCOT(ctx context.Context, analyses []domain.COTAnalysis) (string, error) {
	m.callCount++
	return m.returnResult, m.returnErr
}

func (m *mockAnalyzer) AnalyzeCOTWithPrice(ctx context.Context, analyses []domain.COTAnalysis, priceCtx map[string]*domain.PriceContext) (string, error) {
	m.callCount++
	return m.returnResult, m.returnErr
}

func (m *mockAnalyzer) GenerateWeeklyOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	m.callCount++
	return m.returnResult, m.returnErr
}

func (m *mockAnalyzer) AnalyzeCrossMarket(ctx context.Context, cotData map[string]*domain.COTAnalysis) (string, error) {
	m.callCount++
	return m.returnResult, m.returnErr
}

func (m *mockAnalyzer) AnalyzeNewsOutlook(ctx context.Context, events []domain.NewsEvent, lang string) (string, error) {
	m.callCount++
	return m.returnResult, m.returnErr
}

func (m *mockAnalyzer) AnalyzeCombinedOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	m.callCount++
	return m.returnResult, m.returnErr
}

func (m *mockAnalyzer) AnalyzeFREDOutlook(ctx context.Context, data *fred.MacroData, lang string) (string, error) {
	m.callCount++
	return m.returnResult, m.returnErr
}

func (m *mockAnalyzer) AnalyzeActualRelease(ctx context.Context, event domain.NewsEvent, lang string) (string, error) {
	m.callCount++
	return m.returnResult, m.returnErr
}

var _ ports.AIAnalyzer = (*mockAnalyzer)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeAnalysis(date time.Time) domain.COTAnalysis {
	return domain.COTAnalysis{
		Contract:   domain.COTContract{Code: "EUR", Currency: "EUR"},
		ReportDate: date,
	}
}

// ---------------------------------------------------------------------------
// Test 1: Cache miss → inner called, result stored in cache
// ---------------------------------------------------------------------------

func TestCachedInterpreter_CacheMiss_CallsInner(t *testing.T) {
	inner := &mockAnalyzer{available: true, returnResult: "EUR is bullish"}
	cache := newMockCache()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()
	analyses := []domain.COTAnalysis{makeAnalysis(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))}

	result, err := ci.AnalyzeCOTWithPrice(ctx, analyses, nil)

	require.NoError(t, err)
	assert.Equal(t, "EUR is bullish", result)
	// Inner should have been called exactly once
	assert.Equal(t, 1, inner.callCount)
}

// ---------------------------------------------------------------------------
// Test 2: Cache hit → inner NOT called on second request
// ---------------------------------------------------------------------------

func TestCachedInterpreter_CacheHit_SkipsInner(t *testing.T) {
	inner := &mockAnalyzer{available: true, returnResult: "EUR is bullish"}
	cache := newMockCache()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()
	analyses := []domain.COTAnalysis{makeAnalysis(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))}

	// First call — populates cache
	result1, err := ci.AnalyzeCOTWithPrice(ctx, analyses, nil)
	require.NoError(t, err)
	assert.Equal(t, "EUR is bullish", result1)
	assert.Equal(t, 1, inner.callCount)

	// Second call with same data — should hit cache
	result2, err := ci.AnalyzeCOTWithPrice(ctx, analyses, nil)
	require.NoError(t, err)
	assert.Equal(t, "EUR is bullish", result2)
	// Inner should still have been called only once
	assert.Equal(t, 1, inner.callCount)
}

// ---------------------------------------------------------------------------
// Test 3: Cache nil → bypass caching, always call inner
// ---------------------------------------------------------------------------

func TestCachedInterpreter_NilCache_AlwaysCallsInner(t *testing.T) {
	inner := &mockAnalyzer{available: true, returnResult: "direct result"}
	ci := NewCachedInterpreter(inner, nil) // nil cache = passthrough

	ctx := context.Background()
	analyses := []domain.COTAnalysis{makeAnalysis(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))}

	_, err := ci.AnalyzeCOTWithPrice(ctx, analyses, nil)
	require.NoError(t, err)
	_, err = ci.AnalyzeCOTWithPrice(ctx, analyses, nil)
	require.NoError(t, err)

	// Inner called both times (no caching)
	assert.Equal(t, 2, inner.callCount)
}

// ---------------------------------------------------------------------------
// Test 4: IsAvailable — delegates to inner
// ---------------------------------------------------------------------------

func TestCachedInterpreter_IsAvailable(t *testing.T) {
	t.Run("available=true", func(t *testing.T) {
		inner := &mockAnalyzer{available: true}
		ci := NewCachedInterpreter(inner, newMockCache())
		assert.True(t, ci.IsAvailable())
	})

	t.Run("available=false", func(t *testing.T) {
		inner := &mockAnalyzer{available: false}
		ci := NewCachedInterpreter(inner, newMockCache())
		assert.False(t, ci.IsAvailable())
	})
}

// ---------------------------------------------------------------------------
// Test 5: InvalidateOnCOTUpdate — removes COT/weekly/cross/combined cache entries
// ---------------------------------------------------------------------------

func TestCachedInterpreter_InvalidateOnCOTUpdate(t *testing.T) {
	inner := &mockAnalyzer{available: true, returnResult: "cached"}
	cache := newMockCache()

	// Manually seed cache with multiple prefixes
	ctx := context.Background()
	cache.Set(ctx, "aicache:cot:20260401:np", "cot result", "cot", "20260401")
	cache.Set(ctx, "aicache:weekly:20260401:20260401:id", "weekly result", "weekly", "20260401")
	cache.Set(ctx, "aicache:cross:20260401", "cross result", "cross", "20260401")
	cache.Set(ctx, "aicache:combined:20260401:id:nf", "combined result", "combined", "20260401")
	cache.Set(ctx, "aicache:fred:20260401:id", "fred result", "fred", "20260401") // should survive

	ci := NewCachedInterpreter(inner, cache)
	ci.InvalidateOnCOTUpdate(ctx)

	// COT-related entries should be gone
	_, ok := cache.Get(ctx, "aicache:cot:20260401:np")
	assert.False(t, ok, "cot entry should be invalidated")

	_, ok = cache.Get(ctx, "aicache:weekly:20260401:20260401:id")
	assert.False(t, ok, "weekly entry should be invalidated")

	_, ok = cache.Get(ctx, "aicache:cross:20260401")
	assert.False(t, ok, "cross entry should be invalidated")

	_, ok = cache.Get(ctx, "aicache:combined:20260401:id:nf")
	assert.False(t, ok, "combined entry should be invalidated")

	// FRED should survive COT invalidation
	fredVal, ok := cache.Get(ctx, "aicache:fred:20260401:id")
	assert.True(t, ok, "fred entry should survive COT invalidation")
	assert.Equal(t, "fred result", fredVal)
}

// ---------------------------------------------------------------------------
// Test 6: AnalyzeActualRelease — never cached, always calls inner
// ---------------------------------------------------------------------------

func TestCachedInterpreter_AnalyzeActualRelease_NeverCached(t *testing.T) {
	inner := &mockAnalyzer{available: true, returnResult: "release analysis"}
	cache := newMockCache()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()
	event := domain.NewsEvent{Event: "NFP", Actual: "250K", Forecast: "200K"}

	_, err := ci.AnalyzeActualRelease(ctx, event, "en")
	require.NoError(t, err)
	_, err = ci.AnalyzeActualRelease(ctx, event, "en")
	require.NoError(t, err)

	// Should be called both times (no caching for AnalyzeActualRelease)
	assert.Equal(t, 2, inner.callCount)
}

// ---------------------------------------------------------------------------
// Test 7: Cache miss with inner error — error propagated, nothing stored
// ---------------------------------------------------------------------------

func TestCachedInterpreter_InnerError_NotCached(t *testing.T) {
	expectedErr := errors.New("AI service unavailable")
	inner := &mockAnalyzer{available: true, returnResult: "", returnErr: expectedErr}
	cache := newMockCache()
	ci := NewCachedInterpreter(inner, cache)

	ctx := context.Background()
	analyses := []domain.COTAnalysis{makeAnalysis(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))}

	_, err := ci.AnalyzeCOTWithPrice(ctx, analyses, nil)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 1, inner.callCount)

	// Second call — should call inner again (nothing was cached due to error)
	_, _ = ci.AnalyzeCOTWithPrice(ctx, analyses, nil)
	assert.Equal(t, 2, inner.callCount)
}

// ---------------------------------------------------------------------------
// Test 8: latestReportDate helper — returns correct date string
// ---------------------------------------------------------------------------

func TestLatestReportDate(t *testing.T) {
	t.Run("empty slice returns unknown", func(t *testing.T) {
		got := latestReportDate(nil)
		assert.Equal(t, "unknown", got)
	})

	t.Run("single analysis", func(t *testing.T) {
		analyses := []domain.COTAnalysis{
			makeAnalysis(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)),
		}
		got := latestReportDate(analyses)
		assert.Equal(t, "20260401", got)
	})

	t.Run("picks latest date", func(t *testing.T) {
		analyses := []domain.COTAnalysis{
			makeAnalysis(time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)),
			makeAnalysis(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)),
			makeAnalysis(time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)),
		}
		got := latestReportDate(analyses)
		assert.Equal(t, "20260401", got)
	})
}

// ---------------------------------------------------------------------------
// Test 9: latestReportDateFromMap helper
// ---------------------------------------------------------------------------

func TestLatestReportDateFromMap(t *testing.T) {
	t.Run("nil map returns unknown", func(t *testing.T) {
		got := latestReportDateFromMap(nil)
		assert.Equal(t, "unknown", got)
	})

	t.Run("picks latest date from map", func(t *testing.T) {
		a1 := makeAnalysis(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
		a2 := makeAnalysis(time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC))
		cotMap := map[string]*domain.COTAnalysis{
			"EUR": &a1,
			"GBP": &a2,
		}
		got := latestReportDateFromMap(cotMap)
		assert.Equal(t, "20260401", got)
	})
}

// ---------------------------------------------------------------------------
// Test 10: InvalidateAll — clears all aicache: entries
// ---------------------------------------------------------------------------

func TestCachedInterpreter_InvalidateAll(t *testing.T) {
	inner := &mockAnalyzer{available: true}
	cache := newMockCache()

	ctx := context.Background()
	cache.Set(ctx, "aicache:cot:abc", "v1", "cot", "abc")
	cache.Set(ctx, "aicache:fred:abc", "v2", "fred", "abc")
	cache.Set(ctx, "aicache:weekly:abc", "v3", "weekly", "abc")
	cache.Set(ctx, "other:key", "v4", "other", "other") // should survive

	ci := NewCachedInterpreter(inner, cache)
	ci.InvalidateAll(ctx)

	_, ok := cache.Get(ctx, "aicache:cot:abc")
	assert.False(t, ok)
	_, ok = cache.Get(ctx, "aicache:fred:abc")
	assert.False(t, ok)
	_, ok = cache.Get(ctx, "aicache:weekly:abc")
	assert.False(t, ok)

	// Non-aicache key should survive
	_, ok = cache.Get(ctx, "other:key")
	assert.True(t, ok)
}
