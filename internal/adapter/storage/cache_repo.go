package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// CacheTTL is the default TTL for AI cache entries (7 days).
// Because keys embed the data version, stale entries auto-expire
// even if invalidation is not explicitly triggered.
const CacheTTL = 7 * 24 * time.Hour

// CacheRepo implements ports.AICacheRepository using BadgerDB.
type CacheRepo struct {
	db *badger.DB
}

// NewCacheRepo creates a new CacheRepo backed by the given DB.
func NewCacheRepo(db *DB) *CacheRepo {
	return &CacheRepo{db: db.Badger()}
}

// Get retrieves a cached AI response. Returns ("", false) on miss.
func (r *CacheRepo) Get(_ context.Context, key string) (string, bool) {
	var response string
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			var entry domain.AICacheEntry
			if err := json.Unmarshal(val, &entry); err != nil {
				return err
			}
			response = entry.Response
			return nil
		})
	})
	if err != nil {
		return "", false
	}
	return response, true
}

// Set stores an AI response with TTL.
func (r *CacheRepo) Set(_ context.Context, key string, response string, cacheType string, dataVersion string) error {
	entry := domain.AICacheEntry{
		CacheType:   cacheType,
		CacheKey:    key,
		Response:    response,
		DataVersion: dataVersion,
		CreatedAt:   time.Now(),
	}
	data, err := json.Marshal(&entry)
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}

	return r.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), data).WithTTL(CacheTTL)
		return txn.SetEntry(e)
	})
}

// InvalidateByPrefix deletes all cache entries matching a key prefix.
func (r *CacheRepo) InvalidateByPrefix(_ context.Context, prefix string) error {
	deleteKeys := make([][]byte, 0)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = false // keys only

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			deleteKeys = append(deleteKeys, it.Item().KeyCopy(nil))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan cache prefix %q: %w", prefix, err)
	}

	if len(deleteKeys) == 0 {
		return nil
	}

	wb := r.db.NewWriteBatch()
	defer wb.Cancel()
	for _, k := range deleteKeys {
		if err := wb.Delete(k); err != nil {
			return fmt.Errorf("delete cache key %s: %w", k, err)
		}
	}
	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush cache delete: %w", err)
	}

	log.Printf("[cache] Invalidated %d entries with prefix %q", len(deleteKeys), prefix)
	return nil
}
