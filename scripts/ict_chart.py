#!/usr/bin/env python3
"""
ICT Chart Renderer — Inner Circle Trader structural analysis chart.

Usage: python3 ict_chart.py <input.json> <output.png>

Input JSON schema:
  symbol, timeframe, bars[],
  fvg_zones: [{type, high, low, bar_index, filled, fill_pct}],
  order_blocks: [{type, high, low, bar_index, broken}],
  structure: [{type, direction, level, bar_index}],
  sweeps: [{type, level, bar_index, reversed}],
  ote: [{direction, high, low, midpoint}],
  equilibrium, premium_zone, discount_zone, current_price,
  liquidity_levels: [{price, type, count, swept}]

Output: PNG image, dark professional theme with ICT overlays.
"""

import json
import sys
import warnings

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
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

# ICT-specific colors
FVG_BULL_COLOR = "#26a69a"      # Bullish FVG — green
FVG_BEAR_COLOR = "#ef5350"      # Bearish FVG — red
FVG_BULL_FILL = "#26a69a"       # Filled bullish FVG — dimmed
FVG_BEAR_FILL = "#ef5350"
OB_BULL_COLOR = "#2196F3"       # Bullish Order Block — blue
OB_BEAR_COLOR = "#FF9800"       # Bearish Order Block — orange
BREAKER_COLOR = "#9C27B0"       # Breaker Block — purple
OTE_BULL_COLOR = "#00E676"      # Bullish OTE — bright green
OTE_BEAR_COLOR = "#FF5252"      # Bearish OTE — bright red
BOS_BULL_COLOR = "#26a69a"      # BOS bullish
BOS_BEAR_COLOR = "#ef5350"     # BOS bearish
CHOCH_COLOR = "#FFD700"         # CHoCH — gold
SWEEP_HIGH_COLOR = "#FF6D00"    # Sweep high — orange
SWEEP_LOW_COLOR = "#00B0FF"     # Sweep low — cyan
EQUIL_COLOR = "#FFFFFF"         # Equilibrium — white
PREMIUM_COLOR = "#ef5350"       # Premium zone — red
DISCOUNT_COLOR = "#26a69a"      # Discount zone — green
LL_BUY_COLOR = "#FF6D00"        # Buy-side liquidity — orange
LL_SELL_COLOR = "#00B0FF"       # Sell-side liquidity — cyan

# Killzone box colors (matching PineScript reference)
KZ_ASIAN_COLOR = "#2196F3"      # Asian — blue
KZ_LONDON_COLOR = "#ef5350"     # London — red
KZ_NYAM_COLOR = "#089981"       # NY AM — teal
KZ_NYLU_COLOR = "#FFD700"       # NY Lunch — yellow
KZ_NYPM_COLOR = "#9C27B0"       # NY PM — purple

# DWM Pivot colors
PDH_PDL_COLOR = "#2196F3"       # Previous Day H/L — blue
PWH_PWL_COLOR = "#089981"       # Previous Week H/L — teal
PMH_PML_COLOR = "#ef5350"       # Previous Month H/L — red

# Silver Bullet box colors
SB_LONDON_AM_COLOR = "#ef5350"  # London AM SB — red
SB_NY_AM_COLOR = "#089981"      # NY AM SB — teal
SB_NY_PM_COLOR = "#9C27B0"      # NY PM SB — purple

# PO3 / AMD colors
PO3_ACC_COLOR = "#FFD700"       # Accumulation — gold
PO3_MAN_COLOR = "#FF6D00"       # Manipulation — orange
PO3_DIST_COLOR = "#00E676"      # Distribution — green

# Relevant anchor colors
ANCHOR_HIGH_COLOR = "#FF5252"   # Relevant High — bright red
ANCHOR_LOW_COLOR = "#00E676"    # Relevant Low — bright green


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def build_dataframe(bars: list) -> pd.DataFrame:
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


def bar_index_to_plot_idx(bar_index, n):
    """Convert newest-first bar_index to plot x-position (oldest-first)."""
    idx = n - 1 - bar_index
    if idx < 0 or idx >= n:
        return None
    return idx


