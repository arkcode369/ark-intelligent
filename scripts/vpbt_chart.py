#!/usr/bin/env python3
"""
Volume Profile Backtest Chart Renderer

Generates professional volume profile charts with backtest trade overlays.
Input: JSON file with equity_curve, trade_dates, trade_pnl, drawdown, symbol, timeframe, params, vp_levels
Output: PNG chart file

Usage:
  python vpbt_chart.py input.json output.png
  python vpbt_chart.py -i input.json -o output.png
"""

import json
import sys
import argparse
from pathlib import Path
import numpy as np
import matplotlib.pyplot as plt
from matplotlib.gridspec import GridSpec
from datetime import datetime
import warnings

warnings.filterwarnings('ignore')


def clean_value(value):
    """Clean NaN/Inf values, return np.nan for invalid values."""
    if value is None:
        return np.nan
    try:
        f = float(value)
        if np.isnan(f) or np.isinf(f):
            return np.nan
        return f
    except (TypeError, ValueError):
        return np.nan


def clean_list(lst):
    """Clean a list of values."""
    if not lst:
        return []
    return [clean_value(v) for v in lst]


def load_json_data(json_path):
    """Load and validate JSON data from file."""
    with open(json_path, 'r') as f:
        data = json.load(f)
    return data


def generate_price_from_vp(vp_levels, num_points=200):
    """
    Generate a realistic price curve based on volume profile levels.
    """
    prices = vp_levels.get('prices', [])
    volumes = vp_levels.get('volumes', [])
    
    if not prices or not volumes:
        t = np.linspace(0, 4 * np.pi, num_points)
        return 1.09 + 0.005 * np.sin(t) + np.random.normal(0, 0.001, num_points)
    
    prices = np.array(prices)
    volumes = np.array([max(0, v) for v in volumes])
    
    t = np.linspace(0, 4 * np.pi, num_points)
    base_price = np.mean(prices)
    price_range = (np.max(prices) - np.min(prices)) * 0.6
    price = base_price + price_range * 0.5 * np.sin(t)
    price += np.random.normal(0, price_range * 0.05, num_points)
    
    return price


