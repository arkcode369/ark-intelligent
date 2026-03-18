package fred

import (
	"context"
	"sync"
	"time"
)

// defaultTTL is how long fetched FRED data stays valid before a re-fetch.
// FRED daily series update intraday; monthly series (PCE, CPI) update less often.
// 1 hour is a safe balance between freshness and rate-limit protection.
const defaultTTL = 1 * time.Hour

type cachedMacroData struct {
	data      *MacroData
	fetchedAt time.Time
}

var (
	globalCache *cachedMacroData
	cacheMu     sync.RWMutex
	cacheTTL    = defaultTTL //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached MacroData if still within TTL, else fetches fresh data.
// Thread-safe. Use this in all handlers instead of FetchMacroData directly.
func GetCachedOrFetch(ctx context.Context) (*MacroData, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.fetchedAt) < cacheTTL {
		data := globalCache.data
		cacheMu.RUnlock()
		return data, nil
	}
	cacheMu.RUnlock()

	// Fetch fresh data
	data, err := FetchMacroData(ctx)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	globalCache = &cachedMacroData{data: data, fetchedAt: time.Now()}
	cacheMu.Unlock()

	return data, nil
}

// InvalidateCache forces the next call to GetCachedOrFetch to re-fetch from FRED.
// Call this when the user explicitly requests a refresh.
func InvalidateCache() {
	cacheMu.Lock()
	globalCache = nil
	cacheMu.Unlock()
}

// CacheAge returns how old the current cached data is, or -1 if no cache exists.
func CacheAge() time.Duration {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if globalCache == nil {
		return -1
	}
	return time.Since(globalCache.fetchedAt)
}
