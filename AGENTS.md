# ARK Intelligent — Agent Playbook

## 🎯 Purpose
ARK Intelligent adalah **institutional-grade macro intelligence Telegram bot** untuk forex trader, yang menyediakan analisis multi-strategi dari COT positioning, data makro FRED, economic calendar surprises, dan analisis teknikal.

**Agent di project ini bertugas**: Membangun, memelihara, dan mengoptimalkan sistem analisis trading yang reliable, data-driven, dan scalable — dengan fokus pada **data integrity**, **signal accuracy**, dan **graceful degradation** saat external services unavailable.

---

## 🧭 Core Principles

### 1. **Data Integrity First**
- Semua data dari external APIs harus divalidasi sebelum persistensi
- Graceful degradation: sistem harus tetap berfungsi jika satu data source down (fallback chain)
- Historical data tidak boleh corrupt — use transactions, atomic writes

### 2. **Signal Quality Over Quantity**
- Setiap signal harus punya **calibrated confidence score** (Platt scaling)
- Backtest sebelum deploy strategi baru
- Track signal performance (hit rate, avg PnL, max drawdown) per regime

### 3. **Defensive Programming**
- Assume all external APIs can fail — implement retry dengan exponential backoff
- Rate limiting: respect API quotas, implement local caching (TTL-based)
- Circuit breaker pattern untuk services yang sering timeout

### 4. **Observability**
- Log semua data fetch failures dengan context (source, timestamp, error)
- Metrics: fetch success rate, signal accuracy, AI response latency
- Health check endpoint untuk monitoring (already at :8080)

### 5. **Modularity & Testability**
- Hexagonal architecture: business logic terpisah dari adapters (Telegram, APIs, DB)
- Dependency injection manual (no framework) — testable dengan mock implementations
- Unit tests untuk semua analyzer/strategy logic

---

## 🛠️ Capabilities

Agent di ark-intelligent bisa:

### **Data Pipeline**
- Fetch & sync data dari CFTC Socrata, FRED API, MQL5 Economic Calendar, Bybit
- Implement fallback chains (TwelveData → AlphaVantage → Yahoo → CoinGecko)
- Bootstrap historical data dengan incremental backfill
- Detect data anomalies (missing weeks, outliers, regime shifts)

### **Strategy Development**
- Implement new COT signal logic (net change, percentile ranking, confluence)
- Add new technical indicators (TA library: RSI, MACD, Bollinger, ATR, etc.)
- Build multi-factor models (COT + Macro + Technical confluence)
- Create regime-aware strategies (adaptive based on FRED macro regime)

### **Backtest & Validation**
- Replay historical signals against price data
- Calculate performance metrics (Sharpe, Sortino, max drawdown, win rate)
- Calibrate confidence scores dengan isotonic regression / Platt scaling
- Walk-forward analysis untuk avoid overfitting

### **AI/ML Integration**
- Wire new AI providers (Claude, Gemini, custom models)
- Implement tool use (memory, file search, API calls)
- Build prompt templates untuk macro narrative generation
- Cache AI responses dengan invalidation on data updates

### **Telegram Bot Features**
- Add new commands (/xfactors, /playbook, /heat, /rankx, /transition, /cryptoalpha)
- Implement user preferences (tiered access, alert filters, personal quotas)
- Build interactive keyboards (inline buttons untuk drill-down)
- Handle free-text chatbot mode dengan context awareness

### **Infrastructure & Ops**
- Docker Compose setup untuk production deployment
- Graceful shutdown handling (SIGINT/SIGTERM, drain in-flight requests)
- Background job schedulers dengan graceful cancellation
- Health check & readiness probes

---

## 🔄 Workflow

### 1. **Understand Requirement**
- Baca context: issue description, PR comments, user feedback
- Identifikasi domain: data pipeline, strategy, backtest, AI, bot UI, infra
- Cek existing implementation: jangan re-invent wheel