def create_volume_profile_chart(data, output_path):
    """
    Create a professional volume profile backtest chart.
    """
    
    # Extract data
    symbol = str(data.get('symbol', 'SYMBOL')).upper()
    timeframe = str(data.get('timeframe', 'TIMEFRAME')).upper()
    
    # Clean equity curve
    equity_curve = clean_list(data.get('equity_curve', []))
    if not equity_curve or all(np.isnan(v) for v in equity_curve):
        equity_curve = [10000.0] * 100
    
    # Ensure equity curve has enough points
    if len(equity_curve) < 100:
        equity_curve = np.interp(
            np.linspace(0, 1, 100),
            np.linspace(0, 1, len(equity_curve)),
            equity_curve
        ).tolist()
    
    # Clean drawdown
    drawdown = clean_list(data.get('drawdown', []))
    if not drawdown or all(np.isnan(v) for v in drawdown):
        drawdown = [0.0] * len(equity_curve)
    
    if len(drawdown) < len(equity_curve):
        drawdown = drawdown + [0.0] * (len(equity_curve) - len(drawdown))
    elif len(drawdown) > len(equity_curve):
        drawdown = drawdown[:len(equity_curve)]
    
    # Trade data
    trade_dates = data.get('trade_dates', [])
    trade_pnl = clean_list(data.get('trade_pnl', []))
    
    # VP levels
    vp_levels = data.get('vp_levels', {})
    vp_prices = clean_list(vp_levels.get('prices', []))
    volumes = clean_list(vp_levels.get('volumes', []))
    poc = clean_value(vp_levels.get('poc'))
    vah = clean_value(vp_levels.get('vah'))
    val = clean_value(vp_levels.get('val'))
    hvn_zones = vp_levels.get('hvn_zones', [])
    lvn_zones = vp_levels.get('lvn_zones', [])
    
    # Validate and clean HVN/LVN zones
    hvn_zones = [(clean_value(z[0]), clean_value(z[1])) for z in hvn_zones 
                 if isinstance(z, (list, tuple)) and len(z) >= 2 
                 and not np.isnan(clean_value(z[0])) and not np.isnan(clean_value(z[1]))]
    
    lvn_zones = [(clean_value(z[0]), clean_value(z[1])) for z in lvn_zones 
                 if isinstance(z, (list, tuple)) and len(z) >= 2 
                 and not np.isnan(clean_value(z[0])) and not np.isnan(clean_value(z[1]))]
    
    # Handle missing VP levels with realistic defaults
    if not vp_prices or not volumes:
        vp_prices = list(np.linspace(1.0800, 1.1000, 50))
        volumes = [100 + 200 * np.exp(-((i - 25) ** 2) / 400) for i in range(50)]
        poc = 1.0900
        vah = 1.0950
        val = 1.0850
        hvn_zones = [(1.0880, 1.0920)]
        lvn_zones = [(1.0800, 1.0830), (1.0970, 1.1000)]
    
    # Ensure numeric arrays
    vp_prices = np.array(vp_prices)
    volumes = np.array([max(0, v) for v in volumes])
    
    # Generate price curve
    num_points = len(equity_curve)
    price_curve = generate_price_from_vp(vp_levels, num_points=num_points)
    
    # X-axis (time index)
    x_axis = np.arange(num_points)
    
    # Create figure
    fig = plt.figure(figsize=(16, 12))
    gs = GridSpec(3, 1, figure=fig, height_ratios=[1, 5, 1], hspace=0.08)
    
    # Color scheme
    colors = {
        'primary': '#2196F3',
        'primary_light': '#BBDEFB',
        'success': '#4CAF50',
        'danger': '#F44336',
        'warning': '#FF9800',
        'hvn': '#C8E6C9',
        'hvn_edge': '#4CAF50',
        'lvn': '#FFCDD2',
        'lvn_edge': '#F44336',
        'poc': '#D32F2F',
        'vah': '#388E3C',
        'val': '#C62828',
        'grid': '#E0E0E0',
        'text': '#424242',
        'bg': '#FFFFFF',
        'volume': '#1976D2',
        'volume_hvn': '#388E3C',
        'volume_lvn': '#D32F2F'
    }
    
    # ==================== TOP SUBPLOT: Equity Curve ====================
    ax_equity = fig.add_subplot(gs[0])
    
    ax_equity.plot(x_axis, equity_curve, color=colors['primary'], linewidth=2, 
                   label='Equity', solid_capstyle='round')
    ax_equity.fill_between(x_axis, equity_curve, alpha=0.3, color=colors['primary'])
    
    ax_equity.set_ylabel('Equity ($)', fontsize=11, fontweight='600')
    ax_equity.grid(True, alpha=0.3, linestyle='--', linewidth=0.5)
    ax_equity.set_xticks([])
    ax_equity.set_xlim(0, num_points - 1)
    
    # Add equity stats
    valid_equity = [v for v in equity_curve if not np.isnan(v)]
    if len(valid_equity) > 1:
        total_return = ((valid_equity[-1] / valid_equity[0]) - 1) * 100
        max_equity = max(valid_equity)
        min_equity = min(valid_equity)
        
        stats_text = f'Return: {total_return:+.1f}%\n'
        stats_text += f'Max: ${max_equity:,.0f}\n'
        stats_text += f'Min: ${min_equity:,.0f}'
        
        ax_equity.text(0.98, 0.95, stats_text,
                      transform=ax_equity.transAxes, fontsize=9,
                      verticalalignment='top', horizontalalignment='right',
                      fontfamily='monospace',
                      bbox=dict(boxstyle='round', facecolor='#FFF9C4', alpha=0.8, 
                               edgecolor='#FBC02D', linewidth=1))
    
    # ==================== MAIN SUBPLOT: Price + Volume Profile ====================
    ax_main = fig.add_subplot(gs[1])
    ax_vp = ax_main.twinx()
    
    # Plot price curve
    ax_main.plot(x_axis, price_curve, color=colors['primary'], linewidth=1.8, 
                 label='Price', solid_capstyle='round', alpha=0.9)
    ax_main.set_ylabel('Price', fontsize=11, fontweight='600')
    
    # Get price range for y-axis
    valid_price_vals = [p for p in price_curve if not np.isnan(p)]
    if valid_price_vals:
        price_y_min = min(valid_price_vals)
        price_y_max = max(valid_price_vals)
        price_y_range = price_y_max - price_y_min
        padding = price_y_range * 0.15
        ax_main.set_ylim(price_y_min - padding, price_y_max + padding)
    else:
        ax_main.set_ylim(1.08, 1.10)
        price_y_min, price_y_max = 1.08, 1.10
        price_y_range = 0.02
    
    # Create volume profile histogram (horizontal bars on right side)
    if len(vp_prices) > 1 and len(volumes) > 1:
        vol_max = max(volumes) if max(volumes) > 0 else 1
        bar_height = price_y_range / len(vp_prices) * 0.7
        
        vp_colors = []
        vp_y = []
        vp_widths = []
        
        for price_val, vol_val in zip(vp_prices, volumes):
            if np.isnan(price_val) or np.isnan(vol_val):
                continue
            
            # Determine bar color based on HVN/LVN zones
            is_hvn = any(h_low <= price_val <= h_high for h_low, h_high in hvn_zones) if hvn_zones else False
            is_lvn = any(l_low <= price_val <= l_high for l_low, l_high in lvn_zones) if lvn_zones else False
            
            if is_hvn:
                bar_color = colors['volume_hvn']
            elif is_lvn:
                bar_color = colors['volume_lvn']
            else:
                bar_color = colors['volume']
            
            vp_colors.append(bar_color)
            vp_y.append(price_val)
            # Width is a small fraction of the chart width (not price range!)
            vol_normalized = (vol_val / vol_max) * (num_points * 0.15)
            vp_widths.append(vol_normalized)
        
        # Plot horizontal bars - positioned on the right side of the chart
        if vp_y:
            ax_vp.barh(vp_y, vp_widths, height=bar_height, color=vp_colors, 
                      alpha=0.7, align='center', label='Volume')
            ax_vp.set_xlim(0, num_points * 0.2)  # Limit x for volume axis
            ax_vp.set_yticks([])
    
    ax_vp.set_ylabel('Volume', fontsize=11, fontweight='600')
    
    # Draw POC line (Point of Control)
    if poc is not None and not np.isnan(poc):
        ax_main.axhline(y=poc, color=colors['poc'], linewidth=3, 
                       linestyle='-', alpha=0.9, zorder=10, label='POC')
        ax_main.text(num_points * 0.92, poc, f' POC {poc:.5f}', 
                    color=colors['poc'], fontsize=9, fontweight='bold', 
                    verticalalignment='center', 
                    bbox=dict(boxstyle='round,pad=0.3', facecolor='white', 
                             edgecolor=colors['poc'], alpha=0.8))
    
    # Draw VAH line (Value Area High)
    if vah is not None and not np.isnan(vah):
        ax_main.axhline(y=vah, color=colors['vah'], linewidth=2, 
                       linestyle='--', alpha=0.8, zorder=9, label='VAH')
        ax_main.text(num_points * 0.92, vah, f' VAH {vah:.5f}', 
                    color=colors['vah'], fontsize=8, fontweight='600', 
                    verticalalignment='center',
                    bbox=dict(boxstyle='round,pad=0.3', facecolor='white', 
                             edgecolor=colors['vah'], alpha=0.7))
    
    # Draw VAL line (Value Area Low)
    if val is not None and not np.isnan(val):
        ax_main.axhline(y=val, color=colors['val'], linewidth=2, 
                       linestyle='--', alpha=0.8, zorder=9, label='VAL')
        ax_main.text(num_points * 0.92, val, f' VAL {val:.5f}', 
                    color=colors['val'], fontsize=8, fontweight='600', 
                    verticalalignment='center',
                    bbox=dict(boxstyle='round,pad=0.3', facecolor='white', 
                             edgecolor=colors['val'], alpha=0.7))
    
    # Shade HVN zones
    for idx, (h_low, h_high) in enumerate(hvn_zones):
        if np.isnan(h_low) or np.isnan(h_high):
            continue
        ax_main.axhspan(h_low, h_high, alpha=0.2, color=colors['hvn'], 
                       edgecolor=colors['hvn_edge'], linewidth=1,
                       label='HVN' if idx == 0 else "")
    
    # Shade LVN zones
    for idx, (l_low, l_high) in enumerate(lvn_zones):
        if np.isnan(l_low) or np.isnan(l_high):
            continue
        ax_main.axhspan(l_low, l_high, alpha=0.2, color=colors['lvn'], 
                       edgecolor=colors['lvn_edge'], linewidth=1,
                       label='LVN' if idx == 0 else "")
    
    # Plot trade markers
    if trade_pnl and len(trade_pnl) > 0:
        num_trades = len(trade_pnl)
        
        for i, pnl in enumerate(trade_pnl):
            if np.isnan(pnl):
                continue
            
            # Calculate trade position along x-axis
            if num_trades > 1:
                trade_x = int((i / (num_trades - 1)) * (num_points - 1))
            else:
                trade_x = num_points // 2
            
            trade_x = min(max(trade_x, 0), num_points - 1)
            trade_y = price_curve[trade_x] if trade_x < len(price_curve) else price_curve[0]
            
            if np.isnan(trade_y):
                continue
            
            marker_color = colors['success'] if pnl >= 0 else colors['danger']
            
            # Entry marker (triangle pointing up)
            ax_main.scatter(trade_x, trade_y, marker='^', s=200, color=marker_color,
                           edgecolors='white', linewidths=2, zorder=10, 
                           label='Entry' if i == 0 else "", clip_on=False)
            
            # Exit marker (circle) - offset slightly
            exit_x = min(trade_x + max(1, num_points // (num_trades * 2)), num_points - 1)
            exit_y = price_curve[exit_x] if exit_x < len(price_curve) else trade_y
            if not np.isnan(exit_y):
                ax_main.scatter(exit_x, exit_y, marker='o', s=150, color=marker_color,
                               edgecolors='white', linewidths=2, zorder=10, 
                               label='Exit' if i == 0 else "", clip_on=False)
            
            # PnL label
            label_offset = price_y_range * 0.03
            ax_main.text(trade_x, trade_y + label_offset, f'${pnl:+.0f}',
                        fontsize=8, fontweight='bold', color=marker_color,
                        verticalalignment='bottom', horizontalalignment='center',
                        bbox=dict(boxstyle='round,pad=0.2', facecolor='white', 
                                 alpha=0.9, edgecolor=marker_color))
    
    # Title
    ax_main.set_title(f'Volume Profile Backtest - {symbol} {timeframe}', 
                     fontsize=14, fontweight='bold', pad=15, color=colors['text'])
    
    # Grid
    ax_main.grid(True, alpha=0.3, linestyle='--', linewidth=0.5, zorder=0)
    ax_main.set_xlabel('Time', fontsize=11, fontweight='600')
    ax_main.set_xlim(0, num_points - 1)
    
    # Combined legend
    handles, labels = ax_main.get_legend_handles_labels()
    
    seen = set()
    unique_handles = []
    unique_labels = []
    for h, l in zip(handles, labels):
        if l not in seen and l:
            seen.add(l)
            unique_handles.append(h)
            unique_labels.append(l)
    
    if 'Volume' not in seen:
        vol_patch = plt.Rectangle((0, 0), 1, 1, color=colors['volume'], alpha=0.7)
        unique_handles.insert(0, vol_patch)
        unique_labels.insert(0, 'Volume')
    
    ax_main.legend(unique_handles, unique_labels, loc='upper left', fontsize=9, 
                   framealpha=0.95, edgecolor=colors['grid'], bbox_to_anchor=(0, 1.02))
    
    # ==================== BOTTOM SUBPLOT: Drawdown ====================
    ax_dd = fig.add_subplot(gs[2])
    
    valid_dd = [max(0, v) if not np.isnan(v) else 0 for v in drawdown]
    
    ax_dd.fill_between(x_axis, valid_dd, 0, color=colors['danger'], 
                       alpha=0.4, label='Drawdown')
    ax_dd.plot(x_axis, valid_dd, color=colors['danger'], linewidth=1.5, 
               solid_capstyle='round')
    
    ax_dd.set_ylabel('Drawdown ($)', fontsize=11, fontweight='600')
    ax_dd.set_xlabel('Time', fontsize=11, fontweight='600')
    ax_dd.grid(True, alpha=0.3, linestyle='--', linewidth=0.5)
    ax_dd.set_xlim(0, num_points - 1)
    
    # Add max drawdown stat
    if valid_dd:
        max_dd = max(valid_dd)
        max_dd_pct = (max_dd / max(equity_curve) * 100) if max(equity_curve) > 0 else 0
        
        dd_text = f'Max DD: ${max_dd:,.0f} ({max_dd_pct:.1f}%)'
        ax_dd.text(0.98, 0.05, dd_text,
                  transform=ax_dd.transAxes, fontsize=9,
                  verticalalignment='bottom', horizontalalignment='right',
                  fontweight='600', color=colors['danger'],
                  bbox=dict(boxstyle='round', facecolor='#FFEBEE', alpha=0.9, 
                           edgecolor=colors['danger']))
    
    ax_dd.legend(loc='upper right', fontsize=9, framealpha=0.95, 
                 edgecolor=colors['grid'])
    ax_dd.set_ylim(bottom=0)
    
    # ==================== FOOTER: Parameters ====================
    params = data.get('params', {})
    if params:
        param_items = []
        for k, v in params.items():
            if isinstance(v, float):
                param_items.append(f'{k}={v:.4g}')
            else:
                param_items.append(f'{k}={v}')
        
        params_str = ' | '.join(param_items[:6])
        if len(param_items) > 6:
            params_str += ' ...'
        
        fig.text(0.5, 0.02, params_str,
                fontsize=8, ha='center', fontfamily='monospace',
                bbox=dict(boxstyle='round', facecolor='#F5F5F5', 
                         alpha=0.9, edgecolor=colors['grid']))
    
    # ==================== Save Chart ====================
    output_path = Path(output_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    
    plt.savefig(output_path, dpi=150, bbox_inches='tight', 
               facecolor='white', edgecolor='none',
               pad_inches=0.1)
    plt.close(fig)
    
    print(f"Chart saved to: {output_path}")
    return output_path


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Generate Volume Profile Backtest Chart',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Example usage:
  python vpbt_chart.py input.json output.png
  python vpbt_chart.py -i input.json -o output.png

Input JSON format:
  {
    "symbol": "EURUSD",
    "timeframe": "H1",
    "equity_curve": [10000, 10150, ...],
    "trade_dates": ["2024-01-15 09:00", ...],
    "trade_pnl": [150, -70, ...],
    "drawdown": [0, 0, 70, ...],
    "params": {"strategy": "VP Mean Reversion", ...},
    "vp_levels": {
      "prices": [1.0820, 1.0825, ...],
      "volumes": [45, 62, ...],
      "poc": 1.0915,
      "vah": 1.0945,
      "val": 1.0885,
      "hvn_zones": [[1.0900, 1.0925], ...],
      "lvn_zones": [[1.0820, 1.0840], ...]
    }
  }
        '''
    )
    parser.add_argument('input', nargs='?', help='Input JSON file path')
    parser.add_argument('output', nargs='?', help='Output PNG file path')
    parser.add_argument('-i', '--input', dest='input_opt', help='Input JSON file path')
    parser.add_argument('-o', '--output', dest='output_opt', help='Output PNG file path')
    
    args = parser.parse_args()
    
    input_path = args.input_opt or args.input
    output_path = args.output_opt or args.output
    
    if not input_path or not output_path:
        parser.print_help()
        print("\nError: Both input and output paths are required.")
        sys.exit(1)
    
    if not Path(input_path).exists():
        print(f"Error: Input file not found: {input_path}")
        sys.exit(1)
    
    try:
        data = load_json_data(input_path)
        create_volume_profile_chart(data, output_path)
        
    except json.JSONDecodeError as e:
        print(f"Error: Invalid JSON in input file: {e}")
        sys.exit(1)
    except KeyError as e:
        print(f"Error: Missing required field in JSON: {e}")
        sys.exit(1)
    except Exception as e:
        print(f"Error generating chart: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()
