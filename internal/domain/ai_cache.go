package domain

import "time"

// AICacheEntry stores a cached AI response in BadgerDB.
type AICacheEntry struct {
	// CacheType identifies the analysis type (e.g. "cot", "weekly", "cross", "news", "combined", "fred").
	CacheType string `json:"cache_type"`

	// CacheKey is the full BadgerDB key used for this entry.
	CacheKey string `json:"cache_key"`

	// Response is the AI-generated narrative text (HTML for Telegram).
	Response string `json:"response"`

	// DataVersion encodes the version of input data (e.g. COT report date, week start).
	DataVersion string `json:"data_version"`

	// CreatedAt is when this cache entry was stored.
	CreatedAt time.Time `json:"created_at"`
}
