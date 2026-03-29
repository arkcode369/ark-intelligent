#!/usr/bin/env python3
"""
CTA Chart Renderer — Professional TA chart using mplfinance + matplotlib.

Usage: python3 cta_chart.py <input.json> <output.png>

Input JSON schema:
  symbol, timeframe, bars[], indicators{}, fibonacci{}, patterns[]
  mode: "default" | "ichimoku" | "fibonacci" | "zones"
  ichimoku: {tenkan_sen[], kijun_sen[], senkou_span_a[], senkou_span_b[], chikou_span[]}
  zones: {direction, entry_high, entry_low, stop_loss, take_profit_1, take_profit_2}

Output: PNG image, dark professional theme.
"""

import json
import sys
import warnings

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import mplfinance as mpf
import numpy as np
import pandas as pd

warnings.filterwarnings("ignore")

# ---------------------------------------------------------------------------
# Color Palette
# ---------------------------------------------------------------------------
BG_COLOR = "#0e1117"
PANEL_BG = "#0e1117"
GRID_COLOR = "#1e2530"
TEXT_COLOR = "#c9d1d9"
UP_COLOR = "#26a69a"
DOWN_COLOR = "#ef5350"
EMA9_COLOR = "#FFD700"
EMA21_COLOR = "#FF8C00"
EMA55_COLOR = "#00BCD4"
BB_COLOR = "#555555"
ST_UP_COLOR = "#26a69a"
ST_DOWN_COLOR = "#ef5350"
RSI_COLOR = "#AB47BC"
MACD_COLOR = "#42A5F5"
SIGNAL_COLOR = "#FF8C00"
FIB_COLOR = "#FFD54F"
BULL_ARROW = "#26a69a"
BEAR_ARROW = "#ef5350"
# Ichimoku colors
ICH_TENKAN = "#2196F3"    # Blue
ICH_KIJUN = "#F44336"     # Red
ICH_CHIKOU = "#9C27B0"    # Purple
ICH_CLOUD_BULL = "#26a69a"
ICH_CLOUD_BEAR = "#ef5350"
# Zone colors
ZONE_ENTRY = "#FFD700"
ZONE_SL = "#ef5350"
ZONE_TP = "#26a69a"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def safe_array(indicators: dict, key: str, length: int, price_range=None):
    arr = indicators.get(key)
    if arr is None or len(arr) == 0:
        return None
    arr = [float(v) if v is not None else np.nan for v in arr]
    if len(arr) < length:
        arr = [np.nan] * (length - len(arr)) + arr
    elif len(arr) > length:
        arr = arr[-length:]
    if price_range is not None:
        lo, hi = price_range
        margin = (hi - lo) * 2
        arr = [v if (not np.isnan(v) and lo - margin < v < hi + margin) else np.nan for v in arr]
    return arr


def determine_bar_limit(timeframe: str) -> int:
    tf = timeframe.lower()
    if tf in ("daily", "d", "1d"):
        return 80
    elif tf in ("weekly", "w", "1w"):
        return 52
    else:
        return 96


def build_dataframe(bars: list) -> pd.DataFrame:
    """Build OHLCV DataFrame from bar list, filtering bad data."""
    rows = []
    for b in bars:
        dt = b.get("date", "")
        ts = pd.Timestamp(dt)
        o, h, l, c = float(b["open"]), float(b["high"]), float(b["low"]), float(b["close"])
        v = float(b.get("volume", 0))
        if o <= 0 or h <= 0 or l <= 0 or c <= 0:
            continue
        rows.append({"Date": ts, "Open": o, "High": h, "Low": l, "Close": c, "Volume": v})
    if not rows:
        return pd.DataFrame()
    df = pd.DataFrame(rows)
    df["Date"] = pd.to_datetime(df["Date"])
    df.set_index("Date", inplace=True)
    df.sort_index(inplace=True)
    return df


