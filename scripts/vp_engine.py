#!/usr/bin/env python3
"""
Volume Profile Engine — Institutional-Grade Analysis.

Usage: python3 vp_engine.py <input.json> <output.json> [chart_output.png]

Modes: profile, session, shape, composite, vwap, tpo, delta, auction, confluence, full

Input JSON:
  { mode, symbol, timeframe, bars[],
    all_tf_bars: { "15m": [...], "30m": [...], ... },  # for confluence/composite
    params: { lookback, va_pct, bin_count } }

Output JSON:
  { mode, symbol, success, error, result{}, chart_path, text_output }
"""

import json
import sys
import warnings
import traceback
from datetime import datetime, timedelta

import numpy as np
import pandas as pd

warnings.filterwarnings("ignore")

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as mticker
from matplotlib.patches import FancyBboxPatch
import matplotlib.dates as mdates

# ---------------------------------------------------------------------------
# Theme (matches quant_engine.py)
# ---------------------------------------------------------------------------
BG_COLOR = "#0e1117"
TEXT_COLOR = "#c9d1d9"
GRID_COLOR = "#1e2530"
UP_COLOR = "#26a69a"
DOWN_COLOR = "#ef5350"
ACCENT1 = "#42A5F5"
ACCENT2 = "#FFD700"
ACCENT3 = "#AB47BC"
ACCENT4 = "#FF8C00"
POC_COLOR = "#FF4444"
VAH_COLOR = "#42A5F5"
VAL_COLOR = "#42A5F5"
HVN_COLOR = "#26a69a"
LVN_COLOR = "#ef5350"
VWAP_COLOR = "#FFD700"

plt.rcParams.update({
    "figure.facecolor": BG_COLOR,
    "axes.facecolor": BG_COLOR,
    "axes.edgecolor": GRID_COLOR,
    "axes.labelcolor": TEXT_COLOR,
    "xtick.color": TEXT_COLOR,
    "ytick.color": TEXT_COLOR,
    "text.color": TEXT_COLOR,
    "grid.color": GRID_COLOR,
    "grid.alpha": 0.3,
    "legend.facecolor": BG_COLOR,
    "legend.edgecolor": GRID_COLOR,
    "font.size": 9,
})


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def safe_float(v):
    """Convert to JSON-safe float."""
    if v is None or (isinstance(v, float) and (np.isnan(v) or np.isinf(v))):
        return None
    return round(float(v), 8)


def bars_to_df(bars):
    """Convert list of bar dicts to DataFrame with OHLCV."""
    df = pd.DataFrame(bars)
    df["date"] = pd.to_datetime(df["date"])
    df = df.sort_values("date").set_index("date")
    for col in ["open", "high", "low", "close"]:
        df[col] = pd.to_numeric(df[col], errors="coerce")
    if "volume" in df.columns:
        df["volume"] = pd.to_numeric(df["volume"], errors="coerce").fillna(0)
    else:
        df["volume"] = 0.0
    df.rename(columns={"open": "Open", "high": "High", "low": "Low",
                        "close": "Close", "volume": "Volume"}, inplace=True)
    df.dropna(subset=["Open", "High", "Low", "Close"], inplace=True)
    return df


def save_chart(fig, path):
    """Save chart to file and close."""
    fig.savefig(path, dpi=150, bbox_inches="tight", facecolor=BG_COLOR)
    plt.close(fig)


def output(mode, symbol, success, result, text, error="", chart_path=""):
    """Build output dict."""
    return {
        "mode": mode,
        "symbol": symbol,
        "success": success,
        "error": error,
        "result": result,
        "text_output": text,
        "chart_path": chart_path,
    }


def compute_atr(df, period=14):
    """Compute Average True Range."""
    high = df["High"]
    low = df["Low"]
    close = df["Close"]
    tr1 = high - low
    tr2 = (high - close.shift(1)).abs()
    tr3 = (low - close.shift(1)).abs()
    tr = pd.concat([tr1, tr2, tr3], axis=1).max(axis=1)
    return tr.rolling(period).mean().iloc[-1]


def price_decimals(price):
    """Determine appropriate decimal places for a price."""
    if price > 1000:
        return 2
    elif price > 10:
        return 3
    elif price > 1:
        return 4
    else:
        return 5


def format_price(price, decimals=None):
    """Format price with appropriate decimals."""
    if decimals is None:
        decimals = price_decimals(price)
    return f"{price:.{decimals}f}"


def pip_value(symbol):
    """Return pip size for a symbol."""
    sym = symbol.upper()
    if "JPY" in sym:
        return 0.01
    if any(x in sym for x in ["XAU", "GOLD"]):
        return 0.1
    if any(x in sym for x in ["XAG", "SILVER"]):
        return 0.01
    if any(x in sym for x in ["OIL", "CL", "COPPER"]):
        return 0.01
    if any(x in sym for x in ["SPX", "NDX", "DJI", "RUT", "ES", "NQ"]):
        return 1.0
    if any(x in sym for x in ["BTC", "ETH"]):
        return 1.0
    if any(x in sym for x in ["BOND"]):
        return 0.01
    return 0.0001  # default forex


# ===========================================================================
# CORE: Volume Profile Computation
# ===========================================================================

def compute_volume_profile(df, n_bins=None, va_pct=0.70):
    """
    Compute volume profile from OHLCV data.

    Returns dict with:
      bins, volumes, poc, vah, val, hvn_zones, lvn_zones, total_volume
    """
    if len(df) < 10:
        return None

    price_high = df["High"].max()
    price_low = df["Low"].min()
    price_range = price_high - price_low

    if price_range <= 0:
        return None

    # Auto-determine bin count based on ATR
    if n_bins is None:
        atr = compute_atr(df)
        if atr and atr > 0:
            n_bins = int(np.clip(price_range / (atr * 0.08), 50, 250))
        else:
            n_bins = 100

    # Create price bins
    bin_edges = np.linspace(price_low, price_high, n_bins + 1)
    bin_centers = (bin_edges[:-1] + bin_edges[1:]) / 2
    bin_width = bin_edges[1] - bin_edges[0]
    volumes = np.zeros(n_bins)

    # Distribute volume across bins proportionally
    for _, bar in df.iterrows():
        bar_low = bar["Low"]
        bar_high = bar["High"]
        bar_vol = bar["Volume"]

        if bar_vol <= 0:
            bar_vol = 1.0  # tick proxy: count each bar as 1 unit

        if bar_high <= bar_low:
            # Zero-range bar: assign all volume to closest bin
            idx = np.searchsorted(bin_edges, bar["Close"]) - 1
            idx = np.clip(idx, 0, n_bins - 1)
            volumes[idx] += bar_vol
            continue

        # Find bins touched by this bar
        low_bin = np.searchsorted(bin_edges, bar_low) - 1
        high_bin = np.searchsorted(bin_edges, bar_high) - 1
        low_bin = np.clip(low_bin, 0, n_bins - 1)
        high_bin = np.clip(high_bin, 0, n_bins - 1)

        if low_bin == high_bin:
            volumes[low_bin] += bar_vol
        else:
            # Proportional distribution
            bar_range = bar_high - bar_low
            for b in range(low_bin, high_bin + 1):
                bin_lo = max(bin_edges[b], bar_low)
                bin_hi = min(bin_edges[b + 1], bar_high)
                overlap = max(0, bin_hi - bin_lo)
                fraction = overlap / bar_range
                volumes[b] += bar_vol * fraction

    total_volume = volumes.sum()
    if total_volume <= 0:
        return None

    # POC — bin with highest volume
    poc_idx = np.argmax(volumes)
    poc_price = bin_centers[poc_idx]

    # Value Area — expand from POC until va_pct of total volume
    va_target = total_volume * va_pct
    va_volume = volumes[poc_idx]
    va_low_idx = poc_idx
    va_high_idx = poc_idx

    while va_volume < va_target:
        expand_up = volumes[va_high_idx + 1] if va_high_idx + 1 < n_bins else 0
        expand_down = volumes[va_low_idx - 1] if va_low_idx - 1 >= 0 else 0

        if expand_up == 0 and expand_down == 0:
            break

        if expand_up >= expand_down:
            va_high_idx += 1
            va_volume += expand_up
        else:
            va_low_idx -= 1
            va_volume += expand_down

    vah = bin_edges[min(va_high_idx + 1, n_bins)]
    val = bin_edges[va_low_idx]

    # HVN — bins with volume > mean + 1σ
    vol_mean = volumes.mean()
    vol_std = volumes.std()
    hvn_threshold = vol_mean + vol_std
    lvn_threshold = max(vol_mean - 0.5 * vol_std, 0)

    # Cluster adjacent HVN/LVN bins into zones
    hvn_zones = _cluster_zones(volumes, bin_edges, bin_centers, hvn_threshold, "above")
    lvn_zones = _cluster_zones(volumes, bin_edges, bin_centers, lvn_threshold, "below")

    return {
        "bin_edges": bin_edges,
        "bin_centers": bin_centers,
        "bin_width": bin_width,
        "volumes": volumes,
        "poc_price": float(poc_price),
        "poc_idx": int(poc_idx),
        "vah": float(vah),
        "val": float(val),
        "va_pct_actual": float(va_volume / total_volume) if total_volume > 0 else 0,
        "hvn_zones": hvn_zones,
        "lvn_zones": lvn_zones,
        "total_volume": float(total_volume),
        "n_bins": n_bins,
        "hvn_threshold": float(hvn_threshold),
        "lvn_threshold": float(lvn_threshold),
    }


def _cluster_zones(volumes, bin_edges, bin_centers, threshold, direction):
    """Cluster adjacent bins into contiguous zones."""
    zones = []
    if direction == "above":
        mask = volumes > threshold
    else:
        # LVN: below threshold AND non-zero (truly thin, not empty)
        mask = (volumes < threshold) & (volumes > 0)

    in_zone = False
    zone_start = None
    zone_vol = 0

    for i in range(len(volumes)):
        if mask[i]:
            if not in_zone:
                zone_start = i
                in_zone = True
                zone_vol = 0
            zone_vol += volumes[i]
        else:
            if in_zone:
                zones.append({
                    "low": float(bin_edges[zone_start]),
                    "high": float(bin_edges[i]),
                    "mid": float((bin_edges[zone_start] + bin_edges[i]) / 2),
                    "volume": float(zone_vol),
                })
                in_zone = False
    if in_zone:
        zones.append({
            "low": float(bin_edges[zone_start]),
            "high": float(bin_edges[len(volumes)]),
            "mid": float((bin_edges[zone_start] + bin_edges[len(volumes)]) / 2),
            "volume": float(zone_vol),
        })

    # Sort by volume descending
    zones.sort(key=lambda z: z["volume"], reverse=True)
    return zones


# ===========================================================================
# MODE: PROFILE — Core VP Chart + Levels
# ===========================================================================

