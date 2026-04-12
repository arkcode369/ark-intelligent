# ARK Intelligent

**Institutional-grade macro intelligence Telegram bot** for forex traders, delivering multi-strategy analysis from COT positioning, FRED macro data, economic calendar surprises, technical indicators, and AI-powered insights.

---

## 🎯 Core Features

### **COT Analysis**
- Weekly CFTC Commitment of Traders positioning with net change, percentile ranking, and signal detection
- Multi-factor confluence scoring combining COT + macro + technical data
- Regime-aware signal adaptation based on FRED macro conditions

### **Economic Calendar Intelligence**
- Real-time high-impact event tracking via MQL5 Economic Calendar
- Revision detection and surprise scoring (actual vs consensus)
- Historical impact tracking and cumulative surprise metrics

### **FRED Macro Dashboard**
- Federal Reserve economic data monitoring (yields, labor, inflation)
- Macro regime detection (inflation, labor market, yield curve)
- Regime change alerts with historical context

### **Technical Analysis Engine**
- Full TA library integration: RSI, MACD, Bollinger Bands, ATR, Fibonacci, Ichimoku, Supertrend
- Multi-timeframe analysis (daily + 4H intraday)
- Pattern recognition and divergence detection
- Support/resistance zone mapping

### **AI-Powered Insights**
- Multi-model support: Google Gemini, Anthropic Claude, custom models
- Macro narrative generation and weekly outlooks
- Directional bias analysis with confidence scoring
- Graceful template fallback when AI services unavailable

### **Signal & Backtest System**
- Multi-factor signal confluence engine
- Calibrated confidence scores (Platt scaling / isotonic regression)
- Walk-forward backtest with performance metrics (Sharpe, Sortino, max drawdown, hit rate)
- Signal performance tracking per regime

### **Price Data & Market Microstructure**
- Multi-source price fetcher with fallback chains (TwelveData → AlphaVantage → Yahoo → CoinGecko)
- Bybit order book and microstructure data (optional)
- Volatility modeling (GARCH, Hurst exponent)
- Seasonal patterns and correlation analysis

---

## 🏗️ Architecture

**Hexagonal (ports & adapters)** architecture in Go 1.22+:

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

### **Technology Stack**
- **Language**: Go 1.22+
- **Storage**: BadgerDB (embedded key-value, zero external dependencies)
- **Messaging**: Telegram Bot API (long-polling)
- **AI**: Google Gemini / Anthropic Claude with caching & rate limiting
- **Logging**: Structured JSON via zerolog
- **Testing**: Unit tests ≥80% coverage, race detector enabled
- **Deployment**: Docker Compose for local & production

---

## 🚀 Quick Start

### **1. Environment Setup**
```bash
cp .env.example .env
```

### **2. Required Variables**
| Variable | Description |
|----------|-------------|
| `BOT_TOKEN` | Telegram bot token from BotFather |
| `CHAT_ID` | Default Telegram chat ID for notifications |

### **3. Optional Variables**
| Variable | Default | Description |
|----------|---------|-------------|
| `GEMINI_API_KEY` | -- | Google Gemini API key |
| `CLAUDE_API_KEY` | -- | Anthropic Claude API key |
| `FRED_API_KEY` | -- | FRED API key (free at stlouisfed.org) |
| `GEMINI_MODEL` | `gemini-3.1-flash-lite-preview` | AI model name |
| `DATA_DIR` | `/app/data` | BadgerDB storage directory |
| `AI_CACHE_TTL` | `1h` | AI response cache duration |
| `AI_MAX_RPM` | `15` | AI requests per minute limit |
| `LOG_LEVEL` | `info` | Logging verbosity |

### **4. Run**
```bash
# Docker Compose (recommended)
docker-compose up -d

# Or build directly
go build -o ark-intelligent ./cmd/bot
./ark-intelligent
```

---

## 📱 Telegram Commands

### **Core Commands**
| Command | Description |
|---------|-------------|
| `/cot` | COT positioning summary (all currencies or single pair) |
| `/calendar` | Upcoming high-impact economic events |
| `/outlook` | AI-generated weekly macro outlook |
| `/bias` | Directional bias from COT + macro analysis |
| `/rank` | Currency strength ranking (COT + macro confluence) |
| `/macro` | FRED macro dashboard (yields, labor, inflation) |

### **Advanced Commands**
| Command | Description |
|---------|-------------|
| `/xfactors` | Cross-factor analysis and confluence scoring |
| `/playbook` | Trading playbook with setup templates |
| `/heat` | Market heat map (volatility, momentum, sentiment) |
| `/rankx` | Extended ranking with technical overlay |
| `/transition` | Regime transition tracking and alerts |
| `/cryptoalpha` | Crypto-specific macro signals |

### **Configuration**
| Command | Description |
|---------|-------------|
| `/prefs` | Configure personal alert preferences |
| `/help` | List all available commands |

---

## 📊 Data Sources

| Source | Data | Update Frequency |
|--------|------|------------------|
| CFTC Socrata API | Commitment of Traders reports | Weekly (Friday) |
| MQL5 Economic Calendar | Economic events and actuals | Real-time |
| FRED (St. Louis Fed) | Macro indicators | Varies by series |
| TwelveData / AlphaVantage / Yahoo | Price data | Real-time / Daily |
| Bybit API | Order book & microstructure | Real-time (optional) |
| Google Gemini / Anthropic Claude | AI analysis | On-demand |

---

## 🛡️ Core Principles

### **Data Integrity First**
- All external API data validated before persistence
- Graceful degradation with fallback chains
- Atomic writes for historical data protection

### **Signal Quality Over Quantity**
- Calibrated confidence scores (Platt scaling)
- Backtest before deploy new strategies
- Track performance per regime (hit rate, PnL, drawdown)

### **Defensive Programming**
- Retry with exponential backoff for all external calls
- Rate limiting with local TTL-based caching
- Circuit breaker pattern for timeout-prone services

### **Observability**
- Structured logging with full context
- Metrics: fetch success rate, signal accuracy, AI latency
- Health check endpoint at `:8080`

### **Modularity & Testability**
- Hexagonal architecture (business logic isolated from infra)
- Manual dependency injection (no framework)
- Unit tests for all analyzer/strategy logic

---

## 🧪 Testing & Validation

```bash
# Run all tests with race detector
go test ./... -race -cover

# Run specific test
go test -run TestCOTAnalyzer_Analyze ./internal/service/cot

# Static analysis
go vet ./...
```

**Coverage Targets:**
- Core logic: ≥80%
- Adapters: ≥60%

---

## 📈 Success Metrics

| Metric | Target |
|--------|--------|
| Signal Accuracy (COT) | Hit rate ≥60% (backtested) |
| Data Reliability | Fetch success rate ≥95% |
| System Uptime | ≥99% (graceful degradation) |
| Test Coverage | Core ≥80%, Adapters ≥60% |
| Performance | P95 response time <2s |
| Technical Debt | No accumulation (refactor each sprint) |

---

## 📚 Key Concepts

- **COT (Commitment of Traders)**: Weekly institutional positioning data from CFTC
- **FRED Regime**: Macro regime detection (inflation, labor, yield curve)
- **Surprise Scoring**: Actual vs consensus economic data, weighted by historical impact
- **Signal Calibration**: Platt scaling to convert raw scores to calibrated probabilities
- **Hexagonal Architecture**: Ports & adapters pattern, business logic isolated from infrastructure

---

## 📄 License

All rights reserved. See LICENSE file for details.

---

*Built with institutional-grade data and defensive engineering for serious forex traders.*
