# Volume Profile Implementation Plan — Institutional Grade

## 📊 Data Assessment

### What We Have
- **OHLCV bars** at 6 intraday TFs (15m/30m/1h/4h/6h/12h) + daily
- **Volume**: tick volume for FX, actual volume for futures/indices/crypto
- **~24 symbols** across FX, metals, energy, indices, bonds, crypto
- **Historical depth**: ~52 days intraday (15m base), ~730 days daily

### What We Don't Have
- ❌ Tick-by-tick data (no real footprint/delta volume)
- ❌ Bid/Ask separate volume (no order flow)
- ❌ Level 2 / order book data

### Implication
Levels 1-3 fully feasible. Level 4 partially feasible — we simulate delta & footprint using OHLCV heuristics (industry-standard approach when tick data unavailable).

---

## 🏗️ Architecture

### Command: `/vp EUR [timeframe]`
- Separate command (not inside /cta) — VP is complex enough to warrant its own space
- Default TF: daily. User can specify any TF.
- Keyboard with all analysis modes + TF selector

### Engine: `scripts/vp_engine.py`
- Same pattern as `quant_engine.py`: Go → JSON → Python → JSON + chart PNG
- Input: OHLCV bars (primary TF + optionally all TFs for composite)
- Output: VP metrics + chart + text analysis

### Go Handler: `handler_vp.go`
- State cache with TTL (same pattern as CTA/Quant)
- Keyboard: VP modes + TF selector

---

## 📐 Implementation Phases

### Phase 1 — Core VP (Level 1-2)
**Goal**: Basic volume profile with key levels

#### Computation
1. **Price binning** — divide price range into N bins (auto-scale based on ATR)
   - Bin count: `max(50, min(200, price_range / (ATR * 0.1)))`
   - Each bar distributes volume across bins it touches (proportional allocation)
   
2. **Key Levels**:
   - **POC** — bin with highest volume
   - **VAH/VAL** — expand from POC until 70% of total volume captured
   - **HVN** — bins with volume > mean + 1σ
   - **LVN** — bins with volume < mean - 0.5σ

3. **Chart**: candlestick + horizontal volume histogram (left side)
   - POC: bold red line
   - VAH/VAL: dashed blue lines  
   - Value Area: shaded zone
   - HVN: highlighted green bars in histogram
   - LVN: highlighted red bars in histogram
   - Current price marker

#### Multi-Timeframe (one TF at a time)
- Compute VP for selected TF
- Show all 7 TFs in keyboard for switching

#### Text Output
```
📊 Volume Profile: EUR/USD — 4H
━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 POC: 1.08234 (fair value)
🔵 VAH: 1.08456 (value area high)
🔵 VAL: 1.08012 (value area low)
📏 VA Width: 44.4 pips (68% of range)

🟢 HVN Zones (strong S/R):
  1.08200 - 1.08260 (cluster)
  1.07980 - 1.08040 (cluster)

🔴 LVN Zones (fast move areas):
  1.08350 - 1.08400 (thin)

💡 Price at 1.08300 — above POC, inside VA
   → Trading in accepted value zone
```

---

### Phase 2 — Session Analysis (Level 2-3)
**Goal**: Time-segmented VP + developing context

#### Computation
1. **Session VP** — split bars into sessions:
   - **Asian**: 00:00-08:00 UTC
   - **London**: 08:00-16:00 UTC  
   - **New York**: 13:00-22:00 UTC
   - **Daily**: each trading day as separate profile

2. **Developing POC** — POC computed incrementally as bars arrive
   - Track POC migration over time (trending vs mean-reverting)
   - POC velocity: how fast is POC moving?

3. **Naked POC Detection** — find POCs from previous sessions that price hasn't revisited
   - These act as magnets — 80%+ historical revisit rate

4. **Value Area Migration**:
   - VA shifting up = bullish auction
   - VA shifting down = bearish auction
   - VA expanding = increasing acceptance
   - VA contracting = narrowing balance

5. **Initial Balance (IB)** — first 1h range of each session
   - IB extension targets: 1.5x, 2x IB range
   - Wide IB = range day likely
   - Narrow IB = trend day likely