def compute_profile(df, symbol, timeframe, params, chart_path=None):
    """Basic Volume Profile — POC, VAH/VAL, HVN/LVN with chart."""
    n = len(df)
    if n < 20:
        return output("profile", symbol, False, {}, "",
                       error="Minimal 20 bar untuk Volume Profile")

    va_pct = params.get("va_pct", 0.70)
    n_bins = params.get("bin_count", None)
    vp = compute_volume_profile(df, n_bins=n_bins, va_pct=va_pct)
    if vp is None:
        return output("profile", symbol, False, {}, "",
                       error="Tidak dapat menghitung Volume Profile")

    current_price = df["Close"].iloc[-1]
    dec = price_decimals(current_price)
    pip = pip_value(symbol)

    # Position relative to VP
    if current_price > vp["vah"]:
        position = "above VA"
        position_emoji = "⬆️"
        position_advice = "Di atas Value Area — potential breakout atau pullback ke VAH"
    elif current_price < vp["val"]:
        position = "below VA"
        position_emoji = "⬇️"
        position_advice = "Di bawah Value Area — potential breakdown atau bounce ke VAL"
    elif abs(current_price - vp["poc_price"]) / vp["poc_price"] < 0.001:
        position = "at POC"
        position_emoji = "🎯"
        position_advice = "Di POC (fair value) — market in balance"
    else:
        position = "inside VA"
        position_emoji = "↔️"
        position_advice = "Dalam Value Area — trading di zona accepted price"

    va_width_pips = (vp["vah"] - vp["val"]) / pip
    poc_dist_pips = (current_price - vp["poc_price"]) / pip

    result = {
        "poc": safe_float(vp["poc_price"]),
        "vah": safe_float(vp["vah"]),
        "val": safe_float(vp["val"]),
        "va_width_pips": safe_float(va_width_pips),
        "va_pct_actual": safe_float(vp["va_pct_actual"]),
        "current_price": safe_float(current_price),
        "position": position,
        "hvn_zones": [{"low": safe_float(z["low"]), "high": safe_float(z["high"]),
                        "volume": safe_float(z["volume"])} for z in vp["hvn_zones"][:5]],
        "lvn_zones": [{"low": safe_float(z["low"]), "high": safe_float(z["high"]),
                        "volume": safe_float(z["volume"])} for z in vp["lvn_zones"][:5]],
        "total_volume": safe_float(vp["total_volume"]),
        "n_bins": vp["n_bins"],
        "n_bars": n,
    }

    # Chart
    if chart_path:
        _draw_profile_chart(df, vp, symbol, timeframe, chart_path, current_price)

    # Text
    text = f"""📊 <b>Volume Profile: {symbol} — {timeframe}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 POC: {format_price(vp["poc_price"], dec)} (fair value)
🔵 VAH: {format_price(vp["vah"], dec)}
🔵 VAL: {format_price(vp["val"], dec)}
📏 VA Width: {va_width_pips:.1f} pips ({vp["va_pct_actual"]*100:.0f}% volume)
📊 {n} bars, {vp["n_bins"]} price bins
"""
    if vp["hvn_zones"]:
        text += "\n🟢 <b>HVN Zones (strong S/R):</b>\n"
        for z in vp["hvn_zones"][:4]:
            text += f"  {format_price(z['low'], dec)} — {format_price(z['high'], dec)}\n"

    if vp["lvn_zones"]:
        text += "\n🔴 <b>LVN Zones (fast move areas):</b>\n"
        for z in vp["lvn_zones"][:4]:
            text += f"  {format_price(z['low'], dec)} — {format_price(z['high'], dec)}\n"

    text += f"""
{position_emoji} Price {format_price(current_price, dec)} — <b>{position}</b>
  POC distance: {poc_dist_pips:+.1f} pips
💡 {position_advice}"""

    return output("profile", symbol, True, result, text, chart_path=chart_path or "")