# ===========================================================================
# ICT Chart Renderer
# ===========================================================================

def render_ict_chart(data: dict, output_path: str):
    bars = data.get("bars", [])
    if not bars:
        print("ERROR: No bars in input data", file=sys.stderr)
        sys.exit(1)

    # Limit bars for readability
    timeframe = data.get("timeframe", "")
    tf_lower = timeframe.lower()
    if tf_lower in ("daily", "d", "1d"):
        bar_limit = 80
    else:
        bar_limit = 96
    bars = bars[-bar_limit:]

    df = build_dataframe(bars)
    if df.empty:
        print("ERROR: No valid bars after filtering", file=sys.stderr)
        sys.exit(1)

    n = len(df)
    symbol = data.get("symbol", "???")
    last_date = df.index[-1].strftime("%d %b %Y")
    title = f"{symbol} — ICT Analysis — {timeframe} — {last_date}"

    price_lo, price_hi = df["Low"].min(), df["High"].max()

    # Plot candlestick
    style = make_style()
    fig, axes = mpf.plot(df, type="candle", style=style, volume=False,
                         figsize=(16, 10), returnfig=True,
                         tight_layout=False, warn_too_much_data=999)
    fig.suptitle(title, color=TEXT_COLOR, fontsize=13, fontweight="bold", y=0.98)
    main_ax = axes[0]

    # =======================================================================
    # 1. Equilibrium / Premium / Discount Zone
    # =======================================================================
    equilibrium = data.get("equilibrium", 0)
    premium_zone = data.get("premium_zone", False)
    discount_zone = data.get("discount_zone", False)
    current_price = data.get("current_price", 0)

    if equilibrium > 0 and price_lo < equilibrium < price_hi:
        main_ax.axhline(equilibrium, color=EQUIL_COLOR, linestyle="-.",
                        linewidth=1.0, alpha=0.7, zorder=4)
        main_ax.text(n + 0.5, equilibrium, f" EQ {equilibrium:.5f}",
                     color=EQUIL_COLOR, fontsize=7, va="center", fontweight="bold")

        # Shade premium/discount zones
        if premium_zone:
            main_ax.axhspan(equilibrium, price_hi * 1.01, alpha=0.04,
                            color=PREMIUM_COLOR, zorder=1)
            main_ax.text(0.01, 0.97, "🔴 PREMIUM", transform=main_ax.transAxes,
                         color=PREMIUM_COLOR, fontsize=8, fontweight="bold", va="top")
        elif discount_zone:
            main_ax.axhspan(price_lo * 0.99, equilibrium, alpha=0.04,
                            color=DISCOUNT_COLOR, zorder=1)
            main_ax.text(0.01, 0.97, "🟢 DISCOUNT", transform=main_ax.transAxes,
                         color=DISCOUNT_COLOR, fontsize=8, fontweight="bold", va="top")

    # Current price line
    if current_price > 0 and price_lo < current_price < price_hi:
        main_ax.axhline(current_price, color="#FFFFFF", linestyle=":",
                        linewidth=0.6, alpha=0.5, zorder=3)

    # =======================================================================
    # 2. Fair Value Gaps (FVG) — shaded rectangles
    # =======================================================================
    fvg_zones = data.get("fvg_zones", [])
    for fvg in fvg_zones[:8]:  # limit to 8 for readability
        bar_idx = fvg.get("bar_index", -1)
        plot_idx = bar_index_to_plot_idx(bar_idx, n)
        if plot_idx is None:
            continue

        fvg_high = fvg.get("high", 0)
        fvg_low = fvg.get("low", 0)
        fvg_type = fvg.get("type", "BULLISH")
        filled = fvg.get("filled", False)
        fill_pct = fvg.get("fill_pct", 0)

        color = FVG_BULL_COLOR if fvg_type == "BULLISH" else FVG_BEAR_COLOR
        alpha = 0.08 if filled else 0.18

        # Draw FVG as horizontal band spanning 3 candles
        x_start = max(0, plot_idx - 1)
        x_end = min(n - 1, plot_idx + 1)
        main_ax.fill_between([x_start, x_end], fvg_low, fvg_high,
                             alpha=alpha, color=color, zorder=2)
        # Border lines
        main_ax.hlines(fvg_high, x_start, x_end, colors=color,
                       linewidth=0.8, alpha=0.6, zorder=3)
        main_ax.hlines(fvg_low, x_start, x_end, colors=color,
                       linewidth=0.8, alpha=0.6, zorder=3)

        # Label
        label = f"FVG {fvg_type[:1]}"
        if filled:
            label += " ✓"
        elif fill_pct > 0:
            label += f" {fill_pct:.0f}%"
        mid = (fvg_high + fvg_low) / 2
        main_ax.text(x_end + 0.3, mid, label, color=color,
                     fontsize=5, va="center", alpha=0.8)

    # =======================================================================
    # 3. Order Blocks — shaded zones
    # =======================================================================
    order_blocks = data.get("order_blocks", [])
    for ob in order_blocks[:6]:
        bar_idx = ob.get("bar_index", -1)
        plot_idx = bar_index_to_plot_idx(bar_idx, n)
        if plot_idx is None:
            continue

        ob_high = ob.get("high", 0)
        ob_low = ob.get("low", 0)
        ob_type = ob.get("type", "BULLISH")
        broken = ob.get("broken", False)

        if broken:
            color = BREAKER_COLOR
            alpha = 0.12
            label = "⚡BRK"
        else:
            color = OB_BULL_COLOR if ob_type == "BULLISH" else OB_BEAR_COLOR
            alpha = 0.15
            label = "OB"

        # Draw OB zone as a band spanning the candle ±1
        x_start = max(0, plot_idx - 1)
        x_end = min(n - 1, plot_idx + 1)
        main_ax.fill_between([x_start, x_end], ob_low, ob_high,
                             alpha=alpha, color=color, zorder=2)
        main_ax.hlines(ob_high, x_start, x_end, colors=color,
                       linewidth=1.0, alpha=0.8, zorder=3)
        main_ax.hlines(ob_low, x_start, x_end, colors=color,
                       linewidth=1.0, alpha=0.8, zorder=3)

        # Label on right side
        mid = (ob_high + ob_low) / 2
        main_ax.text(x_end + 0.3, mid, f"{label} {ob_type[:1]}",
                     color=color, fontsize=5, va="center", alpha=0.9, fontweight="bold")

    # =======================================================================
    # 4. OTE Zones — dashed lines + shaded band
    # =======================================================================
    ote_zones = data.get("ote", [])
    for ote in ote_zones[:3]:
        ote_high = ote.get("high", 0)
        ote_low = ote.get("low", 0)
        ote_mid = ote.get("midpoint", 0)
        ote_dir = ote.get("direction", "BULLISH")

        color = OTE_BULL_COLOR if ote_dir == "BULLISH" else OTE_BEAR_COLOR

        # Shade the OTE zone across the entire chart
        main_ax.axhspan(ote_low, ote_high, alpha=0.08, color=color, zorder=1)
        main_ax.axhline(ote_high, color=color, linestyle="--",
                        linewidth=0.7, alpha=0.6, zorder=3)
        main_ax.axhline(ote_low, color=color, linestyle="--",
                        linewidth=0.7, alpha=0.6, zorder=3)
        # Midpoint (sweet spot)
        main_ax.axhline(ote_mid, color=color, linestyle=":",
                        linewidth=0.5, alpha=0.4, zorder=3)

        # Labels on right
        main_ax.text(n + 0.5, ote_high, f" OTE 62% {ote_high:.5f}",
                     color=color, fontsize=6, va="center")
        main_ax.text(n + 0.5, ote_low, f" OTE 79% {ote_low:.5f}",
                     color=color, fontsize=6, va="center")

        # Check if current price is in OTE zone
        if current_price > 0 and ote_low <= current_price <= ote_high:
            main_ax.text(0.01, 0.92, f"⚡ PRICE IN OTE ZONE ({ote_dir})",
                         transform=main_ax.transAxes, color=color,
                         fontsize=8, fontweight="bold", va="top")

    # =======================================================================
    # 5. Structure Events (BOS / CHoCH) — markers on candles
    # =======================================================================
    structure = data.get("structure", [])
    for ev in structure[-6:]:  # last 6 events
        bar_idx = ev.get("bar_index", -1)
        plot_idx = bar_index_to_plot_idx(bar_idx, n)
        if plot_idx is None:
            continue

        ev_type = ev.get("type", "BOS")
        ev_dir = ev.get("direction", "BULLISH")
        ev_level = ev.get("level", 0)

        if ev_type == "CHOCH":
            color = CHOCH_COLOR
            marker = "D"  # diamond
            ms = 7
            label = "CHoCH"
        else:  # BOS
            color = BOS_BULL_COLOR if ev_dir == "BULLISH" else BOS_BEAR_COLOR
            marker = "^" if ev_dir == "BULLISH" else "v"
            ms = 8
            label = "BOS"

        # Place marker above/below the candle
        if ev_dir == "BULLISH":
            y = df.iloc[plot_idx]["Low"] * 0.9995
        else:
            y = df.iloc[plot_idx]["High"] * 1.0005

        main_ax.plot(plot_idx, y, marker, color=color, markersize=ms,
                     zorder=10, markeredgecolor="white", markeredgewidth=0.3)
        main_ax.annotate(f"{label}", xy=(plot_idx, y), fontsize=5,
                         color=color, fontweight="bold", alpha=0.9,
                         textcoords="offset points", xytext=(3, -10 if ev_dir == "BULLISH" else 8))

    # =======================================================================
    # 6. Liquidity Sweeps — markers
    # =======================================================================
    sweeps = data.get("sweeps", [])
    for sw in sweeps[-4:]:
        bar_idx = sw.get("bar_index", -1)
        plot_idx = bar_index_to_plot_idx(bar_idx, n)
        if plot_idx is None:
            continue

        sw_type = sw.get("type", "SWEEP_HIGH")
        sw_level = sw.get("level", 0)
        reversed_flag = sw.get("reversed", False)

        color = SWEEP_HIGH_COLOR if sw_type == "SWEEP_HIGH" else SWEEP_LOW_COLOR

        if sw_type == "SWEEP_HIGH":
            y = df.iloc[plot_idx]["High"] * 1.0005
            marker = "v"
        else:
            y = df.iloc[plot_idx]["Low"] * 0.9995
            marker = "^"

        main_ax.plot(plot_idx, y, marker, color=color, markersize=9,
                     zorder=10, markeredgecolor="white", markeredgewidth=0.5)
        label = "💧SWEEP"
        if reversed_flag:
            label += "↩"
        main_ax.annotate(label, xy=(plot_idx, y), fontsize=5,
                         color=color, fontweight="bold", alpha=0.9,
                         textcoords="offset points",
                         xytext=(3, 8 if sw_type == "SWEEP_HIGH" else -12))

    # =======================================================================
    # 7. Liquidity Levels (Buy-side / Sell-side pools)
    # =======================================================================
    liq_levels = data.get("liquidity_levels", [])
    for ll in liq_levels[:6]:
        price = ll.get("price", 0)
        ll_type = ll.get("type", "BUY_SIDE")
        count = ll.get("count", 0)
        swept = ll.get("swept", False)

        if price <= 0 or not (price_lo < price < price_hi):
            continue

        color = LL_BUY_COLOR if ll_type == "BUY_SIDE" else LL_SELL_COLOR
        style_line = ":" if swept else "--"
        alpha = 0.4 if swept else 0.7

        main_ax.axhline(price, color=color, linestyle=style_line,
                        linewidth=0.6, alpha=alpha, zorder=3)
        label = f"{'🧹' if swept else '🎯'}{ll_type[:1]}L x{count}"
        main_ax.text(n + 0.5, price, f" {label} {price:.5f}",
                     color=color, fontsize=5, va="center", alpha=0.8)

    # =======================================================================
    # 8. Killzone Boxes — time window boxes with pivot lines
    # =======================================================================
    killzone_boxes = data.get("killzone_boxes", [])
    kz_color_map = {
        "ASIAN": KZ_ASIAN_COLOR,
        "LONDON": KZ_LONDON_COLOR,
        "NY_AM": KZ_NYAM_COLOR,
        "NY_LUNCH": KZ_NYLU_COLOR,
        "NY_PM": KZ_NYPM_COLOR,
    }
    for kz in killzone_boxes:
        kz_name = kz.get("name", "")
        kz_high = kz.get("high", 0)
        kz_low = kz.get("low", 0)
        kz_start = kz.get("start_utc", 0)
        kz_end = kz.get("end_utc", 0)
        kz_mitigated = kz.get("mitigated", False)
        kz_date = kz.get("date", "")

        color = kz_color_map.get(kz_name, KZ_ASIAN_COLOR)
        alpha_box = 0.06 if kz_mitigated else 0.12

        # Find the bar indices that correspond to this killzone window
        x_start = None
        x_end = None
        for i, (idx, row) in enumerate(df.iterrows()):
            bar_hour = idx.hour if hasattr(idx, "hour") else 0
            bar_date_str = idx.strftime("%Y-%m-%d") if hasattr(idx, "strftime") else ""
            if bar_date_str == kz_date[:10] if isinstance(kz_date, str) else False:
                if kz_start > kz_end:  # overnight session
                    if bar_hour >= kz_start or bar_hour < kz_end:
                        if x_start is None:
                            x_start = i
                        x_end = i
                else:
                    if kz_start <= bar_hour < kz_end:
                        if x_start is None:
                            x_start = i
                        x_end = i

        if x_start is not None and x_end is not None and kz_high > kz_low > 0:
            # Draw killzone box
            rect = mpatches.FancyBboxPatch(
                (x_start - 0.5, kz_low), x_end - x_start + 1, kz_high - kz_low,
                boxstyle="square,pad=0", facecolor=color, alpha=alpha_box,
                edgecolor=color, linewidth=0.8, zorder=2
            )
            main_ax.add_patch(rect)

            # Killzone label at top of box
            main_ax.text(x_start, kz_high, f" {kz_name}",
                         color=color, fontsize=5, va="bottom", fontweight="bold", alpha=0.9)

            # Pivot high/low lines extending from the box to the right
            if not kz_mitigated:
                main_ax.hlines(kz_high, x_start, n - 1, colors=color,
                               linewidth=0.6, alpha=0.5, linestyle="--", zorder=3)
                main_ax.hlines(kz_low, x_start, n - 1, colors=color,
                               linewidth=0.6, alpha=0.5, linestyle="--", zorder=3)
                main_ax.text(n + 0.5, kz_high, f" {kz_name}.H {kz_high:.5f}",
                             color=color, fontsize=5, va="center", alpha=0.7)
                main_ax.text(n + 0.5, kz_low, f" {kz_name}.L {kz_low:.5f}",
                             color=color, fontsize=5, va="center", alpha=0.7)

    # =======================================================================
    # 9. DWM Pivots — Previous Day/Week/Month High/Low lines
    # =======================================================================
    dwm_pivots = data.get("dwm_pivots", [])
    dwm_color_map = {
        "PDH": PDH_PDL_COLOR, "PDL": PDH_PDL_COLOR,
        "PWH": PWH_PWL_COLOR, "PWL": PWH_PWL_COLOR,
        "PMH": PMH_PML_COLOR, "PML": PMH_PML_COLOR,
    }
    for piv in dwm_pivots:
        piv_type = piv.get("type", "")
        piv_level = piv.get("level", 0)
        piv_broken = piv.get("broken", False)

        if piv_level <= 0 or not (price_lo < piv_level < price_hi):
            continue

        color = dwm_color_map.get(piv_type, PDH_PDL_COLOR)
        style_line = ":" if piv_broken else "-"
        alpha = 0.4 if piv_broken else 0.8
        lw = 0.8 if piv_broken else 1.2

        main_ax.axhline(piv_level, color=color, linestyle=style_line,
                        linewidth=lw, alpha=alpha, zorder=4)
        broken_mark = " ✗" if piv_broken else ""
        main_ax.text(n + 0.5, piv_level, f" {piv_type}{broken_mark} {piv_level:.5f}",
                     color=color, fontsize=6, va="center", fontweight="bold", alpha=alpha)

    # =======================================================================
    # 10. Silver Bullet Boxes — time window boxes with price range
    # =======================================================================
    silver_bullets = data.get("silver_bullets", [])
    sb_color_map = {
        "LONDON_AM": SB_LONDON_AM_COLOR,
        "NY_AM": SB_NY_AM_COLOR,
        "NY_PM": SB_NY_PM_COLOR,
    }
    for sb in silver_bullets:
        sb_window = sb.get("window", "")
        sb_high = sb.get("high", 0)
        sb_low = sb.get("low", 0)
        sb_start = sb.get("start_utc", 0)
        sb_end = sb.get("end_utc", 0)
        sb_active = sb.get("active", False)
        sb_date = sb.get("date", "")

        if sb_high <= 0 or sb_low <= 0:
            continue

        color = sb_color_map.get(sb_window, SB_NY_AM_COLOR)
        alpha_box = 0.15 if sb_active else 0.06

        # Find bar indices for this window
        x_start = None
        x_end = None
        for i, (idx, row) in enumerate(df.iterrows()):
            bar_hour = idx.hour if hasattr(idx, "hour") else 0
            bar_date_str = idx.strftime("%Y-%m-%d") if hasattr(idx, "strftime") else ""
            if isinstance(sb_date, str) and bar_date_str == sb_date[:10]:
                if sb_start <= bar_hour < sb_end:
                    if x_start is None:
                        x_start = i
                    x_end = i

        if x_start is not None and x_end is not None:
            rect = mpatches.FancyBboxPatch(
                (x_start - 0.5, sb_low), x_end - x_start + 1, sb_high - sb_low,
                boxstyle="square,pad=0", facecolor=color, alpha=alpha_box,
                edgecolor=color, linewidth=1.0 if sb_active else 0.5, zorder=2
            )
            main_ax.add_patch(rect)

            label = f"🎯{sb_window}"
            if sb_active:
                label += " ACTIVE"
            main_ax.text(x_start, sb_high, f" {label}",
                         color=color, fontsize=5, va="bottom", fontweight="bold", alpha=0.9)

    # =======================================================================
    # 11. PO3 (AMD) — Accumulation range shading + phase markers
    # =======================================================================
    amd_phases = data.get("amd", [])
    for amd in amd_phases:
        amd_session = amd.get("session", "")
        amd_tf = amd.get("tf", "SESSION")
        amd_phase = amd.get("phase", "")
        amd_dir = amd.get("direction", "")
        acc_high = amd.get("acc_high", 0)
        acc_low = amd.get("acc_low", 0)
        range_high = amd.get("range_high", 0)
        range_low = amd.get("range_low", 0)

        if acc_high <= 0 or acc_low <= 0:
            continue

        # Color based on phase
        if amd_phase == "ACCUMULATION":
            color = PO3_ACC_COLOR
        elif amd_phase == "MANIPULATION":
            color = PO3_MAN_COLOR
        else:
            color = PO3_DIST_COLOR

        # Shade accumulation range
        if price_lo < acc_high < price_hi and price_lo < acc_low < price_hi:
            main_ax.axhspan(acc_low, acc_high, alpha=0.06, color=color, zorder=1)
            main_ax.axhline(acc_high, color=color, linestyle=":",
                            linewidth=0.5, alpha=0.5, zorder=3)
            main_ax.axhline(acc_low, color=color, linestyle=":",
                            linewidth=0.5, alpha=0.5, zorder=3)

            tf_label = amd_tf if amd_tf != "SESSION" else ""
            main_ax.text(n + 0.5, acc_high,
                         f" PO3{tf_label} {amd_session} ACC↑ {acc_high:.5f}",
                         color=color, fontsize=5, va="center", alpha=0.7)
            main_ax.text(n + 0.5, acc_low,
                         f" PO3{tf_label} {amd_session} ACC↓ {acc_low:.5f}",
                         color=color, fontsize=5, va="center", alpha=0.7)

        # Phase indicator in top-left area
        phase_emoji = {"ACCUMULATION": "📦", "MANIPULATION": "🎭", "DISTRIBUTION": "🚀"}
        phase_text = f"{phase_emoji.get(amd_phase, '?')} PO3 {amd_tf} {amd_session}: {amd_phase} ({amd_dir})"
        y_pos = 0.87 - amd_phases.index(amd) * 0.03
        main_ax.text(0.01, y_pos, phase_text,
                     transform=main_ax.transAxes, color=color,
                     fontsize=6, fontweight="bold", va="top", alpha=0.9)

    # =======================================================================
    # 12. Relevant Anchors — most significant high/low markers
    # =======================================================================
    rel_high = data.get("relevant_high")
    rel_low = data.get("relevant_low")

    if rel_high and rel_high.get("level", 0) > 0:
        rh_level = rel_high["level"]
        if price_lo < rh_level < price_hi:
            main_ax.axhline(rh_level, color=ANCHOR_HIGH_COLOR, linestyle="-",
                            linewidth=1.5, alpha=0.7, zorder=5)
            atr_m = rel_high.get("atr_multiple", 0)
            main_ax.text(n + 0.5, rh_level,
                         f" ⬆REL.H {rh_level:.5f} ({atr_m:.1f}x ATR)",
                         color=ANCHOR_HIGH_COLOR, fontsize=6, va="center",
                         fontweight="bold")

    if rel_low and rel_low.get("level", 0) > 0:
        rl_level = rel_low["level"]
        if price_lo < rl_level < price_hi:
            main_ax.axhline(rl_level, color=ANCHOR_LOW_COLOR, linestyle="-",
                            linewidth=1.5, alpha=0.7, zorder=5)
            atr_m = rel_low.get("atr_multiple", 0)
            main_ax.text(n + 0.5, rl_level,
                         f" ⬇REL.L {rl_level:.5f} ({atr_m:.1f}x ATR)",
                         color=ANCHOR_LOW_COLOR, fontsize=6, va="center",
                         fontweight="bold")

    # =======================================================================
    # Legend
    # =======================================================================
    legend_items = [
        ("■ FVG Zone", FVG_BULL_COLOR),
        ("■ Order Block", OB_BULL_COLOR),
        ("■ Breaker", BREAKER_COLOR),
        ("■ OTE Zone", OTE_BULL_COLOR),
        ("◆ CHoCH", CHOCH_COLOR),
        ("△▽ BOS/Sweep", BOS_BULL_COLOR),
        ("□ Killzone Box", KZ_LONDON_COLOR),
        ("— DWM Pivot", PDH_PDL_COLOR),
        ("□ Silver Bullet", SB_NY_AM_COLOR),
        ("■ PO3 Acc Range", PO3_ACC_COLOR),
        ("— Relevant Anchor", ANCHOR_HIGH_COLOR),
    ]
    for i, (label, color) in enumerate(legend_items):
        main_ax.text(0.99, 0.97 - i * 0.035, label,
                     transform=main_ax.transAxes, color=color,
                     fontsize=6, ha="right", va="top", alpha=0.8)

    save_chart(fig, output_path)


# ===========================================================================
# Dispatcher
# ===========================================================================

def main():
    if len(sys.argv) < 3:
        print("Usage: python3 ict_chart.py <input.json> <output.png>", file=sys.stderr)
        sys.exit(1)

    with open(sys.argv[1], "r") as f:
        data = json.load(f)

    render_ict_chart(data, sys.argv[2])


if __name__ == "__main__":
    main()
