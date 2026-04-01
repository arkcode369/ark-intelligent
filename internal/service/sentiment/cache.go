package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

// cacheTTL controls how long sentiment data stays valid before a re-fetch.
// AAII updates weekly; CNN F&G updates daily. 6 hours is a safe balance.
const cacheTTL = 6 * time.Hour

// badgerCacheKey is the key used to persist sentiment data in BadgerDB.
const badgerCacheKey = "sentiment:v1:latest"

var (
	cachedSentiment *SentimentData
	cacheMu         sync.Mutex // full Mutex prevents TOCTOU race on cache miss
	cacheExpiry     time.Time

	// cacheDB holds an optional BadgerDB handle for disk persistence.
	// When nil, the cache operates in pure in-memory mode (backward-compatible).
	cacheDB *badger.DB
)

// InitSentimentCache injects a BadgerDB handle so that sentiment data survives
// process restarts. This must be called during startup before any fetch.
// If db is nil, the cache falls back to pure in-memory behavior.
func InitSentimentCache(db *badger.DB) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheDB = db
}

// GetCachedOrFetch returns cached SentimentData if still within TTL,
// else fetches fresh data. Thread-safe; only one in-flight fetch at a time
// (prevents duplicate API calls when multiple goroutines hit an expired cache).
func GetCachedOrFetch(ctx context.Context) (*SentimentData, error) {
	cacheMu.Lock()

	// Fast path: in-memory cache hit
	if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
		data := cachedSentiment
		cacheMu.Unlock()
		return data, nil
	}

	// Try loading from BadgerDB before fetching from network.
	if cacheDB != nil && cachedSentiment == nil {
		if data, exp, err := loadFromBadger(); err == nil && time.Now().Before(exp) {
			cachedSentiment = data
			cacheExpiry = exp
			log.Debug().Dur("age", time.Since(data.FetchedAt)).Msg("sentiment loaded from disk cache")
			cacheMu.Unlock()
			return data, nil
		}
	}

	// Slow path: fetch while holding lock (serializes concurrent misses)
	cacheMu.Unlock()

	data, err := FetchSentiment(ctx)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	// Double-check: another goroutine may have fetched while we were fetching
	if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
		data = cachedSentiment
	} else {
		cachedSentiment = data
		cacheExpiry = time.Now().Add(cacheTTL)
		persistToBadger(data, cacheExpiry)
	}
	cacheMu.Unlock()

	return data, nil
}

// InvalidateCache forces the next call to GetCachedOrFetch to re-fetch.
func InvalidateCache() {
	cacheMu.Lock()
	cachedSentiment = nil
	cacheExpiry = time.Time{}
	if cacheDB != nil {
		_ = cacheDB.Update(func(txn *badger.Txn) error {
			return txn.Delete([]byte(badgerCacheKey))
		})
	}
	cacheMu.Unlock()
}

// CacheAge returns how old the current cached data is, or -1 if no cache exists.
func CacheAge() time.Duration {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedSentiment == nil {
		return -1
	}
	return time.Since(cachedSentiment.FetchedAt)
}

// --- BadgerDB persistence helpers ----------------------------------------

// badgerEntry is the on-disk envelope for sentiment data.
type badgerEntry struct {
	Data   *SentimentData `json:"data"`
	Expiry time.Time      `json:"expiry"`
}

// loadFromBadger reads the cached sentiment data from BadgerDB.
// Returns the data, its expiry time, or an error if not found/corrupt.
func loadFromBadger() (*SentimentData, time.Time, error) {
	if cacheDB == nil {
		return nil, time.Time{}, fmt.Errorf("no db")
	}

	var entry badgerEntry
	err := cacheDB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(badgerCacheKey))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry)
		})
	})
	if err != nil {
		return nil, time.Time{}, err
	}
	return entry.Data, entry.Expiry, nil
}

// persistToBadger writes the sentiment data to BadgerDB with TTL.
// Errors are logged but not propagated — disk persistence is best-effort.
func persistToBadger(data *SentimentData, expiry time.Time) {
	if cacheDB == nil {
		return
	}

	entry := badgerEntry{
		Data:   data,
		Expiry: expiry,
	}
	val, err := json.Marshal(&entry)
	if err != nil {
		log.Warn().Err(err).Msg("sentiment cache: failed to marshal for disk persistence")
		return
	}

	err = cacheDB.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(badgerCacheKey), val).WithTTL(cacheTTL)
		return txn.SetEntry(e)
	})
	if err != nil {
		log.Warn().Err(err).Msg("sentiment cache: failed to persist to BadgerDB")
	} else {
		log.Debug().Msg("sentiment cache: persisted to disk")
	}
}