def _draw_profile_chart(df, vp, symbol, timeframe, chart_path, current_price):
    """Draw candlestick + horizontal volume histogram chart."""
    fig, (ax_candle, ax_vol) = plt.subplots(
        1, 2, figsize=(16, 8), gridspec_kw={"width_ratios": [3, 1]}, sharey=True)

    fig.suptitle(f"{symbol} — Volume Profile — {timeframe} ({len(df)} bars)",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    # Candlestick chart
    dates = np.arange(len(df))
    opens = df["Open"].values
    highs = df["High"].values
    lows = df["Low"].values
    closes = df["Close"].values

    for i in range(len(df)):
        color = UP_COLOR if closes[i] >= opens[i] else DOWN_COLOR
        # Wick
        ax_candle.plot([dates[i], dates[i]], [lows[i], highs[i]],
                       color=color, linewidth=0.5)
        # Body
        body_lo = min(opens[i], closes[i])
        body_hi = max(opens[i], closes[i])
        body_h = max(body_hi - body_lo, (highs[i] - lows[i]) * 0.001)
        ax_candle.bar(dates[i], body_h, bottom=body_lo, width=0.6,
                      color=color, alpha=0.9, edgecolor="none")

    # VP overlay lines on candlestick
    ax_candle.axhline(vp["poc_price"], color=POC_COLOR, linewidth=1.5,
                       linestyle="-", alpha=0.9, label=f"POC {format_price(vp['poc_price'])}")
    ax_candle.axhline(vp["vah"], color=VAH_COLOR, linewidth=1,
                       linestyle="--", alpha=0.7, label=f"VAH {format_price(vp['vah'])}")
    ax_candle.axhline(vp["val"], color=VAL_COLOR, linewidth=1,
                       linestyle="--", alpha=0.7, label=f"VAL {format_price(vp['val'])}")

    # Value Area shading
    ax_candle.axhspan(vp["val"], vp["vah"], alpha=0.05, color=VAH_COLOR)

    # Current price
    ax_candle.axhline(current_price, color=ACCENT2, linewidth=0.8,
                       linestyle=":", alpha=0.8)

    # x-axis: show some date labels
    n_labels = min(8, len(df))
    label_indices = np.linspace(0, len(df) - 1, n_labels, dtype=int)
    ax_candle.set_xticks(label_indices)
    ax_candle.set_xticklabels([df.index[i].strftime("%m/%d") for i in label_indices],
                               rotation=45, fontsize=7)
    ax_candle.set_xlim(-1, len(df))
    ax_candle.grid(True, alpha=0.2)
    ax_candle.legend(fontsize=7, loc="upper left", facecolor=BG_COLOR, edgecolor=GRID_COLOR)

    # Volume histogram (horizontal)
    volumes = vp["volumes"]
    bin_centers = vp["bin_centers"]
    max_vol = volumes.max()

    # Color bars by HVN/LVN
    colors = []
    for i in range(len(volumes)):
        if volumes[i] > vp["hvn_threshold"]:
            colors.append(HVN_COLOR)
        elif volumes[i] < vp["lvn_threshold"] and volumes[i] > 0:
            colors.append(LVN_COLOR)
        else:
            colors.append("#4a5568")

    ax_vol.barh(bin_centers, volumes, height=vp["bin_width"] * 0.9,
                color=colors, alpha=0.7, edgecolor="none")

    # POC/VAH/VAL lines on histogram too
    ax_vol.axhline(vp["poc_price"], color=POC_COLOR, linewidth=1.5, linestyle="-")
    ax_vol.axhline(vp["vah"], color=VAH_COLOR, linewidth=1, linestyle="--", alpha=0.7)
    ax_vol.axhline(vp["val"], color=VAL_COLOR, linewidth=1, linestyle="--", alpha=0.7)
    ax_vol.axhspan(vp["val"], vp["vah"], alpha=0.05, color=VAH_COLOR)

    ax_vol.set_xlabel("Volume", fontsize=8)
    ax_vol.grid(True, alpha=0.2)
    ax_vol.yaxis.set_visible(False)

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: SESSION — Asian/London/New York Session VP
# ===========================================================================

def compute_session(df, symbol, timeframe, params, chart_path=None):
    """Session Volume Profile — split by trading session."""
    n = len(df)
    if n < 30:
        return output("session", symbol, False, {}, "",
                       error="Minimal 30 bar untuk Session VP")

    # Ensure index is datetime
    df_utc = df.copy()
    if df_utc.index.tz is None:
        df_utc.index = pd.to_datetime(df_utc.index)

    # Define sessions (UTC hours)
    sessions = {
        "Asian": (0, 8),
        "London": (8, 16),
        "New York": (13, 22),
    }

    session_vps = {}
    session_dfs = {}
    dec = price_decimals(df["Close"].iloc[-1])

    for name, (start_h, end_h) in sessions.items():
        hours = df_utc.index.hour
        if start_h < end_h:
            mask = (hours >= start_h) & (hours < end_h)
        else:
            mask = (hours >= start_h) | (hours < end_h)

        session_df = df_utc[mask]
        if len(session_df) < 10:
            continue

        session_dfs[name] = session_df
        vp = compute_volume_profile(session_df, n_bins=80, va_pct=0.70)
        if vp is not None:
            session_vps[name] = vp

    if not session_vps:
        return output("session", symbol, False, {}, "",
                       error="Tidak cukup data per session")

    # Naked POC detection — find POCs from previous days not revisited
    naked_pocs = _find_naked_pocs(df, n_days=10)

    # Value Area migration
    va_migration = _compute_va_migration(session_vps)

    # Initial Balance (first 1h of each session today)
    ib = _compute_initial_balance(df_utc)

    current_price = df["Close"].iloc[-1]

    result = {
        "sessions": {},
        "naked_pocs": naked_pocs,
        "va_migration": va_migration,
        "initial_balance": ib,
    }

    text = f"""🕐 <b>Session Volume Profile: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
"""
    session_emojis = {"Asian": "🌏", "London": "🇪🇺", "New York": "🇺🇸"}

    for name in ["Asian", "London", "New York"]:
        if name not in session_vps:
            continue
        vp = session_vps[name]
        emoji = session_emojis.get(name, "📊")
        text += f"{emoji} <b>{name}</b>:\n"
        text += f"  POC: {format_price(vp['poc_price'], dec)}"
        text += f" | VA: {format_price(vp['val'], dec)} — {format_price(vp['vah'], dec)}\n"
        text += f"  Bars: {len(session_dfs[name])}, Volume: {vp['total_volume']:.0f}\n\n"

        result["sessions"][name] = {
            "poc": safe_float(vp["poc_price"]),
            "vah": safe_float(vp["vah"]),
            "val": safe_float(vp["val"]),
            "total_volume": safe_float(vp["total_volume"]),
        }

    # VA Migration
    if va_migration:
        direction_emoji = "↗️" if va_migration["direction"] == "BULLISH" else (
            "↘️" if va_migration["direction"] == "BEARISH" else "↔️")
        text += f"📈 <b>VA Migration:</b> {direction_emoji} {va_migration['direction']}\n"
        if va_migration.get("detail"):
            text += f"  {va_migration['detail']}\n"
        text += "\n"

    # Naked POCs
    if naked_pocs:
        text += "⚡ <b>Naked POCs (unvisited magnets):</b>\n"
        for np_info in naked_pocs[:5]:
            text += f"  {format_price(np_info['price'], dec)} ({np_info['age']} sessions ago)\n"
        text += "\n"

    # Initial Balance
    if ib and ib.get("valid"):
        ib_range_pips = (ib["high"] - ib["low"]) / pip_value(symbol)
        ib_type = "Narrow" if ib_range_pips < 20 else ("Wide" if ib_range_pips > 50 else "Normal")
        ib_implication = {
            "Narrow": "trend day potential",
            "Wide": "range day likely",
            "Normal": "neutral — watch for breakout direction"
        }
        text += f"📏 <b>Initial Balance:</b> {format_price(ib['low'], dec)} — {format_price(ib['high'], dec)}\n"
        text += f"  Range: {ib_range_pips:.1f} pips ({ib_type})\n"
        text += f"  → {ib_implication[ib_type]}\n"
        text += f"  🎯 Targets: {format_price(ib['target_1_5x'], dec)} (1.5x)"
        text += f" / {format_price(ib['target_2x'], dec)} (2x)\n"

    # Chart
    if chart_path:
        _draw_session_chart(df, session_vps, session_dfs, naked_pocs,
                            symbol, timeframe, chart_path, current_price)

    return output("session", symbol, True, result, text, chart_path=chart_path or "")


def _find_naked_pocs(df, n_days=10):
    """Find POCs from previous trading days that haven't been revisited."""
    naked = []
    df_copy = df.copy()
    df_copy["trade_date"] = df_copy.index.date

    dates = sorted(df_copy["trade_date"].unique())
    if len(dates) < 2:
        return naked

    # Compute POC for each day
    day_pocs = []
    for d in dates:
        day_df = df_copy[df_copy["trade_date"] == d]
        if len(day_df) < 5:
            continue
        vp = compute_volume_profile(day_df, n_bins=50, va_pct=0.70)
        if vp is not None:
            day_pocs.append({"date": d, "poc": vp["poc_price"]})

    if len(day_pocs) < 2:
        return naked

    # Check if each historical POC has been revisited
    current_date = dates[-1]
    for i in range(len(day_pocs) - 1):  # skip current day
        poc_info = day_pocs[i]
        poc_price = poc_info["poc"]

        # Check all bars after this day
        future_bars = df_copy[df_copy["trade_date"] > poc_info["date"]]
        if len(future_bars) == 0:
            continue

        # Check if price crossed through this POC
        touched = ((future_bars["Low"] <= poc_price) & (future_bars["High"] >= poc_price)).any()
        if not touched:
            age = len([d for d in dates if d > poc_info["date"]])
            naked.append({
                "price": poc_price,
                "date": str(poc_info["date"]),
                "age": age,
            })

    # Sort by proximity to current price
    current = df["Close"].iloc[-1]
    naked.sort(key=lambda x: abs(x["price"] - current))
    return naked[:10]


def _compute_va_migration(session_vps):
    """Determine if Value Area is migrating up/down across sessions."""
    ordered = []
    for name in ["Asian", "London", "New York"]:
        if name in session_vps:
            vp = session_vps[name]
            ordered.append({"name": name, "poc": vp["poc_price"],
                            "vah": vp["vah"], "val": vp["val"]})

    if len(ordered) < 2:
        return {"direction": "INSUFFICIENT", "detail": "Need at least 2 sessions"}

    # Compare last two sessions
    prev = ordered[-2]
    curr = ordered[-1]

    poc_shift = curr["poc"] - prev["poc"]
    va_mid_shift = ((curr["vah"] + curr["val"]) / 2) - ((prev["vah"] + prev["val"]) / 2)

    if poc_shift > 0 and va_mid_shift > 0:
        direction = "BULLISH"
        detail = f"{curr['name']} VA lebih tinggi dari {prev['name']} — bullish auction"
    elif poc_shift < 0 and va_mid_shift < 0:
        direction = "BEARISH"
        detail = f"{curr['name']} VA lebih rendah dari {prev['name']} — bearish auction"
    else:
        direction = "MIXED"
        detail = "VA shift tidak konsisten — rotation/balance"

    return {
        "direction": direction,
        "detail": detail,
        "poc_shift": safe_float(poc_shift),
        "va_mid_shift": safe_float(va_mid_shift),
    }


def _compute_initial_balance(df_utc):
    """Compute Initial Balance — first 1h range of today's session."""
    today = df_utc.index[-1].date()
    today_bars = df_utc[df_utc.index.date == today]
    if len(today_bars) < 2:
        return {"valid": False}

    # First hour bars
    first_time = today_bars.index[0]
    first_hour_end = first_time + timedelta(hours=1)
    first_hour = today_bars[today_bars.index <= first_hour_end]

    if len(first_hour) < 1:
        return {"valid": False}

    ib_high = first_hour["High"].max()
    ib_low = first_hour["Low"].min()
    ib_range = ib_high - ib_low

    return {
        "valid": True,
        "high": float(ib_high),
        "low": float(ib_low),
        "range": float(ib_range),
        "target_1_5x": float(ib_high + ib_range * 0.5),
        "target_2x": float(ib_high + ib_range),
        "target_1_5x_down": float(ib_low - ib_range * 0.5),
        "target_2x_down": float(ib_low - ib_range),
    }


def _draw_session_chart(df, session_vps, session_dfs, naked_pocs,
                         symbol, timeframe, chart_path, current_price):
    """Draw multi-session VP overlay chart."""
    fig, (ax, ax_vol) = plt.subplots(
        1, 2, figsize=(16, 8), gridspec_kw={"width_ratios": [3, 1]}, sharey=True)

    fig.suptitle(f"{symbol} — Session Volume Profile — {timeframe}",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    # Price line
    ax.plot(range(len(df)), df["Close"].values, color=ACCENT1, linewidth=1, alpha=0.8)

    # VP histograms per session (stacked horizontally with different colors)
    session_colors = {"Asian": "#FFD700", "London": "#42A5F5", "New York": "#ef5350"}

    for name in ["Asian", "London", "New York"]:
        if name not in session_vps:
            continue
        vp = session_vps[name]
        color = session_colors.get(name, "#888")
        ax_vol.barh(vp["bin_centers"], vp["volumes"], height=vp["bin_width"] * 0.9,
                     color=color, alpha=0.4, label=name, edgecolor="none")

        # POC line
        ax.axhline(vp["poc_price"], color=color, linewidth=1, linestyle="-", alpha=0.7)
        ax_vol.axhline(vp["poc_price"], color=color, linewidth=1, linestyle="-", alpha=0.7)

    # Naked POCs
    for np_info in (naked_pocs or [])[:5]:
        ax.axhline(np_info["price"], color=ACCENT3, linewidth=0.8,
                    linestyle=":", alpha=0.6)

    # Current price
    ax.axhline(current_price, color=ACCENT2, linewidth=0.8, linestyle=":", alpha=0.8)

    ax.set_xlim(-1, len(df))
    ax.grid(True, alpha=0.2)
    ax_vol.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
    ax_vol.set_xlabel("Volume", fontsize=8)
    ax_vol.yaxis.set_visible(False)
    ax_vol.grid(True, alpha=0.2)

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: SHAPE — Profile Shape Classification
# ===========================================================================

def compute_shape(df, symbol, timeframe, params, chart_path=None):
    """Classify VP shape: P, b, D, B, thin."""
    n = len(df)
    if n < 20:
        return output("shape", symbol, False, {}, "",
                       error="Minimal 20 bar untuk shape analysis")

    vp = compute_volume_profile(df, va_pct=0.70)
    if vp is None:
        return output("shape", symbol, False, {}, "",
                       error="Tidak dapat menghitung Volume Profile")

    volumes = vp["volumes"]
    bin_centers = vp["bin_centers"]
    n_bins = len(volumes)
    total_vol = volumes.sum()

    price_mid = (df["High"].max() + df["Low"].min()) / 2
    poc_position = (vp["poc_price"] - df["Low"].min()) / (df["High"].max() - df["Low"].min())

    # Volume distribution analysis
    upper_half = volumes[n_bins // 2:].sum()
    lower_half = volumes[:n_bins // 2].sum()
    vol_skew = (upper_half - lower_half) / total_vol if total_vol > 0 else 0

    # Check for bimodal (B-shape) — two distinct peaks
    from scipy.signal import find_peaks
    smoothed = pd.Series(volumes).rolling(max(3, n_bins // 20), center=True).mean().fillna(0).values
    peaks, properties = find_peaks(smoothed, height=smoothed.max() * 0.3,
                                    distance=n_bins // 5, prominence=smoothed.max() * 0.15)
    n_peaks = len(peaks)

    # Volume concentration (how spread out is volume?)
    vol_normalized = volumes / total_vol if total_vol > 0 else volumes
    entropy = -np.sum(vol_normalized[vol_normalized > 0] * np.log(vol_normalized[vol_normalized > 0]))
    max_entropy = np.log(n_bins)
    concentration = 1 - (entropy / max_entropy) if max_entropy > 0 else 0

    # Kurtosis of volume distribution
    vol_mean = volumes.mean()
    vol_std = volumes.std()
    if vol_std > 0:
        vol_kurtosis = float(((volumes - vol_mean) ** 4).mean() / vol_std ** 4)
    else:
        vol_kurtosis = 3.0

    # Classify shape
    if n_peaks >= 2:
        shape = "B-shape"
        shape_emoji = "🔀"
        shape_desc = "Double Distribution (two-timeframe market)"
        implication = "Dua area value terpisah — breakout expected saat market memilih satu sisi"
        peak_prices = [float(bin_centers[p]) for p in peaks]
    elif poc_position > 0.65 and vol_skew > 0.15:
        shape = "P-shape"
        shape_emoji = "🅿️"
        shape_desc = "Buying/Long Liquidation Profile"
        implication = "Volume terkonsentrasi di atas — bullish accumulation atau long liquidation"
        peak_prices = []
    elif poc_position < 0.35 and vol_skew < -0.15:
        shape = "b-shape"
        shape_emoji = "🅱️"
        shape_desc = "Selling/Short Covering Profile"
        implication = "Volume terkonsentrasi di bawah — bearish distribution atau short covering"
        peak_prices = []
    elif concentration > 0.4 and 0.35 <= poc_position <= 0.65:
        shape = "D-shape"
        shape_emoji = "🔵"
        shape_desc = "Balanced/Normal Profile"
        implication = "Volume terdistribusi seimbang — market in balance, fair value found"
        peak_prices = []
    else:
        shape = "Thin"
        shape_emoji = "📏"
        shape_desc = "Elongated/Trending Profile"
        implication = "Volume tersebar merata — trending market, belum ada consensus value"
        peak_prices = []

    # Distribution normality
    if vol_kurtosis < 2.5:
        dist_type = "Platykurtic (thin-tailed)"
        dist_implication = "trending, directional"
    elif vol_kurtosis > 4.0:
        dist_type = "Leptokurtic (fat-tailed)"
        dist_implication = "excess activity at extremes"
    else:
        dist_type = "Mesokurtic (normal)"
        dist_implication = "balanced distribution"

    dec = price_decimals(df["Close"].iloc[-1])
    current_price = df["Close"].iloc[-1]

    result = {
        "shape": shape,
        "shape_desc": shape_desc,
        "poc_position": safe_float(poc_position),
        "vol_skew": safe_float(vol_skew),
        "n_peaks": n_peaks,
        "concentration": safe_float(concentration),
        "kurtosis": safe_float(vol_kurtosis),
        "peak_prices": [safe_float(p) for p in peak_prices],
        "dist_type": dist_type,
    }

    text = f"""{shape_emoji} <b>Profile Shape: {symbol} — {timeframe}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Shape: <b>{shape}</b> — {shape_desc}
💡 {implication}

📈 Distribution:
  POC position: {poc_position*100:.0f}% (0=bottom, 100=top)
  Volume skew: {vol_skew:+.2f} (+ = upper heavy)
  Concentration: {concentration:.2f} (1 = tight cluster)
  Kurtosis: {vol_kurtosis:.1f} — {dist_type}
  → {dist_implication}
  Peaks detected: {n_peaks}
"""
    if peak_prices:
        text += "\n🎯 <b>Peak Levels:</b>\n"
        for pp in peak_prices:
            text += f"  {format_price(pp, dec)}\n"

    if chart_path:
        _draw_shape_chart(df, vp, shape, peaks, bin_centers, smoothed,
                          symbol, timeframe, chart_path)

    return output("shape", symbol, True, result, text, chart_path=chart_path or "")


def _draw_shape_chart(df, vp, shape, peaks, bin_centers, smoothed,
                       symbol, timeframe, chart_path):
    """Draw shape analysis chart."""
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 7),
                                     gridspec_kw={"width_ratios": [1, 1]})
    fig.suptitle(f"{symbol} — {shape} — {timeframe}",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    # Left: volume distribution with shape overlay
    volumes = vp["volumes"]
    colors_v = []
    for i in range(len(volumes)):
        if volumes[i] > vp["hvn_threshold"]:
            colors_v.append(HVN_COLOR)
        elif volumes[i] < vp["lvn_threshold"] and volumes[i] > 0:
            colors_v.append(LVN_COLOR)
        else:
            colors_v.append("#4a5568")

    ax1.barh(bin_centers, volumes, height=vp["bin_width"] * 0.9,
             color=colors_v, alpha=0.7, edgecolor="none")
    ax1.plot(smoothed, bin_centers, color=ACCENT2, linewidth=1.5, alpha=0.8, label="Smoothed")

    # Mark peaks
    for p in peaks:
        ax1.axhline(bin_centers[p], color=ACCENT3, linewidth=1, linestyle="--", alpha=0.7)
        ax1.plot(smoothed[p], bin_centers[p], "o", color=ACCENT3, markersize=8)

    ax1.axhline(vp["poc_price"], color=POC_COLOR, linewidth=1.5)
    ax1.set_ylabel("Price")
    ax1.set_xlabel("Volume")
    ax1.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
    ax1.grid(True, alpha=0.2)

    # Right: price chart with VP zones
    ax2.plot(range(len(df)), df["Close"].values, color=ACCENT1, linewidth=1)
    ax2.axhline(vp["poc_price"], color=POC_COLOR, linewidth=1.5, label="POC")
    ax2.axhline(vp["vah"], color=VAH_COLOR, linewidth=1, linestyle="--")
    ax2.axhline(vp["val"], color=VAL_COLOR, linewidth=1, linestyle="--")
    ax2.axhspan(vp["val"], vp["vah"], alpha=0.05, color=VAH_COLOR)
    ax2.set_ylabel("Price")
    ax2.grid(True, alpha=0.2)
    ax2.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: COMPOSITE — Multi-Day Merged VP
# ===========================================================================

def compute_composite(df, symbol, timeframe, params, all_tf_bars=None, chart_path=None):
    """Composite VP — merge multiple periods."""
    n = len(df)
    if n < 30:
        return output("composite", symbol, False, {}, "",
                       error="Minimal 30 bar untuk Composite VP")

    dec = price_decimals(df["Close"].iloc[-1])
    current_price = df["Close"].iloc[-1]

    # Compute VP for different lookback windows
    windows = {}

    # Recent (last 1/4 of data)
    recent_n = max(20, n // 4)
    df_recent = df.iloc[-recent_n:]
    vp_recent = compute_volume_profile(df_recent, n_bins=100, va_pct=0.70)
    if vp_recent:
        windows["Recent"] = {"df": df_recent, "vp": vp_recent, "n": recent_n}

    # Medium (last 1/2)
    medium_n = max(30, n // 2)
    df_medium = df.iloc[-medium_n:]
    vp_medium = compute_volume_profile(df_medium, n_bins=100, va_pct=0.70)
    if vp_medium:
        windows["Medium"] = {"df": df_medium, "vp": vp_medium, "n": medium_n}

    # Full (all data)
    vp_full = compute_volume_profile(df, n_bins=100, va_pct=0.70)
    if vp_full:
        windows["Full"] = {"df": df, "vp": vp_full, "n": n}

    if not windows:
        return output("composite", symbol, False, {}, "",
                       error="Tidak dapat menghitung Composite VP")

    # Find overlapping HVN zones across windows
    composite_hvn = _find_composite_confluence(windows)

    result = {
        "windows": {},
        "composite_hvn": composite_hvn,
    }

    text = f"""🔀 <b>Composite Volume Profile: {symbol} — {timeframe}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
"""
    for name, win in windows.items():
        vp = win["vp"]
        text += f"📊 <b>{name}</b> ({win['n']} bars):\n"
        text += f"  POC: {format_price(vp['poc_price'], dec)}"
        text += f" | VA: {format_price(vp['val'], dec)} — {format_price(vp['vah'], dec)}\n\n"
        result["windows"][name] = {
            "poc": safe_float(vp["poc_price"]),
            "vah": safe_float(vp["vah"]),
            "val": safe_float(vp["val"]),
            "n_bars": win["n"],
        }

    if composite_hvn:
        text += "🎯 <b>Composite HVN (multi-window overlap):</b>\n"
        for zone in composite_hvn[:5]:
            text += f"  ★{'★' * (zone['score'] - 1)} {format_price(zone['low'], dec)}"
            text += f" — {format_price(zone['high'], dec)}"
            text += f" (overlap: {zone['overlap']})\n"

    if chart_path:
        _draw_composite_chart(df, windows, composite_hvn, symbol, timeframe,
                               chart_path, current_price)

    return output("composite", symbol, True, result, text, chart_path=chart_path or "")


def _find_composite_confluence(windows):
    """Find price zones where HVN from different windows overlap."""
    all_hvns = []
    for name, win in windows.items():
        for zone in win["vp"]["hvn_zones"]:
            all_hvns.append({"low": zone["low"], "high": zone["high"],
                             "source": name, "volume": zone["volume"]})

    if not all_hvns:
        return []

    # Cluster overlapping zones
    all_hvns.sort(key=lambda z: z["low"])
    clusters = []

    for hvn in all_hvns:
        merged = False
        for cluster in clusters:
            # Check overlap
            if hvn["low"] <= cluster["high"] and hvn["high"] >= cluster["low"]:
                cluster["low"] = min(cluster["low"], hvn["low"])
                cluster["high"] = max(cluster["high"], hvn["high"])
                cluster["sources"].add(hvn["source"])
                cluster["volume"] += hvn["volume"]
                merged = True
                break
        if not merged:
            clusters.append({
                "low": hvn["low"],
                "high": hvn["high"],
                "sources": {hvn["source"]},
                "volume": hvn["volume"],
            })

    # Score by number of overlapping windows
    scored = []
    for c in clusters:
        scored.append({
            "low": c["low"],
            "high": c["high"],
            "score": len(c["sources"]),
            "overlap": ", ".join(sorted(c["sources"])),
            "volume": c["volume"],
        })

    scored.sort(key=lambda z: z["score"], reverse=True)
    return scored


def _draw_composite_chart(df, windows, composite_hvn, symbol, timeframe,
                           chart_path, current_price):
    """Draw composite VP chart."""
    fig, (ax, ax_vol) = plt.subplots(
        1, 2, figsize=(16, 8), gridspec_kw={"width_ratios": [3, 1]}, sharey=True)

    fig.suptitle(f"{symbol} — Composite VP — {timeframe}",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    # Price chart
    ax.plot(range(len(df)), df["Close"].values, color=ACCENT1, linewidth=1, alpha=0.8)
    ax.axhline(current_price, color=ACCENT2, linewidth=0.8, linestyle=":", alpha=0.8)

    # Composite HVN highlights
    for zone in (composite_hvn or [])[:5]:
        alpha = 0.1 + 0.05 * zone["score"]
        ax.axhspan(zone["low"], zone["high"], alpha=alpha, color=HVN_COLOR)

    # VP histograms from each window
    win_colors = {"Recent": ACCENT2, "Medium": ACCENT1, "Full": "#888888"}
    for name, win in windows.items():
        vp = win["vp"]
        color = win_colors.get(name, "#666")
        ax_vol.barh(vp["bin_centers"], vp["volumes"], height=vp["bin_width"] * 0.9,
                     color=color, alpha=0.3, label=name, edgecolor="none")
        ax.axhline(vp["poc_price"], color=color, linewidth=1, linestyle="-", alpha=0.6)

    ax.set_xlim(-1, len(df))
    ax.grid(True, alpha=0.2)
    ax_vol.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
    ax_vol.set_xlabel("Volume", fontsize=8)
    ax_vol.yaxis.set_visible(False)
    ax_vol.grid(True, alpha=0.2)

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: VWAP — VWAP Bands Analysis
# ===========================================================================

def compute_vwap(df, symbol, timeframe, params, chart_path=None):
    """VWAP with standard deviation bands."""
    n = len(df)
    if n < 20:
        return output("vwap", symbol, False, {}, "",
                       error="Minimal 20 bar untuk VWAP")

    dec = price_decimals(df["Close"].iloc[-1])
    current_price = df["Close"].iloc[-1]

    # Compute VWAP
    typical_price = (df["High"] + df["Low"] + df["Close"]) / 3
    volume = df["Volume"].replace(0, 1)  # avoid div by zero for FX tick volume
    cum_tp_vol = (typical_price * volume).cumsum()
    cum_vol = volume.cumsum()
    vwap = cum_tp_vol / cum_vol

    # VWAP bands (standard deviation)
    squared_diff = ((typical_price - vwap) ** 2 * volume).cumsum()
    vwap_std = np.sqrt(squared_diff / cum_vol)

    band_1u = vwap + vwap_std
    band_1l = vwap - vwap_std
    band_2u = vwap + 2 * vwap_std
    band_2l = vwap - 2 * vwap_std
    band_3u = vwap + 3 * vwap_std
    band_3l = vwap - 3 * vwap_std

    current_vwap = float(vwap.iloc[-1])
    current_std = float(vwap_std.iloc[-1])

    # Anchored VWAP from swing high and swing low
    swing_high_idx = df["High"].idxmax()
    swing_low_idx = df["Low"].idxmin()

    avwap_high = _anchored_vwap(df, swing_high_idx)
    avwap_low = _anchored_vwap(df, swing_low_idx)

    # Position relative to VWAP
    if current_std > 0:
        z_score = (current_price - current_vwap) / current_std
    else:
        z_score = 0

    if z_score > 2:
        position = "extended above +2σ"
        advice = "Overbought — high probability pullback ke VWAP"
    elif z_score > 1:
        position = "above +1σ"
        advice = "Bullish momentum — watch for VWAP retest"
    elif z_score < -2:
        position = "extended below -2σ"
        advice = "Oversold — high probability bounce ke VWAP"
    elif z_score < -1:
        position = "below -1σ"
        advice = "Bearish momentum — watch for VWAP retest"
    else:
        position = "near VWAP (fair value zone)"
        advice = "Di zona fair value — watch for directional break"

    result = {
        "vwap": safe_float(current_vwap),
        "std": safe_float(current_std),
        "z_score": safe_float(z_score),
        "band_1u": safe_float(float(band_1u.iloc[-1])),
        "band_1l": safe_float(float(band_1l.iloc[-1])),
        "band_2u": safe_float(float(band_2u.iloc[-1])),
        "band_2l": safe_float(float(band_2l.iloc[-1])),
        "avwap_from_high": safe_float(avwap_high),
        "avwap_from_low": safe_float(avwap_low),
        "position": position,
    }

    text = f"""📏 <b>VWAP Analysis: {symbol} — {timeframe}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 VWAP: {format_price(current_vwap, dec)}
📊 Z-Score: {z_score:+.2f} — {position}

📐 <b>Bands:</b>
  +3σ: {format_price(float(band_3u.iloc[-1]), dec)}
  +2σ: {format_price(float(band_2u.iloc[-1]), dec)}
  +1σ: {format_price(float(band_1u.iloc[-1]), dec)}
  VWAP: {format_price(current_vwap, dec)}
  -1σ: {format_price(float(band_1l.iloc[-1]), dec)}
  -2σ: {format_price(float(band_2l.iloc[-1]), dec)}
  -3σ: {format_price(float(band_3l.iloc[-1]), dec)}

🎯 <b>Anchored VWAP:</b>
  From Swing High: {format_price(avwap_high, dec) if avwap_high else 'N/A'}
  From Swing Low: {format_price(avwap_low, dec) if avwap_low else 'N/A'}

💡 {advice}"""

    if chart_path:
        _draw_vwap_chart(df, vwap, band_1u, band_1l, band_2u, band_2l, band_3u, band_3l,
                          symbol, timeframe, chart_path, current_price)

    return output("vwap", symbol, True, result, text, chart_path=chart_path or "")


def _anchored_vwap(df, anchor_idx):
    """Compute anchored VWAP from a specific index."""
    try:
        anchor_pos = df.index.get_loc(anchor_idx)
        sliced = df.iloc[anchor_pos:]
        if len(sliced) < 2:
            return None
        tp = (sliced["High"] + sliced["Low"] + sliced["Close"]) / 3
        vol = sliced["Volume"].replace(0, 1)
        return float((tp * vol).cumsum().iloc[-1] / vol.cumsum().iloc[-1])
    except Exception:
        return None


def _draw_vwap_chart(df, vwap, b1u, b1l, b2u, b2l, b3u, b3l,
                      symbol, timeframe, chart_path, current_price):
    """Draw VWAP bands chart."""
    fig, ax = plt.subplots(figsize=(14, 7))
    fig.suptitle(f"{symbol} — VWAP Bands — {timeframe} ({len(df)} bars)",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    x = range(len(df))
    ax.plot(x, df["Close"].values, color=ACCENT1, linewidth=1, alpha=0.8, label="Price")
    ax.plot(x, vwap.values, color=VWAP_COLOR, linewidth=1.5, label="VWAP")

    ax.fill_between(x, b1l.values, b1u.values, alpha=0.08, color=VWAP_COLOR, label="±1σ")
    ax.fill_between(x, b2l.values, b2u.values, alpha=0.04, color=VWAP_COLOR, label="±2σ")
    ax.fill_between(x, b3l.values, b3u.values, alpha=0.02, color=VWAP_COLOR, label="±3σ")

    ax.plot(x, b1u.values, color=VWAP_COLOR, linewidth=0.5, alpha=0.5, linestyle="--")
    ax.plot(x, b1l.values, color=VWAP_COLOR, linewidth=0.5, alpha=0.5, linestyle="--")
    ax.plot(x, b2u.values, color=ACCENT4, linewidth=0.5, alpha=0.5, linestyle="--")
    ax.plot(x, b2l.values, color=ACCENT4, linewidth=0.5, alpha=0.5, linestyle="--")

    ax.axhline(current_price, color=ACCENT2, linewidth=0.8, linestyle=":", alpha=0.6)
    ax.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
    ax.grid(True, alpha=0.2)
    ax.set_ylabel("Price")

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: TPO — Time Price Opportunity
# ===========================================================================

def compute_tpo(df, symbol, timeframe, params, chart_path=None):
    """TPO analysis — time spent at each price level."""
    n = len(df)
    if n < 20:
        return output("tpo", symbol, False, {}, "",
                       error="Minimal 20 bar untuk TPO")

    dec = price_decimals(df["Close"].iloc[-1])
    current_price = df["Close"].iloc[-1]

    # Compute volume-based VP and time-based TP (TPO)
    vp = compute_volume_profile(df, va_pct=0.70)
    if vp is None:
        return output("tpo", symbol, False, {}, "",
                       error="Tidak dapat menghitung VP")

    # TPO: count how many bars touch each price bin
    bin_edges = vp["bin_edges"]
    n_bins = vp["n_bins"]
    tpo_counts = np.zeros(n_bins)

    for _, bar in df.iterrows():
        low_bin = np.searchsorted(bin_edges, bar["Low"]) - 1
        high_bin = np.searchsorted(bin_edges, bar["High"]) - 1
        low_bin = np.clip(low_bin, 0, n_bins - 1)
        high_bin = np.clip(high_bin, 0, n_bins - 1)
        for b in range(low_bin, high_bin + 1):
            tpo_counts[b] += 1

    # TPO POC vs Volume POC
    tpo_poc_idx = np.argmax(tpo_counts)
    tpo_poc_price = float(vp["bin_centers"][tpo_poc_idx])
    vol_poc_price = vp["poc_price"]

    poc_divergence = abs(tpo_poc_price - vol_poc_price)
    poc_divergence_pips = poc_divergence / pip_value(symbol)

    if poc_divergence_pips > 10:
        divergence_text = "⚠️ Significant divergence"
        divergence_explain = "Volume POC ≠ Time POC — institutional vs retail focus berbeda"
    elif poc_divergence_pips > 5:
        divergence_text = "⚡ Moderate divergence"
        divergence_explain = "Sedikit perbedaan — perhatikan ke mana volume baru mengalir"
    else:
        divergence_text = "✅ Aligned"
        divergence_explain = "Volume dan Time POC selaras — strong consensus fair value"

    # TPO Value Area
    tpo_total = tpo_counts.sum()
    tpo_va_target = tpo_total * 0.70
    tpo_va_vol = tpo_counts[tpo_poc_idx]
    tpo_va_lo = tpo_poc_idx
    tpo_va_hi = tpo_poc_idx
    while tpo_va_vol < tpo_va_target:
        up = tpo_counts[tpo_va_hi + 1] if tpo_va_hi + 1 < n_bins else 0
        dn = tpo_counts[tpo_va_lo - 1] if tpo_va_lo - 1 >= 0 else 0
        if up == 0 and dn == 0:
            break
        if up >= dn:
            tpo_va_hi += 1
            tpo_va_vol += up
        else:
            tpo_va_lo -= 1
            tpo_va_vol += dn

    tpo_vah = float(bin_edges[min(tpo_va_hi + 1, n_bins)])
    tpo_val = float(bin_edges[tpo_va_lo])

    result = {
        "tpo_poc": safe_float(tpo_poc_price),
        "tpo_vah": safe_float(tpo_vah),
        "tpo_val": safe_float(tpo_val),
        "vol_poc": safe_float(vol_poc_price),
        "poc_divergence_pips": safe_float(poc_divergence_pips),
        "divergence": divergence_text,
    }

    text = f"""⏱ <b>TPO Analysis: {symbol} — {timeframe}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
<b>Time-at-Price:</b>
  TPO POC: {format_price(tpo_poc_price, dec)} (most time spent)
  TPO VAH: {format_price(tpo_vah, dec)}
  TPO VAL: {format_price(tpo_val, dec)}

<b>Volume-at-Price:</b>
  Vol POC: {format_price(vol_poc_price, dec)} (most volume)
  Vol VAH: {format_price(vp["vah"], dec)}
  Vol VAL: {format_price(vp["val"], dec)}

📊 <b>POC Divergence:</b> {poc_divergence_pips:.1f} pips
  {divergence_text}
  {divergence_explain}

💡 TPO = di mana market MENGHABISKAN WAKTU
   Vol = di mana market BERTRANSAKSI BANYAK
   Divergence = smart money ≠ consensus"""

    if chart_path:
        _draw_tpo_chart(df, vp, tpo_counts, tpo_poc_price, tpo_vah, tpo_val,
                         symbol, timeframe, chart_path, current_price)

    return output("tpo", symbol, True, result, text, chart_path=chart_path or "")


def _draw_tpo_chart(df, vp, tpo_counts, tpo_poc, tpo_vah, tpo_val,
                     symbol, timeframe, chart_path, current_price):
    """Draw dual histogram: volume (left) + TPO (right)."""
    fig, (ax_vol, ax_price, ax_tpo) = plt.subplots(
        1, 3, figsize=(18, 8), gridspec_kw={"width_ratios": [1, 3, 1]}, sharey=True)

    fig.suptitle(f"{symbol} — TPO vs Volume — {timeframe}",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    bin_centers = vp["bin_centers"]

    # Volume histogram (left, reversed)
    ax_vol.barh(bin_centers, vp["volumes"], height=vp["bin_width"] * 0.9,
                color=ACCENT1, alpha=0.6, edgecolor="none")
    ax_vol.axhline(vp["poc_price"], color=POC_COLOR, linewidth=1.5)
    ax_vol.invert_xaxis()
    ax_vol.set_xlabel("Volume", fontsize=8)
    ax_vol.set_title("Volume Profile", fontsize=9, color=TEXT_COLOR)
    ax_vol.grid(True, alpha=0.2)

    # Price chart (center)
    ax_price.plot(range(len(df)), df["Close"].values, color=ACCENT1, linewidth=1)
    ax_price.axhline(vp["poc_price"], color=POC_COLOR, linewidth=1, linestyle="-",
                      alpha=0.7, label="Vol POC")
    ax_price.axhline(tpo_poc, color=ACCENT3, linewidth=1, linestyle="-",
                      alpha=0.7, label="TPO POC")
    ax_price.axhline(current_price, color=ACCENT2, linewidth=0.8, linestyle=":")
    ax_price.set_xlim(-1, len(df))
    ax_price.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
    ax_price.grid(True, alpha=0.2)
    ax_price.yaxis.set_visible(False)

    # TPO histogram (right)
    ax_tpo.barh(bin_centers, tpo_counts, height=vp["bin_width"] * 0.9,
                color=ACCENT3, alpha=0.6, edgecolor="none")
    ax_tpo.axhline(tpo_poc, color=ACCENT3, linewidth=1.5)
    ax_tpo.set_xlabel("Time (bars)", fontsize=8)
    ax_tpo.set_title("Time Profile", fontsize=9, color=TEXT_COLOR)
    ax_tpo.grid(True, alpha=0.2)
    ax_tpo.yaxis.set_visible(False)

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: DELTA — Simulated Volume Delta
# ===========================================================================

def compute_delta(df, symbol, timeframe, params, chart_path=None):
    """Simulated delta volume from OHLCV heuristics."""
    n = len(df)
    if n < 20:
        return output("delta", symbol, False, {}, "",
                       error="Minimal 20 bar untuk Delta Volume")

    dec = price_decimals(df["Close"].iloc[-1])
    current_price = df["Close"].iloc[-1]

    # Compute delta per bar: delta ≈ volume × (close - open) / (high - low)
    bar_range = df["High"] - df["Low"]
    bar_range = bar_range.replace(0, np.nan)
    volume = df["Volume"].replace(0, 1)

    delta = volume * (df["Close"] - df["Open"]) / bar_range
    delta = delta.fillna(0)

    # Cumulative delta
    cum_delta = delta.cumsum()

    # Delta per price level (for histogram)
    vp = compute_volume_profile(df, va_pct=0.70)
    if vp is None:
        return output("delta", symbol, False, {}, "",
                       error="Tidak dapat menghitung VP")

    n_bins = vp["n_bins"]
    bin_edges = vp["bin_edges"]
    buy_vol = np.zeros(n_bins)
    sell_vol = np.zeros(n_bins)

    for _, bar in df.iterrows():
        bar_low = bar["Low"]
        bar_high = bar["High"]
        bar_vol = bar["Volume"] if bar["Volume"] > 0 else 1.0
        bar_r = bar_high - bar_low
        if bar_r <= 0:
            continue

        # Estimate buy/sell split
        buy_ratio = (bar["Close"] - bar_low) / bar_r
        sell_ratio = 1 - buy_ratio

        low_bin = np.clip(np.searchsorted(bin_edges, bar_low) - 1, 0, n_bins - 1)
        high_bin = np.clip(np.searchsorted(bin_edges, bar_high) - 1, 0, n_bins - 1)

        for b in range(low_bin, high_bin + 1):
            bin_lo = max(bin_edges[b], bar_low)
            bin_hi = min(bin_edges[b + 1], bar_high)
            overlap = max(0, bin_hi - bin_lo)
            fraction = overlap / bar_r
            buy_vol[b] += bar_vol * buy_ratio * fraction
            sell_vol[b] += bar_vol * sell_ratio * fraction

    delta_per_level = buy_vol - sell_vol

    # Delta divergence detection
    # Compare price trend vs cumulative delta trend (last 20 bars)
    lookback = min(20, n)
    price_change = df["Close"].iloc[-1] - df["Close"].iloc[-lookback]
    delta_change = cum_delta.iloc[-1] - cum_delta.iloc[-lookback]

    if price_change > 0 and delta_change < 0:
        divergence = "🔴 BEARISH DIVERGENCE"
        div_explain = "Price naik tapi delta turun — selling pressure tersembunyi"
    elif price_change < 0 and delta_change > 0:
        divergence = "🟢 BULLISH DIVERGENCE"
        div_explain = "Price turun tapi delta naik — buying pressure tersembunyi"
    elif price_change > 0 and delta_change > 0:
        divergence = "✅ BULLISH CONFIRMATION"
        div_explain = "Price naik + delta naik — genuine bullish momentum"
    elif price_change < 0 and delta_change < 0:
        divergence = "✅ BEARISH CONFIRMATION"
        div_explain = "Price turun + delta turun — genuine bearish momentum"
    else:
        divergence = "↔️ NEUTRAL"
        div_explain = "No significant divergence"

    # Imbalance detection — find levels with extreme buy/sell ratio
    imbalances = []
    for i in range(n_bins):
        total = buy_vol[i] + sell_vol[i]
        if total < vp["volumes"].mean() * 0.5:
            continue  # skip low-activity levels
        if buy_vol[i] > 0 and sell_vol[i] > 0:
            ratio = max(buy_vol[i], sell_vol[i]) / min(buy_vol[i], sell_vol[i])
            if ratio >= 2.5:
                side = "BUY" if buy_vol[i] > sell_vol[i] else "SELL"
                imbalances.append({
                    "price": float(vp["bin_centers"][i]),
                    "ratio": float(ratio),
                    "side": side,
                })

    imbalances.sort(key=lambda x: x["ratio"], reverse=True)

    result = {
        "cum_delta": safe_float(float(cum_delta.iloc[-1])),
        "delta_change_20": safe_float(float(delta_change)),
        "price_change_20": safe_float(float(price_change)),
        "divergence": divergence,
        "imbalances": imbalances[:5],
    }

    text = f"""📈 <b>Simulated Delta: {symbol} — {timeframe}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Cumulative Delta: {cum_delta.iloc[-1]:+,.0f}
  Net {'buyers' if cum_delta.iloc[-1] > 0 else 'sellers'} dominant

📉 <b>Last {lookback} bars:</b>
  Price change: {price_change:+.{dec}f}
  Delta change: {delta_change:+,.0f}
  {divergence}
  {div_explain}
"""
    if imbalances:
        text += "\n⚡ <b>Volume Imbalances (institutional orders):</b>\n"
        for imb in imbalances[:5]:
            emoji = "🟢" if imb["side"] == "BUY" else "🔴"
            text += f"  {emoji} {format_price(imb['price'], dec)}"
            text += f" — {imb['side']} {imb['ratio']:.1f}:1\n"

    text += "\n⚠️ Delta dari OHLCV = estimasi (~70% akurasi), bukan real order flow"

    if chart_path:
        _draw_delta_chart(df, cum_delta, delta, buy_vol, sell_vol, vp,
                          symbol, timeframe, chart_path, current_price)

    return output("delta", symbol, True, result, text, chart_path=chart_path or "")


def _draw_delta_chart(df, cum_delta, bar_delta, buy_vol, sell_vol, vp,
                       symbol, timeframe, chart_path, current_price):
    """Draw delta volume chart: price + cumulative delta + delta histogram."""
    fig, (ax_price, ax_delta, ax_hist) = plt.subplots(
        1, 3, figsize=(18, 8), gridspec_kw={"width_ratios": [3, 2, 1]})

    fig.suptitle(f"{symbol} — Simulated Delta Volume — {timeframe}",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    x = range(len(df))

    # Price
    ax_price.plot(x, df["Close"].values, color=ACCENT1, linewidth=1, label="Price")
    ax_price.axhline(current_price, color=ACCENT2, linewidth=0.8, linestyle=":")
    ax_price.set_ylabel("Price")
    ax_price.grid(True, alpha=0.2)
    ax_price.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)

    # Cumulative delta
    colors_d = [UP_COLOR if v >= 0 else DOWN_COLOR for v in bar_delta.values]
    ax_delta.bar(x, bar_delta.values, color=colors_d, alpha=0.5, width=0.8)
    ax_delta_twin = ax_delta.twinx()
    ax_delta_twin.plot(x, cum_delta.values, color=ACCENT2, linewidth=1.5, label="Cum Delta")
    ax_delta.set_ylabel("Bar Delta")
    ax_delta_twin.set_ylabel("Cumulative Delta", color=ACCENT2)
    ax_delta.grid(True, alpha=0.2)
    ax_delta_twin.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR, loc="upper left")

    # Delta per price level (buy vs sell)
    bin_centers = vp["bin_centers"]
    bw = vp["bin_width"] * 0.9
    ax_hist.barh(bin_centers, buy_vol, height=bw, color=UP_COLOR, alpha=0.6, label="Buy")
    ax_hist.barh(bin_centers, -sell_vol, height=bw, color=DOWN_COLOR, alpha=0.6, label="Sell")
    ax_hist.axvline(0, color=GRID_COLOR, linewidth=0.5)
    ax_hist.set_xlabel("Volume")
    ax_hist.set_title("Buy/Sell", fontsize=9, color=TEXT_COLOR)
    ax_hist.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
    ax_hist.grid(True, alpha=0.2)

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: AUCTION — Auction Market Theory
# ===========================================================================

def compute_auction(df, symbol, timeframe, params, chart_path=None):
    """Auction Market Theory analysis."""
    n = len(df)
    if n < 30:
        return output("auction", symbol, False, {}, "",
                       error="Minimal 30 bar untuk Auction analysis")

    dec = price_decimals(df["Close"].iloc[-1])
    current_price = df["Close"].iloc[-1]

    vp = compute_volume_profile(df, va_pct=0.70)
    if vp is None:
        return output("auction", symbol, False, {}, "",
                       error="Tidak dapat menghitung VP")

    poc = vp["poc_price"]
    vah = vp["vah"]
    val = vp["val"]

    # Auction state
    if current_price > vah:
        auction_state = "BREAKOUT_UP"
        state_emoji = "🚀"
        state_desc = "Price above Value Area — initiative buying"
        strategy = "Follow momentum / buy pullback ke VAH"
    elif current_price < val:
        auction_state = "BREAKOUT_DOWN"
        state_emoji = "💥"
        state_desc = "Price below Value Area — initiative selling"
        strategy = "Follow momentum / sell rally ke VAL"
    elif abs(current_price - poc) / poc < 0.002:
        auction_state = "BALANCE"
        state_emoji = "⚖️"
        state_desc = "Price at POC — perfect balance"
        strategy = "Fade extremes: sell VAH, buy VAL"
    elif current_price > poc:
        auction_state = "RESPONSIVE_SELL_ZONE"
        state_emoji = "📊"
        state_desc = "Above POC but inside VA — responsive selling territory"
        strategy = "Watch for rejection at VAH → short; or break above VAH → long"
    else:
        auction_state = "RESPONSIVE_BUY_ZONE"
        state_emoji = "📊"
        state_desc = "Below POC but inside VA — responsive buying territory"
        strategy = "Watch for bounce at VAL → long; or break below VAL → short"

    # Excess / Poor structure analysis
    excess_poor = _analyze_excess_poor(df, vp)

    # Single prints detection
    single_prints = _find_single_prints(df, vp)

    # Initiative vs Responsive activity
    recent_bars = df.iloc[-10:]
    bars_above_vah = (recent_bars["Close"] > vah).sum()
    bars_below_val = (recent_bars["Close"] < val).sum()
    bars_in_va = len(recent_bars) - bars_above_vah - bars_below_val

    if bars_above_vah > 5 or bars_below_val > 5:
        activity = "INITIATIVE"
        act_desc = "Market bergerak ke luar Value Area — new information driving price"
    elif bars_in_va > 7:
        activity = "RESPONSIVE"
        act_desc = "Market tetap dalam Value Area — responsive to value, range-bound"
    else:
        activity = "TRANSITIONING"
        act_desc = "Market antara initiative dan responsive — watch for confirmation"

    result = {
        "auction_state": auction_state,
        "activity": activity,
        "poc": safe_float(poc),
        "vah": safe_float(vah),
        "val": safe_float(val),
        "bars_above_vah": int(bars_above_vah),
        "bars_below_val": int(bars_below_val),
        "bars_in_va": int(bars_in_va),
        "excess_poor": excess_poor,
        "single_prints": single_prints,
    }

    text = f"""🏛 <b>Auction Market Theory: {symbol} — {timeframe}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
{state_emoji} <b>State: {auction_state}</b>
{state_desc}
🎯 Strategy: {strategy}

📊 <b>Activity: {activity}</b>
{act_desc}
  Last 10 bars: {bars_in_va} in VA, {bars_above_vah} above VAH, {bars_below_val} below VAL
"""

    if excess_poor:
        text += "\n🔴 <b>Structure:</b>\n"
        for ep in excess_poor[:4]:
            text += f"  {ep['emoji']} {ep['type']} at {format_price(ep['price'], dec)}"
            text += f" — {ep['implication']}\n"

    if single_prints:
        text += "\n⚡ <b>Single Prints (gap zones):</b>\n"
        for sp in single_prints[:4]:
            text += f"  {format_price(sp['low'], dec)} — {format_price(sp['high'], dec)}\n"

    if chart_path:
        _draw_auction_chart(df, vp, auction_state, excess_poor, single_prints,
                            symbol, timeframe, chart_path, current_price)

    return output("auction", symbol, True, result, text, chart_path=chart_path or "")


def _analyze_excess_poor(df, vp):
    """Analyze excess (strong rejection) vs poor (weak) highs/lows."""
    results = []
    dec = price_decimals(df["Close"].iloc[-1])

    # Analyze high
    high_idx = df["High"].idxmax()
    high_pos = df.index.get_loc(high_idx)
    high_price = df["High"].max()

    # Check for rejection tail (excess) at the high
    if high_pos < len(df) - 1:
        bars_after_high = df.iloc[high_pos:]
        rejection_tail = high_price - bars_after_high["Close"].iloc[-1]
        bar_range = df["High"].max() - df["Low"].min()

        # Volume at the high
        high_bin = np.searchsorted(vp["bin_edges"], high_price) - 1
        high_bin = np.clip(high_bin, 0, vp["n_bins"] - 1)
        vol_at_high = vp["volumes"][high_bin]
        avg_vol = vp["volumes"].mean()

        if vol_at_high > avg_vol * 1.5 and rejection_tail > bar_range * 0.1:
            results.append({
                "type": "Excess High",
                "price": float(high_price),
                "emoji": "✅",
                "implication": "genuine reversal — strong rejection",
            })
        else:
            results.append({
                "type": "Poor High",
                "price": float(high_price),
                "emoji": "⚠️",
                "implication": "likely revisit — weak rejection",
            })

    # Analyze low
    low_idx = df["Low"].idxmin()
    low_pos = df.index.get_loc(low_idx)
    low_price = df["Low"].min()

    if low_pos < len(df) - 1:
        bars_after_low = df.iloc[low_pos:]
        bounce = bars_after_low["Close"].iloc[-1] - low_price
        bar_range = df["High"].max() - df["Low"].min()

        low_bin = np.searchsorted(vp["bin_edges"], low_price) - 1
        low_bin = np.clip(low_bin, 0, vp["n_bins"] - 1)
        vol_at_low = vp["volumes"][low_bin]

        if vol_at_low > avg_vol * 1.5 and bounce > bar_range * 0.1:
            results.append({
                "type": "Excess Low",
                "price": float(low_price),
                "emoji": "✅",
                "implication": "genuine support — strong bounce",
            })
        else:
            results.append({
                "type": "Poor Low",
                "price": float(low_price),
                "emoji": "⚠️",
                "implication": "likely revisit — weak support",
            })

    return results


def _find_single_prints(df, vp):
    """Find price levels touched only once (potential gap/breakout zones)."""
    volumes = vp["volumes"]
    bin_edges = vp["bin_edges"]
    bin_centers = vp["bin_centers"]
    n_bins = vp["n_bins"]

    # Count unique bars touching each bin
    touch_count = np.zeros(n_bins)
    for _, bar in df.iterrows():
        low_bin = np.clip(np.searchsorted(bin_edges, bar["Low"]) - 1, 0, n_bins - 1)
        high_bin = np.clip(np.searchsorted(bin_edges, bar["High"]) - 1, 0, n_bins - 1)
        for b in range(low_bin, high_bin + 1):
            touch_count[b] += 1

    # Single prints: touched only 1-2 times
    singles = []
    in_zone = False
    zone_start = None

    for i in range(n_bins):
        if touch_count[i] <= 2 and touch_count[i] > 0:
            if not in_zone:
                zone_start = i
                in_zone = True
        else:
            if in_zone:
                singles.append({
                    "low": float(bin_edges[zone_start]),
                    "high": float(bin_edges[i]),
                })
                in_zone = False

    if in_zone:
        singles.append({
            "low": float(bin_edges[zone_start]),
            "high": float(bin_edges[n_bins]),
        })

    return singles[:10]


def _draw_auction_chart(df, vp, state, excess_poor, single_prints,
                         symbol, timeframe, chart_path, current_price):
    """Draw auction market theory chart."""
    fig, (ax, ax_vol) = plt.subplots(
        1, 2, figsize=(16, 8), gridspec_kw={"width_ratios": [3, 1]}, sharey=True)

    fig.suptitle(f"{symbol} — Auction: {state} — {timeframe}",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    # Candlesticks
    for i in range(len(df)):
        c = df.iloc[i]
        color = UP_COLOR if c["Close"] >= c["Open"] else DOWN_COLOR
        ax.plot([i, i], [c["Low"], c["High"]], color=color, linewidth=0.5)
        body_lo = min(c["Open"], c["Close"])
        body_hi = max(c["Open"], c["Close"])
        ax.bar(i, max(body_hi - body_lo, (c["High"] - c["Low"]) * 0.001),
               bottom=body_lo, width=0.6, color=color, alpha=0.9, edgecolor="none")

    # VP levels
    ax.axhline(vp["poc_price"], color=POC_COLOR, linewidth=1.5, label="POC")
    ax.axhline(vp["vah"], color=VAH_COLOR, linewidth=1, linestyle="--", label="VAH")
    ax.axhline(vp["val"], color=VAL_COLOR, linewidth=1, linestyle="--", label="VAL")
    ax.axhspan(vp["val"], vp["vah"], alpha=0.05, color=VAH_COLOR)

    # Single prints
    for sp in (single_prints or [])[:5]:
        ax.axhspan(sp["low"], sp["high"], alpha=0.1, color=ACCENT4)

    # Excess/Poor markers
    for ep in (excess_poor or []):
        marker_color = HVN_COLOR if "Excess" in ep["type"] else LVN_COLOR
        ax.axhline(ep["price"], color=marker_color, linewidth=1, linestyle=":", alpha=0.7)

    ax.axhline(current_price, color=ACCENT2, linewidth=0.8, linestyle=":")
    ax.set_xlim(-1, len(df))
    ax.grid(True, alpha=0.2)
    ax.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)

    # Volume histogram
    colors_v = [HVN_COLOR if v > vp["hvn_threshold"] else
                (LVN_COLOR if v < vp["lvn_threshold"] and v > 0 else "#4a5568")
                for v in vp["volumes"]]
    ax_vol.barh(vp["bin_centers"], vp["volumes"], height=vp["bin_width"] * 0.9,
                color=colors_v, alpha=0.7, edgecolor="none")
    ax_vol.axhline(vp["poc_price"], color=POC_COLOR, linewidth=1.5)
    ax_vol.set_xlabel("Volume", fontsize=8)
    ax_vol.yaxis.set_visible(False)
    ax_vol.grid(True, alpha=0.2)

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: CONFLUENCE — Multi-Timeframe VP Level Confluence
# ===========================================================================

def compute_confluence(df, symbol, timeframe, params, all_tf_bars=None, chart_path=None):
    """Find confluent VP levels across multiple timeframes."""
    n = len(df)
    if n < 20:
        return output("confluence", symbol, False, {}, "",
                       error="Minimal 20 bar untuk Confluence")

    dec = price_decimals(df["Close"].iloc[-1])
    current_price = df["Close"].iloc[-1]

    # Compute VP for each available timeframe
    tf_vps = {}

    # Primary TF
    vp_primary = compute_volume_profile(df, va_pct=0.70)
    if vp_primary:
        tf_vps[timeframe] = vp_primary

    # Other timeframes from all_tf_bars
    if all_tf_bars:
        for tf, bars in all_tf_bars.items():
            if tf == timeframe:
                continue
            try:
                tf_df = bars_to_df(bars)
                if len(tf_df) >= 15:
                    vp_tf = compute_volume_profile(tf_df, n_bins=80, va_pct=0.70)
                    if vp_tf:
                        tf_vps[tf] = vp_tf
            except Exception:
                continue

    if len(tf_vps) < 1:
        return output("confluence", symbol, False, {}, "",
                       error="Tidak cukup timeframe data")

    # Collect all key levels from all TFs
    all_levels = []
    for tf, vp in tf_vps.items():
        all_levels.append({"price": vp["poc_price"], "type": "POC", "tf": tf, "weight": 3})
        all_levels.append({"price": vp["vah"], "type": "VAH", "tf": tf, "weight": 2})
        all_levels.append({"price": vp["val"], "type": "VAL", "tf": tf, "weight": 2})
        for hvn in vp["hvn_zones"][:3]:
            all_levels.append({"price": hvn["mid"], "type": "HVN", "tf": tf, "weight": 1})

    # Cluster nearby levels (within tolerance)
    tolerance = (df["High"].max() - df["Low"].min()) * 0.005  # 0.5% of range
    clusters = _cluster_price_levels(all_levels, tolerance)

    # Score clusters
    scored = []
    for cluster in clusters:
        score = sum(l["weight"] for l in cluster["levels"])
        tfs_involved = list(set(l["tf"] for l in cluster["levels"]))
        types_involved = list(set(l["type"] for l in cluster["levels"]))

        scored.append({
            "price_low": cluster["low"],
            "price_high": cluster["high"],
            "price_mid": (cluster["low"] + cluster["high"]) / 2,
            "score": score,
            "stars": min(score, 5),
            "timeframes": tfs_involved,
            "types": types_involved,
            "n_levels": len(cluster["levels"]),
            "distance_pips": abs((cluster["low"] + cluster["high"]) / 2 - current_price) / pip_value(symbol),
        })

    scored.sort(key=lambda x: x["score"], reverse=True)

    result = {
        "zones": [{"low": safe_float(z["price_low"]), "high": safe_float(z["price_high"]),
                    "score": z["score"], "timeframes": z["timeframes"],
                    "types": z["types"]} for z in scored[:10]],
        "n_timeframes": len(tf_vps),
        "timeframes_used": list(tf_vps.keys()),
    }

    text = f"""🎯 <b>Multi-TF Confluence: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 {len(tf_vps)} timeframes: {', '.join(tf_vps.keys())}
Current: {format_price(current_price, dec)}

<b>Confluence Zones (sorted by strength):</b>
"""
    for z in scored[:8]:
        stars = "★" * z["stars"]
        dist = z["distance_pips"]
        types_str = "+".join(z["types"])
        tfs_str = ",".join(z["timeframes"])
        text += f"  {stars} {format_price(z['price_low'], dec)}"
        text += f" — {format_price(z['price_high'], dec)}"
        text += f" ({types_str}) [{tfs_str}]"
        text += f" ({dist:.0f} pips away)\n"

    # Find nearest above and below
    above = [z for z in scored if z["price_mid"] > current_price]
    below = [z for z in scored if z["price_mid"] < current_price]

    if above:
        nearest_above = min(above, key=lambda z: z["distance_pips"])
        text += f"\n⬆️ Nearest resistance: {format_price(nearest_above['price_mid'], dec)}"
        text += f" ({'★' * nearest_above['stars']}, {nearest_above['distance_pips']:.0f} pips)\n"
    if below:
        nearest_below = min(below, key=lambda z: z["distance_pips"])
        text += f"⬇️ Nearest support: {format_price(nearest_below['price_mid'], dec)}"
        text += f" ({'★' * nearest_below['stars']}, {nearest_below['distance_pips']:.0f} pips)\n"

    if chart_path:
        _draw_confluence_chart(df, tf_vps, scored, symbol, timeframe,
                                chart_path, current_price)

    return output("confluence", symbol, True, result, text, chart_path=chart_path or "")


def _cluster_price_levels(levels, tolerance):
    """Cluster nearby price levels."""
    if not levels:
        return []

    sorted_levels = sorted(levels, key=lambda x: x["price"])
    clusters = []
    current_cluster = {"low": sorted_levels[0]["price"], "high": sorted_levels[0]["price"],
                       "levels": [sorted_levels[0]]}

    for lvl in sorted_levels[1:]:
        if lvl["price"] - current_cluster["high"] <= tolerance:
            current_cluster["high"] = lvl["price"]
            current_cluster["levels"].append(lvl)
        else:
            clusters.append(current_cluster)
            current_cluster = {"low": lvl["price"], "high": lvl["price"],
                              "levels": [lvl]}
    clusters.append(current_cluster)
    return clusters


def _draw_confluence_chart(df, tf_vps, scored_zones, symbol, timeframe,
                            chart_path, current_price):
    """Draw multi-TF confluence chart."""
    fig, ax = plt.subplots(figsize=(14, 8))
    fig.suptitle(f"{symbol} — Multi-TF VP Confluence — {timeframe}",
                 color=TEXT_COLOR, fontsize=13, fontweight="bold")

    # Price
    ax.plot(range(len(df)), df["Close"].values, color=ACCENT1, linewidth=1, alpha=0.8)

    # Confluence zones with color intensity based on score
    for z in scored_zones[:10]:
        alpha = 0.05 + 0.04 * z["stars"]
        color = HVN_COLOR if z["score"] >= 4 else (ACCENT1 if z["score"] >= 2 else GRID_COLOR)
        ax.axhspan(z["price_low"], z["price_high"], alpha=alpha, color=color)
        ax.text(len(df) * 0.02, z["price_mid"], "★" * z["stars"],
                fontsize=8, color=TEXT_COLOR, alpha=0.8, va="center")

    # POC lines per TF
    tf_colors = [POC_COLOR, ACCENT3, ACCENT4, VWAP_COLOR, "#FF69B4", HVN_COLOR, "#888"]
    for i, (tf, vp) in enumerate(tf_vps.items()):
        color = tf_colors[i % len(tf_colors)]
        ax.axhline(vp["poc_price"], color=color, linewidth=0.8, linestyle="-",
                    alpha=0.5, label=f"POC {tf}")

    ax.axhline(current_price, color=ACCENT2, linewidth=0.8, linestyle=":")
    ax.set_xlim(-1, len(df))
    ax.grid(True, alpha=0.2)
    ax.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR, ncol=2)
    ax.set_ylabel("Price")

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# MODE: FULL — Complete VP Report
# ===========================================================================

def compute_full(df, symbol, timeframe, params, all_tf_bars=None, chart_path=None):
    """Full VP Report — synthesize all analyses."""
    n = len(df)
    if n < 30:
        return output("full", symbol, False, {}, "",
                       error="Minimal 30 bar untuk Full VP Report")

    dec = price_decimals(df["Close"].iloc[-1])
    current_price = df["Close"].iloc[-1]

    sub_results = {}
    errors = []

    # Run core analyses
    try:
        sub_results["profile"] = compute_profile(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"profile: {e}")

    try:
        sub_results["shape"] = compute_shape(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"shape: {e}")

    try:
        sub_results["vwap"] = compute_vwap(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"vwap: {e}")

    try:
        sub_results["tpo"] = compute_tpo(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"tpo: {e}")

    try:
        sub_results["delta"] = compute_delta(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"delta: {e}")

    try:
        sub_results["auction"] = compute_auction(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"auction: {e}")

    try:
        sub_results["session"] = compute_session(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"session: {e}")

    if all_tf_bars:
        try:
            sub_results["confluence"] = compute_confluence(
                df, symbol, timeframe, params, all_tf_bars)
        except Exception as e:
            errors.append(f"confluence: {e}")

    # Synthesize decision
    signals = []

    # Auction state signal
    if "auction" in sub_results and sub_results["auction"]["success"]:
        ar = sub_results["auction"]["result"]
        state = ar.get("auction_state", "")
        if state == "BREAKOUT_UP":
            signals.append(("Auction", "BULLISH", 0.8))
        elif state == "BREAKOUT_DOWN":
            signals.append(("Auction", "BEARISH", 0.8))
        elif state == "RESPONSIVE_BUY_ZONE":
            signals.append(("Auction", "BULLISH", 0.4))
        elif state == "RESPONSIVE_SELL_ZONE":
            signals.append(("Auction", "BEARISH", 0.4))
        else:
            signals.append(("Auction", "NEUTRAL", 0.5))

    # Delta signal
    if "delta" in sub_results and sub_results["delta"]["success"]:
        dr = sub_results["delta"]["result"]
        div = dr.get("divergence", "")
        if "BULLISH" in div:
            signals.append(("Delta", "BULLISH", 0.6))
        elif "BEARISH" in div:
            signals.append(("Delta", "BEARISH", 0.6))
        else:
            cum = dr.get("cum_delta", 0) or 0
            signals.append(("Delta", "BULLISH" if cum > 0 else "BEARISH", 0.3))

    # Shape signal
    if "shape" in sub_results and sub_results["shape"]["success"]:
        sr = sub_results["shape"]["result"]
        shape = sr.get("shape", "")
        if shape == "P-shape":
            signals.append(("Shape", "BULLISH", 0.5))
        elif shape == "b-shape":
            signals.append(("Shape", "BEARISH", 0.5))
        elif shape == "B-shape":
            signals.append(("Shape", "BREAKOUT_PENDING", 0.3))

    # VWAP signal
    if "vwap" in sub_results and sub_results["vwap"]["success"]:
        vr = sub_results["vwap"]["result"]
        z = vr.get("z_score", 0) or 0
        if z > 1.5:
            signals.append(("VWAP", "BEARISH", min(abs(z) * 0.3, 0.8)))  # overbought
        elif z < -1.5:
            signals.append(("VWAP", "BULLISH", min(abs(z) * 0.3, 0.8)))  # oversold
        else:
            signals.append(("VWAP", "NEUTRAL", 0.5))

    # VA migration
    if "session" in sub_results and sub_results["session"]["success"]:
        sess_r = sub_results["session"]["result"]
        mig = sess_r.get("va_migration", {})
        if mig.get("direction") == "BULLISH":
            signals.append(("VA Migration", "BULLISH", 0.5))
        elif mig.get("direction") == "BEARISH":
            signals.append(("VA Migration", "BEARISH", 0.5))

    # Overall decision
    bull_score = sum(s[2] for s in signals if s[1] == "BULLISH")
    bear_score = sum(s[2] for s in signals if s[1] == "BEARISH")
    total_score = bull_score + bear_score

    if total_score > 0:
        net = (bull_score - bear_score) / total_score
    else:
        net = 0

    if net > 0.3:
        decision = "📈 BULLISH"
        confidence = min(abs(net) * 100, 95)
    elif net < -0.3:
        decision = "📉 BEARISH"
        confidence = min(abs(net) * 100, 95)
    else:
        decision = "↔️ NEUTRAL"
        confidence = min((1 - abs(net)) * 50, 60)

    result = {
        "decision": decision,
        "confidence": safe_float(confidence),
        "net_score": safe_float(net),
        "signals": [{"source": s[0], "direction": s[1], "strength": safe_float(s[2])} for s in signals],
        "sub_results": {k: v["success"] for k, v in sub_results.items()},
        "errors": errors,
    }

    # Chart: multi-panel summary
    if chart_path:
        _draw_full_chart(df, sub_results, scored_zones=None,
                          symbol=symbol, timeframe=timeframe,
                          chart_path=chart_path, current_price=current_price)

    text = f"""📋 <b>Full VP Report: {symbol} — {timeframe}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
🎯 <b>Decision: {decision}</b> (confidence {confidence:.0f}%)

📊 <b>Signals:</b>
"""
    for s in signals:
        emoji = "🟢" if s[1] == "BULLISH" else ("🔴" if s[1] == "BEARISH" else "⚪")
        text += f"  {emoji} {s[0]}: {s[1]} ({s[2]:.1f})\n"

    # Key levels summary
    if "profile" in sub_results and sub_results["profile"]["success"]:
        pr = sub_results["profile"]["result"]
        text += f"""
📍 <b>Key Levels:</b>
  POC: {format_price(pr['poc'], dec)}
  VAH: {format_price(pr['vah'], dec)}
  VAL: {format_price(pr['val'], dec)}
"""
    # Auction state
    if "auction" in sub_results and sub_results["auction"]["success"]:
        ar = sub_results["auction"]["result"]
        text += f"  Auction: {ar['auction_state']}\n"

    # Shape
    if "shape" in sub_results and sub_results["shape"]["success"]:
        sr = sub_results["shape"]["result"]
        text += f"  Shape: {sr['shape']}\n"

    # Delta
    if "delta" in sub_results and sub_results["delta"]["success"]:
        dr = sub_results["delta"]["result"]
        text += f"  Delta: {dr['divergence']}\n"

    if errors:
        text += f"\n⚠️ Errors: {', '.join(errors)}"

    return output("full", symbol, True, result, text, chart_path=chart_path or "")


def _draw_full_chart(df, sub_results, scored_zones, symbol, timeframe,
                      chart_path, current_price):
    """Draw full report summary chart."""
    fig, axes = plt.subplots(2, 2, figsize=(18, 12))
    fig.suptitle(f"{symbol} — Full VP Report — {timeframe}",
                 color=TEXT_COLOR, fontsize=14, fontweight="bold")

    # Top-left: Price + VP
    ax = axes[0, 0]
    ax.plot(range(len(df)), df["Close"].values, color=ACCENT1, linewidth=1)
    if "profile" in sub_results and sub_results["profile"]["success"]:
        pr = sub_results["profile"]["result"]
        ax.axhline(pr["poc"], color=POC_COLOR, linewidth=1.5, label="POC")
        ax.axhline(pr["vah"], color=VAH_COLOR, linewidth=1, linestyle="--", label="VAH")
        ax.axhline(pr["val"], color=VAL_COLOR, linewidth=1, linestyle="--", label="VAL")
        ax.axhspan(pr["val"], pr["vah"], alpha=0.05, color=VAH_COLOR)
    ax.axhline(current_price, color=ACCENT2, linewidth=0.8, linestyle=":")
    ax.set_title("Price + VP Levels", fontsize=10, color=TEXT_COLOR)
    ax.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
    ax.grid(True, alpha=0.2)

    # Top-right: VWAP
    ax = axes[0, 1]
    if "vwap" in sub_results and sub_results["vwap"]["success"]:
        tp = (df["High"] + df["Low"] + df["Close"]) / 3
        vol = df["Volume"].replace(0, 1)
        cum_tp_vol = (tp * vol).cumsum()
        cum_vol = vol.cumsum()
        vwap_line = cum_tp_vol / cum_vol
        sq_diff = ((tp - vwap_line) ** 2 * vol).cumsum()
        vwap_std = np.sqrt(sq_diff / cum_vol)

        x = range(len(df))
        ax.plot(x, df["Close"].values, color=ACCENT1, linewidth=1, label="Price")
        ax.plot(x, vwap_line.values, color=VWAP_COLOR, linewidth=1.5, label="VWAP")
        ax.fill_between(x, (vwap_line - vwap_std).values, (vwap_line + vwap_std).values,
                        alpha=0.1, color=VWAP_COLOR)
        ax.fill_between(x, (vwap_line - 2 * vwap_std).values, (vwap_line + 2 * vwap_std).values,
                        alpha=0.05, color=VWAP_COLOR)
        ax.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
    ax.set_title("VWAP Bands", fontsize=10, color=TEXT_COLOR)
    ax.grid(True, alpha=0.2)

    # Bottom-left: Delta
    ax = axes[1, 0]
    bar_range = df["High"] - df["Low"]
    bar_range = bar_range.replace(0, np.nan)
    vol = df["Volume"].replace(0, 1)
    delta = vol * (df["Close"] - df["Open"]) / bar_range
    delta = delta.fillna(0)
    cum_delta = delta.cumsum()
    x = range(len(df))
    colors_d = [UP_COLOR if v >= 0 else DOWN_COLOR for v in delta.values]
    ax.bar(x, delta.values, color=colors_d, alpha=0.5, width=0.8)
    ax_twin = ax.twinx()
    ax_twin.plot(x, cum_delta.values, color=ACCENT2, linewidth=1.5)
    ax_twin.set_ylabel("Cumulative", color=ACCENT2, fontsize=8)
    ax.set_title("Simulated Delta", fontsize=10, color=TEXT_COLOR)
    ax.grid(True, alpha=0.2)

    # Bottom-right: VP histogram
    ax = axes[1, 1]
    vp_data = compute_volume_profile(df, va_pct=0.70)
    if vp_data:
        colors_v = [HVN_COLOR if v > vp_data["hvn_threshold"] else
                    (LVN_COLOR if v < vp_data["lvn_threshold"] and v > 0 else "#4a5568")
                    for v in vp_data["volumes"]]
        ax.barh(vp_data["bin_centers"], vp_data["volumes"], height=vp_data["bin_width"] * 0.9,
                color=colors_v, alpha=0.7, edgecolor="none")
        ax.axhline(vp_data["poc_price"], color=POC_COLOR, linewidth=1.5)
        ax.axhline(vp_data["vah"], color=VAH_COLOR, linewidth=1, linestyle="--")
        ax.axhline(vp_data["val"], color=VAL_COLOR, linewidth=1, linestyle="--")
    ax.set_title("Volume Profile", fontsize=10, color=TEXT_COLOR)
    ax.grid(True, alpha=0.2)

    plt.tight_layout()
    save_chart(fig, chart_path)


# ===========================================================================
# Dispatcher
# ===========================================================================

MODES = {
    "profile": lambda data, chart: compute_profile(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "session": lambda data, chart: compute_session(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "shape": lambda data, chart: compute_shape(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "composite": lambda data, chart: compute_composite(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("all_tf_bars", None), chart),
    "vwap": lambda data, chart: compute_vwap(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "tpo": lambda data, chart: compute_tpo(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "delta": lambda data, chart: compute_delta(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "auction": lambda data, chart: compute_auction(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "confluence": lambda data, chart: compute_confluence(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("all_tf_bars", None), chart),
    "full": lambda data, chart: compute_full(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("all_tf_bars", None), chart),
}


def main():
    if len(sys.argv) < 3:
        print("Usage: python3 vp_engine.py <input.json> <output.json> [chart.png]", file=sys.stderr)
        sys.exit(1)

    input_path = sys.argv[1]
    output_path = sys.argv[2]
    chart_path = sys.argv[3] if len(sys.argv) > 3 else None

    with open(input_path) as f:
        data = json.load(f)

    mode = data.get("mode", "profile")
    symbol = data.get("symbol", "?")

    if mode not in MODES:
        result = output(mode, symbol, False, {}, "", error=f"Unknown mode: {mode}")
    else:
        try:
            result = MODES[mode](data, chart_path)
        except Exception as e:
            traceback.print_exc(file=sys.stderr)
            result = output(mode, symbol, False, {}, "", error=str(e))

    with open(output_path, "w") as f:
        json.dump(result, f, indent=2, default=str)

    print(f"VP engine: mode={mode}, symbol={symbol}, success={result['success']}")


if __name__ == "__main__":
    main()
