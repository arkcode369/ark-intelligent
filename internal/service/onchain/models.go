// Package onchain provides on-chain metrics via public APIs (CoinMetrics community tier).
package onchain

import "time"

// ExchangeFlow holds daily exchange in/out flow for a single asset.
type ExchangeFlow struct {
	Date       time.Time
	FlowInNtv  float64 // coins flowing into exchanges
	FlowOutNtv float64 // coins flowing out of exchanges
	NetFlow    float64 // FlowInNtv - FlowOutNtv (negative = accumulation)
}

// ActiveAddressMetric holds daily active address and tx count.
type ActiveAddressMetric struct {
	Date           time.Time
	ActiveAddresses int64
	TxCount        int64
}

// AssetOnChainSummary holds computed on-chain metrics for an asset.
type AssetOnChainSummary struct {
	Asset              string
	Flows              []ExchangeFlow
	NetFlow7D          float64   // sum of net flows over last 7 days
	NetFlow30D         float64   // sum of net flows over last 30 days
	ConsecutiveOutflow int       // consecutive days of net outflow (accumulation)
	LargeInflowSpike   bool      // single-day inflow > 2x avg
	FlowTrend          string    // "ACCUMULATION" | "DISTRIBUTION" | "NEUTRAL"
	ActiveAddresses    int64     // latest active address count
	ActiveAddrChange7D float64   // % change in active addresses over 7 days
	TxCount            int64     // latest tx count
	FetchedAt          time.Time
	Available          bool
}

// OnChainReport holds the combined on-chain report for all tracked assets.
type OnChainReport struct {
	Assets    map[string]*AssetOnChainSummary // keyed by asset symbol (e.g. "btc", "eth")
	FetchedAt time.Time
	Available bool
}