#### Chart
- Multi-session VP overlay (Asian/London/NY in different colors)
- Naked POCs marked with dotted lines
- IB range highlighted

#### Text Output
```
📊 Session Volume Profile: EUR/USD
━━━━━━━━━━━━━━━━━━━━━━━━━━
🌏 Asian (00-08 UTC):
  POC: 1.08200 | VA: 1.08150 - 1.08250
  
🇪🇺 London (08-16 UTC):
  POC: 1.08340 | VA: 1.08280 - 1.08400
  
🇺🇸 New York (13-22 UTC):
  POC: 1.08310 | VA: 1.08250 - 1.08380

📈 VA Migration: ↗️ BULLISH (London > Asian)
⚡ Naked POCs (unvisited magnets):
  1.08120 (2 sessions ago)
  1.07950 (4 sessions ago)

📏 Initial Balance: 1.08280 - 1.08380 (10 pips)
  → Narrow IB — trend day potential
  🎯 IB targets: 1.08430 (1.5x) / 1.08480 (2x)
```

---

### Phase 3 — Advanced Analytics (Level 3)
**Goal**: Profile shape, composite VP, VWAP

#### Computation
1. **Profile Shape Classification**:
   - **P-shape**: POC near top, heavy volume above mid → buying/long liquidation
   - **b-shape**: POC near bottom, heavy volume below mid → selling/short covering
   - **D-shape**: POC near center, balanced → consolidation/balance
   - **B-shape (double)**: two HVN clusters → two-timeframe market, breakout pending
   - **Thin/elongated**: volume evenly spread → trending, no consensus

2. **Composite VP** — merge VP from multiple sessions/days
   - Weekly composite: merge all daily VPs
   - Monthly composite: merge all weekly VPs
   - Detect composite HVN/LVN vs session HVN/LVN

3. **VWAP Bands**:
   - Session VWAP + 1σ, 2σ, 3σ bands
   - Anchored VWAP from swing highs/lows
   - Multi-session VWAP overlay

4. **TPO (Time Price Opportunity)**:
   - Instead of volume at price → count TIME at each price level
   - TPO count = how many bars touched each price bin
   - TPO POC vs Volume POC divergence = institutional vs retail focus

5. **Volume-at-Price momentum**:
   - Compare current VP shape to N-bar-ago VP
   - Detect volume migration (where is new volume building?)
   - Identify absorption (heavy volume, no price move) vs breakout (volume + price move)

#### Chart
- Dual histogram: volume (left) + TPO count (right)
- VWAP bands overlay
- Profile shape label

---

### Phase 4 — Institutional Analysis (Level 4)
**Goal**: Auction theory, simulated delta, confluence

#### Computation
1. **Auction Market Theory Signals**:
   - **Balance area** = price rotating around POC within VA → fade extremes
   - **Imbalance** = price trending outside VA → follow momentum
   - **Initiative activity** = price moves away from value → new information
   - **Responsive activity** = price returns to value → mean reversion

2. **Simulated Delta Volume** (from OHLCV heuristics):
   - For each bar: `delta ≈ volume × (close - open) / (high - low)`
   - Positive delta = net buying pressure
   - Negative delta = net selling pressure
   - Cumulative delta divergence vs price = institutional positioning

3. **Excess & Poor Structure**:
   - **Excess high/low**: strong rejection tail with heavy volume → genuine reversal
   - **Poor high/low**: weak extreme, no rejection → likely revisit
   - **Single prints**: price levels touched only once → breakout/gap zones

4. **Composite Confluence Scoring**:
   - Where do VP levels from multiple TFs align?
   - Score each price zone by number of confluent VP signals:
     * POC from different TFs overlapping = strongest S/R
     * VAH from one TF = VAL from another = confluence zone
     * HVN cluster across TFs = institutional accumulation zone
   - Output: ranked list of key price levels with confluence score

5. **Market Profile Distribution**:
   - **Normal distribution** (68% in VA) → balanced, fair value found
   - **Skewed distribution** → directional, value being discovered
   - **Bimodal** → two separate value areas, breakout expected
   - Kurtosis: fat-tailed = excess, thin-tailed = trend

