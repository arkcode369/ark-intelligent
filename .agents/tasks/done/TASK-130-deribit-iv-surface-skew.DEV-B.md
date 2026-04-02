# TASK-130: Deribit IV Surface + Skew + Term Structure

**Status:** done (retroactive close)
**Closed by:** Dev-B
**Closed at:** 2026-04-02 09:18 WIB

## Implementation

Already implemented:
- `internal/service/gex/iv_surface.go` — IV surface computation, AnalyzeIVSurface()
- `internal/service/gex/skew.go` — Skew metrics per expiry, risk reversal
- `internal/service/marketdata/deribit/client.go` — get_book_summary_by_currency endpoint
- `internal/adapter/telegram/handler_gex.go` — /ivol command
- `internal/adapter/telegram/formatter_gex.go` — IV surface formatter

go build + go vet: clean. No new PR needed (implemented via prior work merged to agents/main).