### 2. **Plan & Break Down**
- Split jadi task kecil (< 2h effort masing-masing)
- Define acceptance criteria yang testable
- Estimate risk: data migration? breaking change? API rate limit?

### 3. **Implement dengan Test-First**
```
1. Write unit test untuk new logic
2. Implement minimal code to pass test
3. Refactor untuk clarity & performance
4. Run integration tests (if applicable)
```

### 4. **Validate**
- Run `go test ./... -race` — no race conditions
- Run `go vet ./...` — static analysis
- Check coverage: new code harus ≥80%
- Manual test: run bot locally, test command end-to-end

### 5. **Document**
- Update README.md jika ada new command/config
- Add inline docs untuk complex logic
- Create ADR (Architecture Decision Record) jika ada major change

### 6. **Review & Merge**
- Submit PR dengan clear description
- Request review dari maintainer
- Address feedback, iterate
- Merge setelah CI green + review approved

---

## 📋 Task Spec Template

Setiap task harus punya struktur berikut:

```markdown
## Problem Statement
[Apa yang salah atau perlu diperbaiki/ditambahkan. Contoh: "Signal COT untuk EUR tidak memperhitungkan regime change, menyebabkan false signals saat inflation regime berubah"]

## Acceptance Criteria
- [ ] Unit tests passing dengan coverage ≥80%
- [ ] Integration test end-to-end (fetch → analyze → persist → notify)
- [ ] Backtest menunjukkan improvement ≥5% hit rate vs baseline
- [ ] Graceful degradation jika FRED API unavailable
- [ ] Log semua failures dengan sufficient context

## Technical Context
**Files involved:**
- `internal/service/cot/analyzer.go` — COT signal logic
- `internal/service/fred/regime.go` — regime detection
- `internal/adapter/telegram/handler.go` — command handlers

**Dependencies:**
- COT data: weekly, Friday release
- FRED data: daily, multiple series
- Price data: daily + 4H intraday

**Constraint:**
- FRED API rate limit: 120 requests/min
- Must not block Telegram polling loop
- Graceful fallback ke template jika AI offline

## Priority & Impact
**Priority:** High
**Impact:** Affects all EUR signals, estimated 15% improvement in accuracy
**Urgency:** Next COT release is Friday 4PM UTC

## Estimated Effort
Medium (4-8 hours)
```

---

## ⚠️ Rules & Constraints

### **Code Quality**
- ✅ Use Go 1.22+ features (but avoid bleeding-edge)
- ✅ Follow hexagonal architecture: business logic tidak depend on Telegram/DB
- ✅ Manual DI: no framework, wire di `main.go`
- ✅ Structured logging dengan `zerolog` (JSON format)
- ✅ Error wrapping dengan `%w` dan context (`fmt.Errorf("fetch cot: %w", err)`)

### **Testing**
- ✅ Unit tests untuk semua analyzer/strategy logic
- ✅ Mock external services (use `interfaces` untuk DI)
- ✅ Integration tests untuk critical paths (signal generation → notification)
- ✅ Race detector: `go test -race` wajib before merge
- ✅ Coverage target: ≥80% untuk core logic, ≥60% untuk adapters

### **Data & APIs**
- ✅ Respect rate limits: implement token bucket / sliding window
- ✅ Cache responses dengan TTL (BadgerDB cache repo)
- ✅ Fallback chains: primary → secondary → tertiary
- ✅ Validate all external data before persist (schema, ranges, outliers)
- ✅ Atomic writes untuk critical updates (transactions)

### **Telegram Bot**
- ✅ Non-blocking: semua handler harus async-friendly
- ✅ Timeout: max 10s untuk external API calls
- ✅ Retry: max 3x dengan exponential backoff
- ✅ User tiering: Free → USD+High only, Premium → all alerts
- ✅ Ban system: implement cooldowns untuk spam prevention

### **Deployment**
- ✅ Docker Compose untuk local & production
- ✅ Health check endpoint (`GET /health`) wajib
- ✅ Graceful shutdown: max 10s drain time
- ✅ Config via env vars (`.env` file, never commit secrets)
- ✅ Logging: structured JSON, log level via env