def make_style():
    mc = mpf.make_marketcolors(
        up=UP_COLOR, down=DOWN_COLOR,
        edge={"up": UP_COLOR, "down": DOWN_COLOR},
        wick={"up": UP_COLOR, "down": DOWN_COLOR},
        volume={"up": UP_COLOR, "down": DOWN_COLOR},
    )
    return mpf.make_mpf_style(
        marketcolors=mc,
        figcolor=BG_COLOR,
        facecolor=PANEL_BG,
        gridcolor=GRID_COLOR,
        gridstyle=":",
        gridaxis="both",
        rc={
            "axes.labelcolor": TEXT_COLOR,
            "xtick.color": TEXT_COLOR,
            "ytick.color": TEXT_COLOR,
            "font.size": 8,
        },
    )


def save_chart(fig, output_path):
    fig.savefig(output_path, dpi=100, bbox_inches="tight", facecolor=BG_COLOR)
    plt.close(fig)
    print(f"Chart saved: {output_path}")


# ===========================================================================
# MODE: DEFAULT — candles + EMA + BB + SuperTrend + RSI + MACD
# ===========================================================================

def render_default(df, data, output_path):
    n = len(df)
    symbol = data.get("symbol", "???")
    timeframe = data.get("timeframe", "")
    indicators = data.get("indicators", {})
    fibonacci = data.get("fibonacci", {})
    patterns = data.get("patterns", [])

    last_date = df.index[-1].strftime("%d %b %Y")
    title = f"{symbol} — {timeframe} — {last_date}"

    price_lo, price_hi = df["Low"].min(), df["High"].max()
    price_range = (price_lo, price_hi)

    # Panels
    rsi = safe_array(indicators, "rsi", n)
    macd = safe_array(indicators, "macd", n)
    macd_signal = safe_array(indicators, "macd_signal", n)
    macd_hist = safe_array(indicators, "macd_histogram", n)
    has_rsi, has_macd = rsi is not None, macd is not None

    rsi_panel, macd_panel, next_panel = None, None, 1
    if has_rsi:
        rsi_panel = next_panel; next_panel += 1
    if has_macd:
        macd_panel = next_panel; next_panel += 1

    panel_ratios = [5.5]
    if has_rsi: panel_ratios.append(1.5)
    if has_macd: panel_ratios.append(1.5)

    addplots = []

    # EMA
    for key, color in [("ema9", EMA9_COLOR), ("ema21", EMA21_COLOR), ("ema55", EMA55_COLOR)]:
        arr = safe_array(indicators, key, n, price_range)
        if arr is not None:
            addplots.append(mpf.make_addplot(arr, panel=0, color=color, width=1.0, secondary_y=False))

    # BB
    bb_upper = safe_array(indicators, "bb_upper", n, price_range)
    bb_lower = safe_array(indicators, "bb_lower", n, price_range)
    if bb_upper and bb_lower:
        addplots.append(mpf.make_addplot(bb_upper, panel=0, color=BB_COLOR, width=0.7, linestyle="--", secondary_y=False))
        addplots.append(mpf.make_addplot(bb_lower, panel=0, color=BB_COLOR, width=0.7, linestyle="--", secondary_y=False))

    # SuperTrend
    st_vals = safe_array(indicators, "supertrend", n, price_range)
    st_dirs = indicators.get("supertrend_dir", [])
    if st_vals and st_dirs:
        if len(st_dirs) < n: st_dirs = [""] * (n - len(st_dirs)) + st_dirs
        elif len(st_dirs) > n: st_dirs = st_dirs[-n:]
        st_up = [v if d == "UP" else np.nan for v, d in zip(st_vals, st_dirs)]
        st_down = [v if d == "DOWN" else np.nan for v, d in zip(st_vals, st_dirs)]
        addplots.append(mpf.make_addplot(st_up, panel=0, color=ST_UP_COLOR, width=1.5, secondary_y=False))
        addplots.append(mpf.make_addplot(st_down, panel=0, color=ST_DOWN_COLOR, width=1.5, secondary_y=False))

    # RSI
    if has_rsi:
        addplots.append(mpf.make_addplot(rsi, panel=rsi_panel, color=RSI_COLOR, width=1.2, ylabel="RSI"))

    # MACD
    if has_macd:
        addplots.append(mpf.make_addplot(macd, panel=macd_panel, color=MACD_COLOR, width=1.0, ylabel="MACD"))
    if macd_signal and macd_panel is not None:
        addplots.append(mpf.make_addplot(macd_signal, panel=macd_panel, color=SIGNAL_COLOR, width=1.0))
    if macd_hist and macd_panel is not None:
        hist_colors = [UP_COLOR if (v or 0) >= 0 else DOWN_COLOR for v in macd_hist]
        addplots.append(mpf.make_addplot(macd_hist, panel=macd_panel, type="bar", color=hist_colors, width=0.7))

    # Plot
    style = make_style()
    plot_kwargs = dict(type="candle", style=style, volume=False, figsize=(14, 9),
                       returnfig=True, tight_layout=False, warn_too_much_data=999)
    if len(panel_ratios) > 1:
        plot_kwargs["panel_ratios"] = panel_ratios
    if addplots:
        plot_kwargs["addplot"] = addplots

    fig, axes = mpf.plot(df, **plot_kwargs)
    fig.suptitle(title, color=TEXT_COLOR, fontsize=13, fontweight="bold", y=0.98)
    main_ax = axes[0]

    # RSI lines
    if has_rsi:
        for ax in axes:
            if hasattr(ax, "get_ylabel") and ax.get_ylabel() == "RSI":
                ax.axhline(70, color=DOWN_COLOR, linestyle="--", linewidth=0.6, alpha=0.7)
                ax.axhline(30, color=UP_COLOR, linestyle="--", linewidth=0.6, alpha=0.7)
                ax.set_ylim(0, 100)
                break

    # Fibonacci levels (light)
    fib_levels = fibonacci.get("levels", {})
    for lname, pv in fib_levels.items():
        try:
            pv = float(pv)
            if price_lo * 0.9 < pv < price_hi * 1.1:
                main_ax.axhline(pv, color=FIB_COLOR, linestyle="--", linewidth=0.5, alpha=0.3)
        except (ValueError, TypeError):
            pass

    # Patterns
    for p in (patterns or [])[-10:]:
        try:
            bar_idx = int(p.get("bar_index", 0))
            direction = p.get("direction", "")
            name = p.get("name", "")
            plot_idx = n - 1 - bar_idx
            if plot_idx < 0 or plot_idx >= n: continue
            if direction == "BULLISH":
                main_ax.plot(plot_idx, df.iloc[plot_idx]["Low"] * 0.999, "^", color=BULL_ARROW, markersize=8, zorder=10)
                main_ax.annotate(name[:15], xy=(plot_idx, df.iloc[plot_idx]["Low"]), fontsize=5, color=BULL_ARROW, alpha=0.8, textcoords="offset points", xytext=(5, -12))
            elif direction == "BEARISH":
                main_ax.plot(plot_idx, df.iloc[plot_idx]["High"] * 1.001, "v", color=BEAR_ARROW, markersize=8, zorder=10)
                main_ax.annotate(name[:15], xy=(plot_idx, df.iloc[plot_idx]["High"]), fontsize=5, color=BEAR_ARROW, alpha=0.8, textcoords="offset points", xytext=(5, 8))
        except (ValueError, TypeError, IndexError):
            pass

    # BB fill
    if bb_upper and bb_lower:
        main_ax.fill_between(range(n),
            [v if not np.isnan(v) else 0 for v in bb_upper],
            [v if not np.isnan(v) else 0 for v in bb_lower],
            alpha=0.06, color=BB_COLOR)

    save_chart(fig, output_path)


