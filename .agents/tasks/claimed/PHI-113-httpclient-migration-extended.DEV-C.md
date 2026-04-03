# PHI-113: TASK-306-EXT Extended httpclient.New() Migration

**Status:** in_progress  
**Assigned to:** Dev-C  
**Priority:** medium  
**Type:** refactor  
**Estimated:** M  
**Area:** internal/service/*  
**Created at:** 2026-04-03 WIB  
**Siklus:** Refactor  

## Deskripsi

Migrate ~18 additional service packages from bare `&http.Client{Timeout: ...}` to `httpclient.New()` 
for TCP connection pool reuse under concurrent load. This extends the work completed in TASK-306.

## Services yang Perlu Dimigrasi

1. `internal/service/sec/client.go` — package-level httpClient
2. `internal/service/imf/weo.go` — package-level httpClient
3. `internal/service/treasury/client.go` — package-level httpClient
4. `internal/service/bis/reer.go` — package-level httpClient
5. `internal/service/cot/fetcher.go` — struct field httpClient
6. `internal/service/vix/fetcher.go` — local var in FetchTermStructure
7. `internal/service/vix/move.go` — local var in FetchMOVE
8. `internal/service/vix/vol_suite.go` — local var in FetchVolSuite
9. `internal/service/price/eia.go` — struct field in NewEIAClient
10. `internal/service/news/fed_rss.go` — local var in fetchFedRSS
11. `internal/service/fed/fedwatch.go` — local var in FetchFedWatch
12. `internal/service/marketdata/massive/client.go` — struct field in NewClient
13. `internal/service/macro/treasury_client.go` — struct field in NewTreasuryClient
14. `internal/service/macro/snb_client.go` — struct field in NewSNBClient
15. `internal/service/macro/oecd_client.go` — struct field in NewOECDClient
16. `internal/service/macro/ecb_client.go` — struct field in NewECBClient
17. `internal/service/macro/dtcc_client.go` — struct field in NewDTCCClient
18. `internal/service/macro/eurostat_client.go` — struct field in NewEurostatClient

## Acceptance Criteria

- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Semua service di atas menggunakan `httpclient.New(httpclient.WithTimeout(d))` atau `httpclient.NewClient(d)`
- [ ] Tidak ada behavior change — semua requests tetap identik
- [ ] `net/http` import dihapus dari file yang tidak lagi menggunakannya langsung

## Referensi

- `pkg/httpclient/transport.go` — factory yang perlu digunakan
- TASK-118 — original migration (completed)
- TASK-306 — first batch migration (completed)
- `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`
- Paperclip: [PHI-113](/PHI/issues/PHI-113)

## Progress

- [ ] sec/client.go
- [ ] imf/weo.go
- [ ] treasury/client.go
- [ ] bis/reer.go
- [ ] cot/fetcher.go
- [ ] vix/*.go (3 files)
- [ ] price/eia.go
- [ ] news/fed_rss.go
- [ ] fed/fedwatch.go
- [ ] marketdata/massive/client.go
- [ ] macro/*_client.go (6 files)
