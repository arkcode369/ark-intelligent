package ports

import "context"

// AICacheRepository defines storage operations for AI narrative caching.
type AICacheRepository interface {
	// Get retrieves a cached AI response by key. Returns ("", false) on cache miss.
	Get(ctx context.Context, key string) (string, bool)

	// Set stores an AI response with the given key and TTL.
	Set(ctx context.Context, key string, response string, cacheType string, dataVersion string) error

	// InvalidateByPrefix deletes all cache entries matching a key prefix.
	InvalidateByPrefix(ctx context.Context, prefix string) error
}