# ===========================================================================
# MODE: ICHIMOKU — candles + Ichimoku Cloud + Tenkan/Kijun/Chikou
# ===========================================================================

def render_ichimoku(df, data, output_path):
    n = len(df)
    symbol = data.get("symbol", "???")
    timeframe = data.get("timeframe", "")
    ich = data.get("ichimoku", {})

    last_date = df.index[-1].strftime("%d %b %Y")
    title = f"{symbol} — Ichimoku Cloud — {timeframe} — {last_date}"

    price_lo, price_hi = df["Low"].min(), df["High"].max()
    price_range = (price_lo, price_hi)

    addplots = []

    # Tenkan-sen (blue, thin)
    tenkan = safe_array(ich, "tenkan_sen", n, price_range)
    if tenkan:
        addplots.append(mpf.make_addplot(tenkan, panel=0, color=ICH_TENKAN, width=1.0, secondary_y=False))

    # Kijun-sen (red, thin)
    kijun = safe_array(ich, "kijun_sen", n, price_range)
    if kijun:
        addplots.append(mpf.make_addplot(kijun, panel=0, color=ICH_KIJUN, width=1.0, secondary_y=False))

    # Chikou Span (purple, dashed)
    chikou = safe_array(ich, "chikou_span", n, price_range)
    if chikou:
        addplots.append(mpf.make_addplot(chikou, panel=0, color=ICH_CHIKOU, width=0.8, linestyle="--", secondary_y=False))

    # Senkou Span A & B (for cloud fill, added as lines)
    span_a = safe_array(ich, "senkou_span_a", n, price_range)
    span_b = safe_array(ich, "senkou_span_b", n, price_range)
    if span_a:
        addplots.append(mpf.make_addplot(span_a, panel=0, color=ICH_CLOUD_BULL, width=0.5, alpha=0.5, secondary_y=False))
    if span_b:
        addplots.append(mpf.make_addplot(span_b, panel=0, color=ICH_CLOUD_BEAR, width=0.5, alpha=0.5, secondary_y=False))

    style = make_style()
    plot_kwargs = dict(type="candle", style=style, volume=False, figsize=(14, 8),
                       returnfig=True, tight_layout=False, warn_too_much_data=999)
    if addplots:
        plot_kwargs["addplot"] = addplots

    fig, axes = mpf.plot(df, **plot_kwargs)
    fig.suptitle(title, color=TEXT_COLOR, fontsize=13, fontweight="bold", y=0.98)
    main_ax = axes[0]

    # Cloud fill between Senkou A and B
    if span_a and span_b:
        x = range(n)
        a_clean = [v if not np.isnan(v) else 0 for v in span_a]
        b_clean = [v if not np.isnan(v) else 0 for v in span_b]
        # Bullish cloud (A > B) = green, Bearish cloud (A < B) = red
        main_ax.fill_between(x, a_clean, b_clean,
                             where=[a >= b for a, b in zip(a_clean, b_clean)],
                             alpha=0.15, color=ICH_CLOUD_BULL, interpolate=True)
        main_ax.fill_between(x, a_clean, b_clean,
                             where=[a < b for a, b in zip(a_clean, b_clean)],
                             alpha=0.15, color=ICH_CLOUD_BEAR, interpolate=True)

    # Legend
    legend_items = []
    if tenkan: legend_items.append(("Tenkan (9)", ICH_TENKAN))
    if kijun: legend_items.append(("Kijun (26)", ICH_KIJUN))
    if chikou: legend_items.append(("Chikou", ICH_CHIKOU))
    if span_a: legend_items.append(("Cloud", ICH_CLOUD_BULL))
    for i, (label, color) in enumerate(legend_items):
        main_ax.text(0.01, 0.97 - i * 0.04, f"● {label}", transform=main_ax.transAxes,
                     color=color, fontsize=7, verticalalignment="top")

    save_chart(fig, output_path)