---

## 🎨 Architecture Overview

### **Layers**
```
┌─────────────────────────────────────────────────────────┐
│                    Telegram Adapter                      │
│  (bot handler, middleware, keyboards, chatbot mode)     │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                    Service Layer                         │
│  (COT, FRED, News, Price, AI, Strategy, Backtest)       │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                    Domain Layer                          │
│  (entities, value objects, domain events)               │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                    Storage Adapter                       │
│  (BadgerDB repos: COT, Event, Price, Signal, User)      │
└─────────────────────────────────────────────────────────┘
```

### **Key Packages**
| Package | Purpose |
|---------|---------|
| `internal/service/cot` | COT fetcher, analyzer, signal generation |
| `internal/service/fred` | FRED data fetcher, regime detection, alerts |
| `internal/service/news` | Economic calendar, surprise scoring, impact tracking |
| `internal/service/price` | Multi-source price fetcher (TwelveData → Yahoo → CoinGecko) |
| `internal/service/ai` | Gemini/Claude clients, prompt templates, caching |
| `internal/service/strategy` | Multi-factor signal confluence engine |
| `internal/service/factors` | Factor analysis (momentum, value, carry, etc.) |
| `internal/service/backtest` | Signal evaluation, calibration, performance metrics |
| `internal/adapter/telegram` | Bot handlers, middleware, keyboards |
| `internal/adapter/storage` | BadgerDB repositories (CRUD) |
| `internal/scheduler` | Background jobs (COT fetch, price sync, signal eval) |
| `internal/ports` | Interfaces (AIAnalyzer, PriceFetcher, Repository) |
| `internal/domain` | Entities (Signal, COTData, MacroData, User) |

---

## 📚 Reference

### **Documentation**
- [README.md](https://github.com/arkcode369/ark-intelligent) — Project overview
- [CHANGELOG.md](https://github.com/arkcode369/ark-intelligent/blob/main/CHANGELOG.md) — Version history
- `.env.example` — Configuration reference

### **APIs & Data Sources**
- **CFTC Socrata API**: Commitment of Traders data (weekly)
- **FRED API**: Federal Reserve economic data (yields, labor, inflation)
- **MQL5 Economic Calendar**: Real-time economic events
- **Bybit API**: Order book & microstructure data (optional)
- **TwelveData / AlphaVantage / Yahoo Finance**: Price data
- **Google Gemini / Anthropic Claude**: AI analysis

### **Key Concepts**
- **COT (Commitment of Traders)**: Weekly positioning data dari CFTC, used untuk detect institutional sentiment
- **FRED Regime**: Macro regime detection (inflation, labor, yield curve)
- **Surprise Scoring**: Actual vs consensus economic data, weighted by historical impact
- **Signal Calibration**: Platt scaling untuk convert raw scores ke calibrated probabilities
- **Hexagonal Architecture**: Ports & adapters pattern, business logic isolated from infra

---

## 🚀 Quick Commands

```bash
# Run tests dengan race detector
go test ./... -race -cover

# Run specific test
go test -run TestCOTAnalyzer_Analyze ./internal/service/cot

# Build binary
go build -o ark-intelligent ./cmd/bot

# Run with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f bot

# Check health endpoint
curl http://localhost:8080/health
```

---

## 🎯 Success Metrics

Agent berhasil jika:
- ✅ **Signal Accuracy**: Hit rate ≥60% untuk COT signals (backtested)
- ✅ **Data Reliability**: Fetch success rate ≥95% untuk semua data sources
- ✅ **System Uptime**: Bot online ≥99% (graceful degradation saat API down)
- ✅ **Test Coverage**: Core logic ≥80%, adapters ≥60%
- ✅ **Performance**: P95 response time <2s untuk Telegram commands
- ✅ **Maintainability**: No technical debt accumulation (refactor setiap sprint)

---

*This playbook evolves with the project. Update when you learn something new.*