6. **Volume Imbalance Detection**:
   - Compare buy delta vs sell delta at each price level
   - Imbalance ratio > 3:1 at a level = institutional order
   - Stack of imbalances = aggressive institutional campaign

#### Chart
- Full institutional chart: price + VP histogram + cumulative delta + VWAP
- Confluence zones highlighted with strength colors
- Excess/poor structure annotations

#### Text Output
```
📊 Institutional VP: EUR/USD — 4H
━━━━━━━━━━━━━━━━━━━━━━━━━━

🏛 AUCTION STATE: BALANCE → VA rotation
  Price inside VA, responding to extremes
  → Fade strategy: sell VAH, buy VAL

📊 Profile Shape: D-shape (balanced)
  Distribution: Normal (kurtosis 2.8)
  → Fair value accepted at current level

📈 Simulated Delta:
  Cumulative: +2,340 (net buyers)
  Delta divergence: ⚠️ Price flat, delta rising
  → Hidden buying — potential bullish breakout

🔴 Structure Alerts:
  Poor High at 1.08456 → likely revisit up
  Excess Low at 1.07890 → genuine support

🎯 Confluence Zones (multi-TF):
  ★★★★★ 1.08200-1.08240: POC(4h)+POC(1h)+HVN(daily)
  ★★★★  1.08400-1.08460: VAH(4h)+VAH(daily)
  ★★★   1.07980-1.08020: VAL(4h)+HVN(1h)

⚡ Naked POCs:
  1.08120 — 3 sessions, strong magnet
  1.07850 — 7 sessions, decaying
```

---

## 📱 Keyboard Layout

```
/vp EUR
├── 📊 Profile       — main VP chart + levels
├── 🕐 Session       — Asian/London/NY split
├── 📐 Shape         — profile shape analysis
├── 🔀 Composite     — multi-day merged VP
├── 📏 VWAP          — VWAP bands
├── ⏱ TPO            — time-at-price
├── 📈 Delta         — simulated volume delta
├── 🏛 Auction       — auction theory state
├── 🎯 Confluence    — multi-TF level confluence
├── 📋 Full Report   — everything combined
├── [TF selector: 15m 30m 1h 4h 6h 12h Daily]
└── [🔄 Refresh]
```

---

## 📁 Files to Create/Modify

### New Files
- `scripts/vp_engine.py` — Python VP computation engine (~1500-2000 lines)
- `internal/adapter/telegram/handler_vp.go` — Go handler + callback
- `internal/service/ta/volume_profile.go` — Go-side VP types (optional, for type safety)

### Modified Files
- `internal/adapter/telegram/keyboard.go` — `VPMenu()`, `VPDetailMenu()`
- `internal/adapter/telegram/handler.go` — register `/vp` command
- `cmd/bot/main.go` — wire VP handler with repos

---

## ⏱ Estimated Effort

| Phase | Complexity | Estimate |
|-------|-----------|----------|
| Phase 1 — Core VP | Medium | ~45 min |
| Phase 2 — Sessions | Medium | ~30 min |
| Phase 3 — Advanced | High | ~45 min |
| Phase 4 — Institutional | High | ~45 min |
| **Total** | | **~3 hours** |

---

## ⚠️ Limitations & Honest Assessment

1. **Tick volume ≠ Real volume** for FX — tick volume correlates (~0.85) with real volume but isn't exact. VP levels will be approximate for FX pairs. For futures/indices/crypto with real volume, VP is accurate.

2. **Simulated delta** from OHLCV is a heuristic, not real order flow. Accuracy ~60-70%. Good for detecting trends, not precise enough for scalping decisions.

3. **No footprint chart** possible without tick data — we use OHLCV delta as proxy.

4. **Session splits** may miss overnight gaps — we handle this by using bar timestamps.

5. **Initial Balance** only meaningful for intraday TFs (15m-1h). Daily/12h too coarse.

These limitations are standard for any system without direct exchange tick data. Institutional traders use the same heuristics when tick feeds aren't available.