# ===========================================================================
# MODE: FIBONACCI — candles + fib levels + swing markers
# ===========================================================================

def render_fibonacci(df, data, output_path):
    n = len(df)
    symbol = data.get("symbol", "???")
    timeframe = data.get("timeframe", "")
    fibonacci = data.get("fibonacci", {})

    last_date = df.index[-1].strftime("%d %b %Y")
    title = f"{symbol} — Fibonacci Retracement — {timeframe} — {last_date}"

    price_lo, price_hi = df["Low"].min(), df["High"].max()

    style = make_style()
    fig, axes = mpf.plot(df, type="candle", style=style, volume=False, figsize=(14, 8),
                         returnfig=True, tight_layout=False, warn_too_much_data=999)
    fig.suptitle(title, color=TEXT_COLOR, fontsize=13, fontweight="bold", y=0.98)
    main_ax = axes[0]

    # Draw Fibonacci levels
    levels = fibonacci.get("levels", {})
    trend_dir = fibonacci.get("trend_dir", "UP")
    swing_high = fibonacci.get("swing_high", 0)
    swing_low = fibonacci.get("swing_low", 0)

    # Color gradient for fib levels
    fib_colors = {
        "0": "#4CAF50",     # Green
        "23.6": "#8BC34A",  # Light green
        "38.2": "#FFD700",  # Gold
        "50": "#FFA500",    # Orange
        "61.8": "#FF6347",  # Tomato — golden ratio
        "78.6": "#E91E63",  # Pink
        "100": "#F44336",   # Red
    }

    for level_name, price_val in levels.items():
        try:
            pv = float(price_val)
            color = fib_colors.get(level_name, FIB_COLOR)
            is_golden = level_name == "61.8"
            lw = 1.2 if is_golden else 0.8
            alpha = 0.9 if is_golden else 0.6

            main_ax.axhline(pv, color=color, linestyle="--" if not is_golden else "-",
                          linewidth=lw, alpha=alpha, zorder=5)
            main_ax.text(n + 0.5, pv, f" {level_name}% — {pv:.4f}",
                        color=color, fontsize=7, alpha=0.9,
                        verticalalignment="center", fontweight="bold" if is_golden else "normal")
        except (ValueError, TypeError):
            pass

    # Swing markers
    swing_high_idx = fibonacci.get("swing_high_idx", -1)
    swing_low_idx = fibonacci.get("swing_low_idx", -1)

    if 0 <= swing_high_idx < n:
        plot_idx = n - 1 - swing_high_idx
        if 0 <= plot_idx < n:
            main_ax.plot(plot_idx, swing_high, "v", color=DOWN_COLOR, markersize=12, zorder=10)
            main_ax.annotate(f"Swing High\n{swing_high:.4f}", xy=(plot_idx, swing_high),
                           fontsize=6, color=DOWN_COLOR, textcoords="offset points", xytext=(8, 10),
                           fontweight="bold")

    if 0 <= swing_low_idx < n:
        plot_idx = n - 1 - swing_low_idx
        if 0 <= plot_idx < n:
            main_ax.plot(plot_idx, swing_low, "^", color=UP_COLOR, markersize=12, zorder=10)
            main_ax.annotate(f"Swing Low\n{swing_low:.4f}", xy=(plot_idx, swing_low),
                           fontsize=6, color=UP_COLOR, textcoords="offset points", xytext=(8, -18),
                           fontweight="bold")

    # Shade between key fib levels (38.2 - 61.8 = "golden zone")
    if "38.2" in levels and "61.8" in levels:
        try:
            l382 = float(levels["38.2"])
            l618 = float(levels["61.8"])
            main_ax.axhspan(min(l382, l618), max(l382, l618), alpha=0.08, color=FIB_COLOR, zorder=1)
            main_ax.text(0.01, 0.03, "Golden Zone (38.2% – 61.8%)", transform=main_ax.transAxes,
                        color=FIB_COLOR, fontsize=7, alpha=0.7)
        except (ValueError, TypeError):
            pass

    # Trend direction label
    main_ax.text(0.01, 0.97, f"Trend: {trend_dir}", transform=main_ax.transAxes,
                color=UP_COLOR if trend_dir == "UP" else DOWN_COLOR, fontsize=8,
                fontweight="bold", verticalalignment="top")

    save_chart(fig, output_path)


# ===========================================================================
# MODE: ZONES — candles + Entry Zone + SL + TP1/TP2
# ===========================================================================

def render_zones(df, data, output_path):
    n = len(df)
    symbol = data.get("symbol", "???")
    timeframe = data.get("timeframe", "")
    zones = data.get("zones", {})

    last_date = df.index[-1].strftime("%d %b %Y")
    direction = zones.get("direction", "LONG")
    title = f"{symbol} — Trade Setup ({direction}) — {timeframe} — {last_date}"

    style = make_style()
    fig, axes = mpf.plot(df, type="candle", style=style, volume=False, figsize=(14, 8),
                         returnfig=True, tight_layout=False, warn_too_much_data=999)
    fig.suptitle(title, color=TEXT_COLOR, fontsize=13, fontweight="bold", y=0.98)
    main_ax = axes[0]

    entry_high = zones.get("entry_high", 0)
    entry_low = zones.get("entry_low", 0)
    sl = zones.get("stop_loss", 0)
    tp1 = zones.get("take_profit_1", 0)
    tp2 = zones.get("take_profit_2", 0)

    # Entry zone (shaded)
    if entry_high > 0 and entry_low > 0:
        main_ax.axhspan(entry_low, entry_high, alpha=0.15, color=ZONE_ENTRY, zorder=1)
        main_ax.axhline(entry_high, color=ZONE_ENTRY, linestyle="-", linewidth=1.0, alpha=0.8)
        main_ax.axhline(entry_low, color=ZONE_ENTRY, linestyle="-", linewidth=1.0, alpha=0.8)
        mid_entry = (entry_high + entry_low) / 2
        main_ax.text(n + 0.5, mid_entry, f" Entry {entry_low:.4f}–{entry_high:.4f}",
                    color=ZONE_ENTRY, fontsize=7, fontweight="bold", verticalalignment="center")

    # Stop Loss (red dashed)
    if sl > 0:
        main_ax.axhline(sl, color=ZONE_SL, linestyle="--", linewidth=1.5, alpha=0.9, zorder=5)
        main_ax.text(n + 0.5, sl, f" SL {sl:.4f}", color=ZONE_SL, fontsize=7,
                    fontweight="bold", verticalalignment="center")

    # TP1 (green dashed)
    if tp1 > 0:
        main_ax.axhline(tp1, color=ZONE_TP, linestyle="--", linewidth=1.2, alpha=0.8, zorder=5)
        main_ax.text(n + 0.5, tp1, f" TP1 {tp1:.4f}", color=ZONE_TP, fontsize=7,
                    fontweight="bold", verticalalignment="center")

    # TP2 (green dotted)
    if tp2 > 0:
        main_ax.axhline(tp2, color=ZONE_TP, linestyle=":", linewidth=1.0, alpha=0.6, zorder=5)
        main_ax.text(n + 0.5, tp2, f" TP2 {tp2:.4f}", color=ZONE_TP, fontsize=7,
                    verticalalignment="center")

    # Direction arrow
    dir_color = UP_COLOR if direction == "LONG" else DOWN_COLOR
    dir_symbol = "▲ LONG" if direction == "LONG" else "▼ SHORT"
    main_ax.text(0.01, 0.97, dir_symbol, transform=main_ax.transAxes,
                color=dir_color, fontsize=10, fontweight="bold", verticalalignment="top")

    # Risk/Reward info
    rr1 = zones.get("risk_reward_1", 0)
    rr2 = zones.get("risk_reward_2", 0)
    conf = zones.get("confidence", "")
    if rr1 > 0:
        main_ax.text(0.01, 0.92, f"R:R  TP1: 1:{rr1:.1f}  |  TP2: 1:{rr2:.1f}  |  {conf}",
                    transform=main_ax.transAxes, color=TEXT_COLOR, fontsize=7, verticalalignment="top")

    save_chart(fig, output_path)


# ===========================================================================
# Dispatcher
# ===========================================================================

def render_chart(data: dict, output_path: str):
    bars = data.get("bars", [])
    if not bars:
        print("ERROR: No bars in input data", file=sys.stderr)
        sys.exit(1)

    timeframe = data.get("timeframe", "")
    bar_limit = determine_bar_limit(timeframe)
    bars = bars[-bar_limit:]
    data["bars"] = bars  # update trimmed bars

    df = build_dataframe(bars)
    if df.empty:
        print("ERROR: No valid bars after filtering", file=sys.stderr)
        sys.exit(1)

    mode = data.get("mode", "default")

    try:
        if mode == "ichimoku":
            render_ichimoku(df, data, output_path)
        elif mode == "fibonacci":
            render_fibonacci(df, data, output_path)
        elif mode == "zones":
            render_zones(df, data, output_path)
        else:
            render_default(df, data, output_path)
    except Exception as e:
        print(f"ERROR in mode '{mode}': {e}", file=sys.stderr)
        # Fallback: try default mode
        try:
            render_default(df, data, output_path)
        except Exception as e2:
            print(f"FALLBACK also failed: {e2}", file=sys.stderr)
            sys.exit(1)


def main():
    if len(sys.argv) < 3:
        print("Usage: python3 cta_chart.py <input.json> <output.png>", file=sys.stderr)
        sys.exit(1)

    with open(sys.argv[1], "r") as f:
        data = json.load(f)

    render_chart(data, sys.argv[2])


if __name__ == "__main__":
    main()
