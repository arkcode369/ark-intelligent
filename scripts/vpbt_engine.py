#!/usr/bin/env python3
"""
Volume Profile Backtest Engine

Professional backtesting engine for Volume Profile trading strategies.
Supports 10 VP modes with comprehensive metrics and grade filtering.

Input: JSON with mode, symbol, timeframe, grade, bars, params
Output: JSON with success, text_output, chart_path, result
"""

import json
import sys
import argparse
import random
import math
from pathlib import Path
from datetime import datetime, timedelta
from typing import Dict, List, Any, Optional, Tuple
import numpy as np
from dataclasses import dataclass, asdict
from enum import Enum

# Try to import chart renderer
try:
    from vpbt_chart import create_volume_profile_chart
    CHART_AVAILABLE = True
except ImportError:
    CHART_AVAILABLE = False


class VPMode(Enum):
    """Volume Profile strategy modes."""
    PROFILE = "profile"       # POC bounce strategy
    VAHVAL = "vahval"         # Value Area breakout/rejection
    HVN = "hvn"               # High Volume Node support/resistance
    LVN = "lvn"               # Low Volume Node breakout
    SESSION = "session"       # Asian/London/NY session split
    SHAPE = "shape"           # P/b/D/B profile classification
    COMPOSITE = "composite"   # Multi-window merged VP
    VWAP = "vwap"             # VWAP + sigma bands strategy
    CONFLUENCE = "confluence" # Multi-TF level overlap
    FULL = "full"             # Complete report with all strategies


class Grade(Enum):
    """Signal quality grades."""
    A = "A"  # High confidence
    B = "B"  # Medium confidence
    C = "C"  # Low confidence


@dataclass
class Trade:
    """Represents a single trade."""
    entry_time: str
    exit_time: str
    entry_price: float
    exit_price: float
    direction: str  # "long" or "short"
    size: float
    pnl: float
    sl: float
    tp: float
    reason: str
    grade: str


@dataclass
class VPLevels:
    """Volume Profile levels."""
    poc: float  # Point of Control
    vah: float  # Value Area High
    val: float  # Value Area Low
    hvn_zones: List[Tuple[float, float]]  # High Volume Node zones
    lvn_zones: List[Tuple[float, float]]  # Low Volume Node zones
    prices: List[float]
    volumes: List[float]


class VolumeProfileCalculator:
    """Calculates Volume Profile levels from price data."""
    
    def __init__(self, prices: List[float], volumes: List[float]):
        self.prices = np.array(prices)
        self.volumes = np.array(volumes)
        
    def calculate_poc(self) -> float:
        """Calculate Point of Control (price with highest volume)."""
        if len(self.volumes) == 0:
            return np.mean(self.prices)
        max_vol_idx = np.argmax(self.volumes)
        return float(self.prices[max_vol_idx])
    
    def calculate_value_area(self, poc: float, confidence: float = 0.7) -> Tuple[float, float]:
        """Calculate Value Area High/Low (70% of volume around POC)."""
        total_volume = np.sum(self.volumes)
        if total_volume == 0:
            return float(np.min(self.prices)), float(np.max(self.prices))
        
        target_volume = total_volume * confidence
        
        # Sort prices by distance from POC
        distances = np.abs(self.prices - poc)
        sorted_indices = np.argsort(distances)
        
        cumulative_volume = 0
        selected_mask = np.zeros(len(self.prices), dtype=bool)
        
        for idx in sorted_indices:
            cumulative_volume += self.volumes[idx]
            selected_mask[idx] = True
            if cumulative_volume >= target_volume:
                break
        
        # Get actual price boundaries from selected prices
        selected_prices = self.prices[selected_mask]
        if len(selected_prices) == 0:
            return float(poc - 0.0050), float(poc + 0.0050)
        
        val_bound = float(selected_prices.min())
        vah_bound = float(selected_prices.max())
        
        # Ensure VAL < POC < VAH
        if val_bound > vah_bound:
            val_bound, vah_bound = vah_bound, val_bound
        
        return val_bound, vah_bound
    
    def calculate_hvn_zones(self, threshold: float = 1.2) -> List[Tuple[float, float]]:
        """Identify High Volume Node zones (volumes > threshold * mean)."""
        mean_vol = np.mean(self.volumes)
        hvn_threshold = mean_vol * threshold
        
        zones = []
        in_zone = False
        zone_start = 0
        
        for i, vol in enumerate(self.volumes):
            if vol > hvn_threshold:
                if not in_zone:
                    zone_start = i
                    in_zone = True
            else:
                if in_zone:
                    zones.append((float(self.prices[zone_start]), float(self.prices[i-1])))
                    in_zone = False
        
        if in_zone:
            zones.append((float(self.prices[zone_start]), float(self.prices[-1])))
        
        return zones
    
    def calculate_lvn_zones(self, threshold: float = 0.5) -> List[Tuple[float, float]]:
        """Identify Low Volume Node zones (volumes < threshold * mean)."""
        mean_vol = np.mean(self.volumes)
        lvn_threshold = mean_vol * threshold
        
        zones = []
        in_zone = False
        zone_start = 0
        
        for i, vol in enumerate(self.volumes):
            if vol < lvn_threshold:
                if not in_zone:
                    zone_start = i
                    in_zone = True
            else:
                if in_zone:
                    zones.append((float(self.prices[zone_start]), float(self.prices[i-1])))
                    in_zone = False
        
        if in_zone:
            zones.append((float(self.prices[zone_start]), float(self.prices[-1])))
        
        return zones
    
    def get_all_levels(self) -> VPLevels:
        """Calculate all VP levels."""
        poc = self.calculate_poc()
        val_bound, vah_bound = self.calculate_value_area(poc)  # Fixed: correct order
        hvn_zones = self.calculate_hvn_zones()
        lvn_zones = self.calculate_lvn_zones()
        
        return VPLevels(
            poc=poc,
            vah=vah_bound,
            val=val_bound,
            hvn_zones=hvn_zones,
            lvn_zones=lvn_zones,
            prices=self.prices.tolist(),
            volumes=self.volumes.tolist()
        )


class BacktestEngine:
    """Main backtesting engine for Volume Profile strategies."""
    
    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self.symbol = config.get('symbol', 'EURUSD')
        self.timeframe = config.get('timeframe', 'H1')
        self.mode = config.get('mode', 'profile')
        self.grade_filter = config.get('grade', 'B')
        self.bars = config.get('bars', 1000)
        self.params = config.get('params', {})
        
        # Trading parameters
        self.initial_capital = self.params.get('initial_capital', 10000)
        self.risk_per_trade = self.params.get('risk_per_trade', 0.02)
        self.stop_loss_pips = self.params.get('stop_loss_pips', 20)
        self.take_profit_pips = self.params.get('take_profit_pips', 30)
        self.pip_value = self.params.get('pip_value', 10)  # USD per pip for standard lot
        
        # Use REAL price data from Go handler if provided, otherwise generate synthetic
        raw_bars = config.get('bars_data', [])
        if raw_bars and len(raw_bars) > 0:
            # Parse real OHLCV data from Go handler
            self.price_data = np.array([bar['close'] for bar in raw_bars])
            self.bar_times = [bar.get('time', '') for bar in raw_bars]
            self.bars = len(raw_bars)
            
            # Handle volume data: use actual volume or create proxy from price range
            raw_volumes = [bar.get('volume', 0) for bar in raw_bars]
            if all(v == 0 or v is None for v in raw_volumes):
                # Create volume proxy from price range (range-based volume)
                # Higher range = higher "volume" proxy
                ranges = np.abs(np.diff(np.concatenate([[self.price_data[0]], self.price_data])))
                self.volume_data = ranges * 100000  # Scale to reasonable volume numbers
                self.volume_data = np.insert(self.volume_data, 0, self.volume_data[0])  # Match length
            else:
                self.volume_data = np.array([v if v else 1000 for v in raw_volumes])
        else:
            # Fallback to synthetic data
            self.price_data = self._generate_price_data()
            self.volume_data = np.ones(len(self.price_data)) * 1000
            self.bar_times = []
        
        # Calculate VP levels from real or synthetic data
        self.vp_levels = self._calculate_vp_levels()
        
    def _generate_price_data(self) -> np.ndarray:
        """Generate synthetic price data with realistic characteristics."""
        random.seed(42)
        np.random.seed(42)
        
        # Base price based on symbol
        base_prices = {
            'EURUSD': 1.0850,
            'GBPUSD': 1.2650,
            'USDJPY': 148.50,
            'AUDUSD': 0.6550,
            'USDCAD': 1.3550,
        }
        base_price = base_prices.get(self.symbol.upper(), 1.0000)
        
        # Generate price with trend and volatility
        n_bars = max(self.bars, 1000)
        daily_vol = 0.008  # 0.8% daily volatility
        
        # Create price series with mean reversion to VP levels
        prices = [base_price]
        for i in range(1, n_bars):
            # Random walk with mean reversion
            drift = 0.0001 * random.choice([-1, 1])
            volatility = np.random.normal(0, daily_vol / np.sqrt(24))  # Hourly vol
            
            # Mean reversion to base price
            reversion = -0.001 * (prices[-1] - base_price) / base_price
            
            new_price = prices[-1] * (1 + drift + volatility + reversion)
            prices.append(new_price)
        
        return np.array(prices)
    
    def _calculate_vp_levels(self) -> VPLevels:
        """Calculate Volume Profile levels from real price data with proper anchoring."""
        if len(self.price_data) == 0:
            # Fallback if no data
            return VPLevels(
                poc=1.0850, vah=1.0900, val=1.0800,
                hvn_zones=[], lvn_zones=[], prices=[], volumes=[]
            )
        
        # Determine anchor window based on timeframe
        anchor_window = self._get_anchor_window()
        
        # Group bars by anchor periods and calculate VP
        if anchor_window > 1 and len(self.bar_times) > 0:
            # Use anchored VP (session/day/week based)
            return self._calculate_anchored_vp(anchor_window)
        else:
            # Use full dataset VP
            return self._calculate_full_vp()
    
    def _get_anchor_window(self) -> int:
        """Determine anchor window size based on timeframe."""
        tf = self.timeframe.lower()
        if tf == 'daily':
            return 1  # Daily anchor
        elif tf in ['12h', '6h', '4h']:
            return 2 if tf == '12h' else (3 if tf == '6h' else 6)  # 12h = 2x 6h, etc
        elif tf == '1h':
            return 24  # Daily anchor for hourly data
        elif tf in ['30m', '15m']:
            return 48  # Daily anchor for intraday
        return 1
    
    def _calculate_full_vp(self) -> VPLevels:
        """Calculate VP over entire dataset with proper binning."""
        if len(self.price_data) == 0:
            return VPLevels(
                poc=1.0850, vah=1.0900, val=1.0800,
                hvn_zones=[], lvn_zones=[], prices=[], volumes=[]
            )
        
        # Create adaptive price bins based on symbol characteristics
        price_range = self.price_data.max() - self.price_data.min()
        
        # Adaptive bin size: more bins for larger ranges, fewer for smaller
        # Target ~100-200 bins for good resolution
        if 'JPY' in self.symbol.upper():
            # JPY pairs: smaller price values
            bin_size = price_range / 150
        elif 'XAU' in self.symbol.upper() or 'XAG' in self.symbol.upper():
            # Commodities: larger price values
            bin_size = price_range / 150
        else:
            # Forex: standard binning
            bin_size = price_range / 150
        
        n_bins = max(50, min(200, int(price_range / bin_size) if bin_size > 0 else 100))
        prices = np.linspace(self.price_data.min(), self.price_data.max(), n_bins)
        
        # Calculate volume at each price level using histogram with weights
        valid_volumes = self.volume_data if len(self.volume_data) == len(self.price_data) else np.ones(len(self.price_data))
        volumes, _ = np.histogram(self.price_data, bins=prices, weights=valid_volumes)
        
        # Ensure we have valid volumes
        if len(volumes) == 0 or np.sum(volumes) == 0:
            volumes = np.ones(n_bins - 1) * 1000
        
        # Use bin centers as price levels
        bin_centers = (prices[:-1] + prices[1:]) / 2
        
        calc = VolumeProfileCalculator(bin_centers.tolist(), volumes.tolist())
        return calc.get_all_levels()
    
    def _calculate_anchored_vp(self, anchor_window: int) -> VPLevels:
        """Calculate anchored VP based on time periods (session/day/week)."""
        from datetime import datetime
        
        if not self.bar_times or len(self.bar_times) == 0:
            # Fallback to full VP if no time data
            return self._calculate_full_vp()
        
        # Group bars by actual time periods
        periods = {}
        
        for i, (price, time_str) in enumerate(zip(self.price_data, self.bar_times)):
            try:
                # Parse ISO format time
                if isinstance(time_str, str):
                    dt = datetime.fromisoformat(time_str.replace('Z', '+00:00').split('+')[0])
                else:
                    continue
                
                # Create time-based key based on timeframe
                if self.timeframe.lower() == 'daily':
                    key = dt.date()
                elif self.timeframe.lower() in ['12h', '6h', '4h']:
                    # Group by 12h/6h/4h sessions
                    hour_bucket = (dt.hour // anchor_window) * anchor_window
                    key = (dt.date(), hour_bucket)
                elif self.timeframe.lower() == '1h':
                    # Daily anchor for hourly
                    key = dt.date()
                elif self.timeframe.lower() in ['30m', '15m']:
                    # Daily anchor for intraday
                    key = dt.date()
                else:
                    key = dt.date()
                
                if key not in periods:
                    periods[key] = {'prices': [], 'volumes': []}
                
                vol = self.volume_data[i] if i < len(self.volume_data) else 1000
                periods[key]['prices'].append(price)
                periods[key]['volumes'].append(vol)
            except Exception as e:
                # Skip invalid time entries
                continue
        
        # Use the most recent complete period for anchored VP
        if periods:
            # Sort keys to get the latest period
            sorted_keys = sorted(periods.keys())
            last_period_key = sorted_keys[-1]
            period_data = periods[last_period_key]
            
            if len(period_data['prices']) > 10:  # Minimum bars required
                prices = np.array(period_data['prices'])
                volumes = np.array(period_data['volumes'])
                
                # Calculate VP for this period
                price_range = prices.max() - prices.min()
                if price_range > 0:
                    n_bins = min(200, max(50, int(price_range * 10000)))
                    bins = np.linspace(prices.min(), prices.max(), n_bins)
                    vols, _ = np.histogram(prices, bins=bins, weights=volumes)
                    
                    if len(vols) > 0 and np.sum(vols) > 0:
                        calc = VolumeProfileCalculator(bins[:-1].tolist(), vols.tolist())
                        return calc.get_all_levels()
        
        # Fallback to full dataset VP
        return self._calculate_full_vp()
    
    def _generate_signal_grade(self, confidence: float) -> str:
        """Generate signal grade based on confidence."""
        if confidence >= 0.8:
            return Grade.A.value
        elif confidence >= 0.6:
            return Grade.B.value
        else:
            return Grade.C.value
    
    def _passes_grade_filter(self, grade: str) -> bool:
        """Check if signal grade passes the filter."""
        grade_values = {'A': 3, 'B': 2, 'C': 1}
        return grade_values.get(grade, 0) >= grade_values.get(self.grade_filter, 0)
    
    def _calculate_pips(self, entry: float, exit_price: float, direction: str) -> float:
        """Calculate pips between two prices."""
        if 'JPY' in self.symbol.upper():
            pip_factor = 100
        else:
            pip_factor = 10000
        
        if direction == 'long':
            diff = (exit_price - entry) * pip_factor
        else:
            diff = (entry - exit_price) * pip_factor
        
        return diff
    
    def _calculate_atr(self, period: int = 14) -> float:
        """Calculate Average True Range for dynamic stop loss/take profit."""
        if len(self.price_data) < period + 1:
            # Fallback to fixed value if not enough data
            return 0.0020 if 'JPY' not in self.symbol.upper() else 0.20
        
        # Calculate true range
        high_low = np.zeros(len(self.price_data))
        # Since we only have close data, estimate high/low from close
        # In production, use actual OHLC data
        range_estimate = np.abs(np.diff(self.price_data))
        range_estimate = np.insert(range_estimate, 0, range_estimate[0])
        
        # Simple ATR calculation
        atr = np.mean(range_estimate[-period:])
        return atr
    
    def _get_dynamic_distance(self, base_pips: float = 15) -> float:
        """Get dynamic distance based on ATR and symbol characteristics."""
        atr = self._calculate_atr()
        
        # Convert ATR to pips
        if 'JPY' in self.symbol.upper():
            atr_pips = atr * 100
        else:
            atr_pips = atr * 10000
        
        # Use ATR-based distance (0.5x ATR as threshold)
        dynamic_distance = atr_pips * 0.5
        
        # Ensure minimum distance
        return max(base_pips, dynamic_distance)
    
    def _execute_trade(self, entry_price: float, direction: str, 
                       sl: float, tp: float, entry_idx: int) -> Optional[Trade]:
        """Execute a trade and return the result."""
        # Simulate price movement to find exit
        exit_idx = min(entry_idx + random.randint(10, 100), len(self.price_data) - 1)
        
        # Find if SL or TP hit first
        price_path = self.price_data[entry_idx:exit_idx + 1]
        
        if direction == 'long':
            sl_hit = np.any(price_path <= sl)
            tp_hit = np.any(price_path >= tp)
        else:
            sl_hit = np.any(price_path >= sl)
            tp_hit = np.any(price_path <= tp)
        
        if sl_hit and not tp_hit:
            exit_price = sl
            pnl = -self.stop_loss_pips * self.pip_value
            exit_time = entry_idx
        elif tp_hit and not sl_hit:
            exit_price = tp
            pnl = self.take_profit_pips * self.pip_value
            exit_time = exit_idx
        else:
            # Close at end
            exit_price = self.price_data[exit_idx]
            pnl = self._calculate_pips(entry_price, exit_price, direction) * self.pip_value
            exit_time = exit_idx
        
        # Calculate size based on risk
        risk_amount = self.initial_capital * self.risk_per_trade
        size = risk_amount / (abs(entry_price - sl) * self.pip_value)
        
        trade = Trade(
            entry_time=str(datetime.now() + timedelta(hours=entry_idx)),
            exit_time=str(datetime.now() + timedelta(hours=exit_time)),
            entry_price=entry_price,
            exit_price=exit_price,
            direction=direction,
            size=size,
            pnl=pnl,
            sl=sl,
            tp=tp,
            reason="",
            grade=""
        )
        
        return trade
    
    def backtest_profile(self) -> List[Trade]:
        """POC bounce strategy - trade reversions at Point of Control."""
        trades = []
        poc = self.vp_levels.poc
        
        # Get dynamic distance based on ATR
        dynamic_pips = self._get_dynamic_distance(base_pips=10)
        
        for i in range(100, len(self.price_data) - 50):
            price = self.price_data[i]
            
            # Check if price is near POC (within dynamic distance)
            distance_pips = abs(self._calculate_pips(price, poc, 'long'))
            
            if distance_pips < dynamic_pips:
                # Determine direction based on price action
                if i > 0:
                    prev_price = self.price_data[i-1]
                    
                    # Dynamic SL/TP based on ATR
                    atr = self._calculate_atr()
                    if 'JPY' in self.symbol.upper():
                        sl_offset = atr * 2  # 2x ATR for SL
                        tp_offset = atr * 3  # 3x ATR for TP
                    else:
                        sl_offset = atr * 2
                        tp_offset = atr * 3
                    
                    if prev_price > price:  # Price coming down to POC
                        direction = 'long'
                        sl = poc - sl_offset
                        tp = poc + tp_offset
                        confidence = 0.75
                    else:  # Price coming up to POC
                        direction = 'short'
                        sl = poc + sl_offset
                        tp = poc - tp_offset
                        confidence = 0.75
                    
                    grade = self._generate_signal_grade(confidence)
                    
                    if self._passes_grade_filter(grade):
                        trade = self._execute_trade(price, direction, sl, tp, i)
                        if trade:
                            trade.reason = f"POC bounce at {poc:.5f}"
                            trade.grade = grade
                            trades.append(trade)
        
        return trades
    
    def backtest_vahval(self) -> List[Trade]:
        """Value Area breakout/rejection strategy."""
        trades = []
        vah = self.vp_levels.vah
        val = self.vp_levels.val
        
        # Get dynamic offset based on ATR
        atr = self._calculate_atr()
        if 'JPY' in self.symbol.upper():
            sl_offset = atr * 1.5
            tp_offset = atr * 3
            rejection_threshold = atr * 0.3
        else:
            sl_offset = atr * 1.5
            tp_offset = atr * 3
            rejection_threshold = atr * 0.3
        
        va_width = abs(vah - val)
        
        for i in range(100, len(self.price_data) - 50):
            price = self.price_data[i]
            
            # Check for breakout above VAH
            if i > 0:
                prev_price = self.price_data[i-1]
                
                # Breakout above VAH
                if prev_price < vah and price >= vah:
                    direction = 'long'
                    sl = vah - sl_offset
                    tp = vah + tp_offset
                    confidence = 0.70
                    reason = f"VAH breakout at {vah:.5f}"
                
                # Rejection at VAH
                elif prev_price > vah and price < vah and abs(price - vah) < rejection_threshold:
                    direction = 'short'
                    sl = vah + sl_offset
                    tp = val - sl_offset
                    confidence = 0.75
                    reason = f"VAH rejection at {vah:.5f}"
                
                # Breakout below VAL
                elif prev_price > val and price <= val:
                    direction = 'short'
                    sl = val + sl_offset
                    tp = val - tp_offset
                    confidence = 0.70
                    reason = f"VAL breakdown at {val:.5f}"
                
                # Rejection at VAL
                elif prev_price < val and price > val and abs(price - val) < rejection_threshold:
                    direction = 'long'
                    sl = val - sl_offset
                    tp = vah + sl_offset
                    confidence = 0.75
                    reason = f"VAL bounce at {val:.5f}"
                
                else:
                    continue
                
                grade = self._generate_signal_grade(confidence)
                
                if self._passes_grade_filter(grade):
                    trade = self._execute_trade(price, direction, sl, tp, i)
                    if trade:
                        trade.reason = reason
                        trade.grade = grade
                        trades.append(trade)
        
        return trades
    
    def backtest_hvn(self) -> List[Trade]:
        """High Volume Node support/resistance strategy."""
        trades = []
        hvn_zones = self.vp_levels.hvn_zones
        
        for i in range(100, len(self.price_data) - 50):
            price = self.price_data[i]
            
            for zone_low, zone_high in hvn_zones:
                # Price approaching HVN zone from below
                if i > 0:
                    prev_price = self.price_data[i-1]
                    
                    if prev_price < zone_low and zone_low <= price <= zone_high:
                        # HVN acts as support
                        direction = 'long'
                        sl = zone_low - 0.0002
                        tp = zone_high + 0.0002
                        confidence = 0.80
                        reason = f"HVN support at {zone_low:.5f}-{zone_high:.5f}"
                        
                        grade = self._generate_signal_grade(confidence)
                        
                        if self._passes_grade_filter(grade):
                            trade = self._execute_trade(price, direction, sl, tp, i)
                            if trade:
                                trade.reason = reason
                                trade.grade = grade
                                trades.append(trade)
                        
                        break
                    
                    # Price approaching HVN zone from above
                    elif prev_price > zone_high and zone_low <= price <= zone_high:
                        # HVN acts as resistance
                        direction = 'short'
                        sl = zone_high + 0.0002
                        tp = zone_low - 0.0002
                        confidence = 0.80
                        reason = f"HVN resistance at {zone_low:.5f}-{zone_high:.5f}"
                        
                        grade = self._generate_signal_grade(confidence)
                        
                        if self._passes_grade_filter(grade):
                            trade = self._execute_trade(price, direction, sl, tp, i)
                            if trade:
                                trade.reason = reason
                                trade.grade = grade
                                trades.append(trade)
                        
                        break
        
        return trades
    
    def backtest_lvn(self) -> List[Trade]:
        """Low Volume Node breakout strategy."""
        trades = []
        lvn_zones = self.vp_levels.lvn_zones
        
        # Get dynamic offset based on ATR
        atr = self._calculate_atr()
        if 'JPY' in self.symbol.upper():
            sl_offset = atr * 1.5
            tp_offset = atr * 3
        else:
            sl_offset = atr * 1.5
            tp_offset = atr * 3
        
        for i in range(100, len(self.price_data) - 50):
            price = self.price_data[i]
            
            for zone_low, zone_high in lvn_zones:
                if i > 0:
                    prev_price = self.price_data[i-1]
                    
                    # Breakout above LVN zone
                    if prev_price < zone_low and price >= zone_high:
                        direction = 'long'
                        sl = zone_low - sl_offset
                        tp = zone_high + tp_offset
                        confidence = 0.65
                        reason = f"LVN breakout above {zone_high:.5f}"
                        
                        grade = self._generate_signal_grade(confidence)
                        
                        if self._passes_grade_filter(grade):
                            trade = self._execute_trade(price, direction, sl, tp, i)
                            if trade:
                                trade.reason = reason
                                trade.grade = grade
                                trades.append(trade)
                        
                        break
                    
                    # Breakout below LVN zone
                    elif prev_price > zone_high and price <= zone_low:
                        direction = 'short'
                        sl = zone_high + sl_offset
                        tp = zone_low - tp_offset
                        confidence = 0.65
                        reason = f"LVN breakdown below {zone_low:.5f}"
                        
                        grade = self._generate_signal_grade(confidence)
                        
                        if self._passes_grade_filter(grade):
                            trade = self._execute_trade(price, direction, sl, tp, i)
                            if trade:
                                trade.reason = reason
                                trade.grade = grade
                                trades.append(trade)
                        
                        break
        
        return trades
    
    def backtest_session(self) -> List[Trade]:
        """Asian/London/NY session split strategy."""
        trades = []
        
        # Use POC as reference for session strategy
        poc = self.vp_levels.poc
        
        # Get dynamic offset based on ATR
        atr = self._calculate_atr()
        if 'JPY' in self.symbol.upper():
            sl_offset = atr * 1.5
            tp_offset = atr * 3
        else:
            sl_offset = atr * 1.5
            tp_offset = atr * 3
        
        for i in range(100, len(self.price_data) - 50):
            # Get actual hour from bar_times
            hour = 0
            if self.bar_times and i < len(self.bar_times):
                try:
                    time_str = self.bar_times[i]
                    if isinstance(time_str, str):
                        from datetime import datetime
                        dt = datetime.fromisoformat(time_str.replace('Z', '+00:00').split('+')[0])
                        hour = dt.hour
                except:
                    hour = i % 24  # Fallback
            else:
                hour = i % 24  # Fallback if no time data
            
            price = self.price_data[i]
            
            # Mean reversion to POC during Asian session (0-8 UTC)
            if 0 <= hour <= 8:
                if price > poc:
                    direction = 'short'
                    sl = price + sl_offset
                    tp = poc
                    confidence = 0.60
                    reason = f"Asian session mean reversion to POC"
                else:
                    direction = 'long'
                    sl = price - sl_offset
                    tp = poc
                    confidence = 0.60
                    reason = f"Asian session mean reversion to POC"
            
            # Breakout strategy during London/NY overlap (13-16 UTC)
            elif 13 <= hour <= 16:
                if price > poc:
                    direction = 'long'
                    sl = poc - sl_offset
                    tp = price + tp_offset
                    confidence = 0.65
                    reason = f"London/NY overlap breakout"
                else:
                    direction = 'short'
                    sl = poc + sl_offset
                    tp = price - tp_offset
                    confidence = 0.65
                    reason = f"London/NY overlap breakdown"
            
            # Mean reversion during NY session (16-22 UTC)
            elif 16 < hour <= 22:
                if price > poc:
                    direction = 'short'
                    sl = price + sl_offset
                    tp = poc
                    confidence = 0.60
                    reason = f"NY session mean reversion"
                else:
                    direction = 'long'
                    sl = price - sl_offset
                    tp = poc
                    confidence = 0.60
                    reason = f"NY session mean reversion"
            
            else:
                continue  # Skip other hours
            
            grade = self._generate_signal_grade(confidence)
            
            if self._passes_grade_filter(grade):
                trade = self._execute_trade(price, direction, sl, tp, i)
                if trade:
                    trade.reason = reason
                    trade.grade = grade
                    trades.append(trade)
        
        return trades
    
    def backtest_shape(self) -> List[Trade]:
        """P/b/D/B profile classification strategy."""
        trades = []
        
        # Get dynamic offset based on ATR
        atr = self._calculate_atr()
        if 'JPY' in self.symbol.upper():
            sl_offset = atr * 1.5
            tp_offset = atr * 3
        else:
            sl_offset = atr * 1.5
            tp_offset = atr * 3
        
        # Analyze price distribution shape
        price_range = self.price_data.max() - self.price_data.min()
        if price_range == 0:
            return trades
        median_price = np.median(self.price_data)
        mean_price = np.mean(self.price_data)
        
        # Determine profile shape
        skew = (mean_price - median_price) / price_range
        
        if abs(skew) < 0.1:
            shape = 'B'  # Balanced
        elif skew > 0.1:
            shape = 'P'  # P-shaped (bullish)
        else:
            shape = 'b'  # b-shaped (bearish)
        
        poc = self.vp_levels.poc
        
        for i in range(100, len(self.price_data) - 50):
            price = self.price_data[i]
            
            if shape == 'B':
                # Balanced profile - mean reversion
                if price > poc:
                    direction = 'short'
                    sl = price + sl_offset
                    tp = poc
                    confidence = 0.70
                    reason = "Balanced profile mean reversion"
                else:
                    direction = 'long'
                    sl = price - sl_offset
                    tp = poc
                    confidence = 0.70
                    reason = "Balanced profile mean reversion"
            
            elif shape == 'P':
                # P-shaped - trend following
                if price > poc:
                    direction = 'long'
                    sl = poc - sl_offset
                    tp = price + tp_offset
                    confidence = 0.65
                    reason = "P-shaped profile continuation"
                else:
                    direction = 'short'
                    sl = poc + sl_offset
                    tp = poc - tp_offset
                    confidence = 0.60
                    reason = "P-shaped profile pullback"
            
            else:  # b-shaped
                # b-shaped - opposite
                if price > poc:
                    direction = 'short'
                    sl = poc + sl_offset
                    tp = poc - tp_offset
                    confidence = 0.60
                    reason = "b-shaped profile pullback"
                else:
                    direction = 'long'
                    sl = poc - sl_offset
                    tp = price + tp_offset
                    confidence = 0.65
                    reason = "b-shaped profile continuation"
            
            grade = self._generate_signal_grade(confidence)
            
            if self._passes_grade_filter(grade):
                trade = self._execute_trade(price, direction, sl, tp, i)
                if trade:
                    trade.reason = reason
                    trade.grade = grade
                    trades.append(trade)
        
        return trades
    
    def backtest_composite(self) -> List[Trade]:
        """Multi-window merged VP strategy."""
        trades = []
        
        # Get dynamic offset based on ATR
        atr = self._calculate_atr()
        if 'JPY' in self.symbol.upper():
            sl_offset = atr * 1.5
            tp_offset = atr * 3
            confluence_threshold = atr * 0.5
        else:
            sl_offset = atr * 1.5
            tp_offset = atr * 3
            confluence_threshold = atr * 0.5
        
        # Simulate multiple timeframes
        timeframes = [
            (self.bars // 4, 'M15'),
            (self.bars // 2, 'H1'),
            (self.bars, 'H4')
        ]
        
        # Get confluence zones where multiple TFs align
        confluence_zones = []
        
        for tf_bars, tf_name in timeframes:
            tf_prices = self.price_data[:tf_bars]
            tf_mid = np.median(tf_prices)
            confluence_zones.append(tf_mid)
        
        # Find overlapping zones (within threshold)
        unique_zones = []
        for zone in sorted(confluence_zones):
            if not unique_zones or abs(zone - unique_zones[-1]) > confluence_threshold:
                unique_zones.append(zone)
        
        poc = self.vp_levels.poc
        
        for i in range(100, len(self.price_data) - 50):
            price = self.price_data[i]
            
            # Check if price is near confluence zone
            for zone in unique_zones:
                if abs(price - zone) < confluence_threshold:
                    # High confidence at confluence
                    if price > poc:
                        direction = 'short'
                        sl = zone + sl_offset
                        tp = zone - tp_offset
                    else:
                        direction = 'long'
                        sl = zone - sl_offset
                        tp = zone + tp_offset
                    
                    confidence = 0.85  # Higher confidence at confluence
                    reason = f"Multi-TF confluence at {zone:.5f}"
                    
                    grade = self._generate_signal_grade(confidence)
                    
                    if self._passes_grade_filter(grade):
                        trade = self._execute_trade(price, direction, sl, tp, i)
                        if trade:
                            trade.reason = reason
                            trade.grade = grade
                            trades.append(trade)
                    
                    break
        
        return trades
    
    def backtest_vwap(self) -> List[Trade]:
        """VWAP + sigma bands strategy."""
        trades = []
        
        # Get dynamic offset based on ATR
        atr = self._calculate_atr()
        if 'JPY' in self.symbol.upper():
            sl_offset = atr * 1.5
        else:
            sl_offset = atr * 1.5
        
        # Calculate VWAP (simplified - using cumulative average)
        vwap = np.cumsum(self.price_data * np.arange(1, len(self.price_data) + 1)) / np.cumsum(np.arange(1, len(self.price_data) + 1))
        
        # Calculate standard deviation
        std = np.std(self.price_data)
        
        # Sigma bands
        upper_band = vwap + 2 * std
        lower_band = vwap - 2 * std
        
        for i in range(100, len(self.price_data) - 50):
            price = self.price_data[i]
            current_vwap = vwap[i] if i < len(vwap) else np.mean(self.price_data[:i])
            current_upper = upper_band[i] if i < len(upper_band) else current_vwap + 2 * std
            current_lower = lower_band[i] if i < len(lower_band) else current_vwap - 2 * std
            
            # Mean reversion at sigma bands
            if price >= current_upper:
                direction = 'short'
                sl = current_upper + sl_offset
                tp = current_vwap
                confidence = 0.75
                reason = f"VWAP +2σ rejection at {current_upper:.5f}"
            
            elif price <= current_lower:
                direction = 'long'
                sl = current_lower - sl_offset
                tp = current_vwap
                confidence = 0.75
                reason = f"VWAP -2σ bounce at {current_lower:.5f}"
            
            else:
                continue
            
            grade = self._generate_signal_grade(confidence)
            
            if self._passes_grade_filter(grade):
                trade = self._execute_trade(price, direction, sl, tp, i)
                if trade:
                    trade.reason = reason
                    trade.grade = grade
                    trades.append(trade)
        
        return trades
    
    def backtest_confluence(self) -> List[Trade]:
        """Multi-TF level overlap strategy."""
        trades = []
        
        # Get dynamic offset based on ATR
        atr = self._calculate_atr()
        if 'JPY' in self.symbol.upper():
            sl_offset = atr * 1.5
            tp_offset = atr * 3
            confluence_threshold = atr * 0.3
        else:
            sl_offset = atr * 1.5
            tp_offset = atr * 3
            confluence_threshold = atr * 0.3
        
        # Get levels from multiple sources
        poc = self.vp_levels.poc
        vah = self.vp_levels.vah
        val = self.vp_levels.val
        
        # Simulate additional TF levels
        tf1_level = np.percentile(self.price_data, 25)
        tf2_level = np.percentile(self.price_data, 50)
        tf3_level = np.percentile(self.price_data, 75)
        
        all_levels = [poc, vah, val, tf1_level, tf2_level, tf3_level]
        
        # Find confluence (levels within threshold of each other)
        confluence_points = []
        for i, level1 in enumerate(all_levels):
            count = 1
            for j, level2 in enumerate(all_levels):
                if i != j and abs(level1 - level2) < confluence_threshold:
                    count += 1
            if count >= 3:
                confluence_points.append(level1)
        
        # Remove duplicates
        confluence_points = list(set([round(p, 5) for p in confluence_points]))
        
        for i in range(100, len(self.price_data) - 50):
            price = self.price_data[i]
            
            for confluence in confluence_points:
                if abs(price - confluence) < confluence_threshold:
                    if price > poc:
                        direction = 'short'
                        sl = confluence + sl_offset
                        tp = confluence - tp_offset
                    else:
                        direction = 'long'
                        sl = confluence - sl_offset
                        tp = confluence + tp_offset
                    
                    confidence = 0.90  # Highest confidence at confluence
                    reason = f"Multi-level confluence at {confluence:.5f}"
                    
                    grade = self._generate_signal_grade(confidence)
                    
                    if self._passes_grade_filter(grade):
                        trade = self._execute_trade(price, direction, sl, tp, i)
                        if trade:
                            trade.reason = reason
                            trade.grade = grade
                            trades.append(trade)
                    
                    break
        
        return trades
    
    def backtest_full(self) -> Dict[str, List[Trade]]:
        """Run all strategies and return combined results."""
        results = {}
        
        strategies = [
            ('profile', self.backtest_profile),
            ('vahval', self.backtest_vahval),
            ('hvn', self.backtest_hvn),
            ('lvn', self.backtest_lvn),
            ('session', self.backtest_session),
            ('shape', self.backtest_shape),
            ('composite', self.backtest_composite),
            ('vwap', self.backtest_vwap),
            ('confluence', self.backtest_confluence),
        ]
        
        for name, func in strategies:
            results[name] = func()
        
        return results
    
    def calculate_metrics(self, trades: List[Trade]) -> Dict[str, Any]:
        """Calculate backtest performance metrics."""
        if not trades:
            return {
                'total_trades': 0,
                'win_rate': 0,
                'profit_factor': 0,
                'sharpe_ratio': 0,
                'max_drawdown': 0,
                'expected_value': 0,
                'avg_win': 0,
                'avg_loss': 0,
                'total_pnl': 0
            }
        
        total_trades = len(trades)
        wins = [t for t in trades if t.pnl > 0]
        losses = [t for t in trades if t.pnl <= 0]
        
        win_rate = len(wins) / total_trades if total_trades > 0 else 0
        
        total_wins = sum(t.pnl for t in wins)
        total_losses = abs(sum(t.pnl for t in losses))
        
        profit_factor = total_wins / total_losses if total_losses > 0 else float('inf')
        
        total_pnl = sum(t.pnl for t in trades)
        expected_value = total_pnl / total_trades if total_trades > 0 else 0
        
        avg_win = total_wins / len(wins) if wins else 0
        avg_loss = total_losses / len(losses) if losses else 0
        
        # Calculate equity curve for Sharpe and drawdown
        equity_curve = [self.initial_capital]
        for trade in trades:
            equity_curve.append(equity_curve[-1] + trade.pnl)
        
        # Sharpe ratio (annualized, assuming 24 trades per day)
        returns = [(equity_curve[i] - equity_curve[i-1]) / equity_curve[i-1] 
                   for i in range(1, len(equity_curve))]
        if returns:
            avg_return = np.mean(returns)
            std_return = np.std(returns)
            sharpe_ratio = (avg_return / std_return * np.sqrt(24 * 365)) if std_return > 0 else 0
        else:
            sharpe_ratio = 0
        
        # Max drawdown
        peak = equity_curve[0]
        max_drawdown = 0
        for equity in equity_curve:
            if equity > peak:
                peak = equity
            drawdown = (peak - equity) / peak
            if drawdown > max_drawdown:
                max_drawdown = drawdown
        
        return {
            'total_trades': total_trades,
            'win_rate': round(win_rate, 4),
            'profit_factor': round(profit_factor, 4) if profit_factor != float('inf') else 999.9999,
            'sharpe_ratio': round(sharpe_ratio, 4),
            'max_drawdown': round(max_drawdown, 4),
            'expected_value': round(expected_value, 2),
            'avg_win': round(avg_win, 2),
            'avg_loss': round(avg_loss, 2),
            'total_pnl': round(total_pnl, 2),
            'win_count': len(wins),
            'loss_count': len(losses)
        }
    
    def generate_equity_curve(self, trades: List[Trade]) -> List[float]:
        """Generate equity curve from trades."""
        equity = [self.initial_capital]
        for trade in trades:
            equity.append(equity[-1] + trade.pnl)
        return equity
    
    def generate_drawdown(self, equity_curve: List[float]) -> List[float]:
        """Calculate drawdown from equity curve."""
        drawdown = []
        peak = equity_curve[0]
        
        for equity in equity_curve:
            if equity > peak:
                peak = equity
            dd = peak - equity
            drawdown.append(dd)
        
        return drawdown
    
    def run(self) -> Dict[str, Any]:
        """Run the backtest and return results."""
        mode = self.mode.lower()
        
        # Run appropriate strategy
        if mode == 'full':
            all_trades = self.backtest_full()
            # Flatten all trades for combined metrics
            flat_trades = []
            for strategy_trades in all_trades.values():
                flat_trades.extend(strategy_trades)
            trades = flat_trades
            strategy_results = {k: len(v) for k, v in all_trades.items()}
        else:
            strategy_map = {
                'profile': self.backtest_profile,
                'vahval': self.backtest_vahval,
                'hvn': self.backtest_hvn,
                'lvn': self.backtest_lvn,
                'session': self.backtest_session,
                'shape': self.backtest_shape,
                'composite': self.backtest_composite,
                'vwap': self.backtest_vwap,
                'confluence': self.backtest_confluence,
            }
            
            if mode in strategy_map:
                trades = strategy_map[mode]()
                strategy_results = {mode: len(trades)}
            else:
                return {
                    'success': False,
                    'text_output': f"Unknown mode: {mode}",
                    'chart_path': None,
                    'result': None
                }
        
        # Calculate metrics
        metrics = self.calculate_metrics(trades)
        
        # Generate equity curve and drawdown
        equity_curve = self.generate_equity_curve(trades)
        drawdown = self.generate_drawdown(equity_curve)
        
        # Generate chart
        chart_path = None
        if CHART_AVAILABLE and trades:
            chart_data = {
                'symbol': self.symbol,
                'timeframe': self.timeframe,
                'equity_curve': equity_curve,
                'trade_dates': [t.entry_time for t in trades[-24:]],  # Last 24 trades
                'trade_pnl': [t.pnl for t in trades[-24:]],
                'drawdown': drawdown[-len(equity_curve):] if len(drawdown) >= len(equity_curve) else drawdown,
                'params': self.params,
                'vp_levels': {
                    'prices': self.vp_levels.prices,
                    'volumes': self.vp_levels.volumes,
                    'poc': self.vp_levels.poc,
                    'vah': self.vp_levels.vah,
                    'val': self.vp_levels.val,
                    'hvn_zones': self.vp_levels.hvn_zones,
                    'lvn_zones': self.vp_levels.lvn_zones
                }
            }
            
            output_path = f"/home/node/.openclaw/workspace/tmp/vpbt_{self.symbol}_{self.mode}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.png"
            try:
                create_volume_profile_chart(chart_data, output_path)
                chart_path = output_path
            except Exception as e:
                chart_path = None
        
        # Format text output
        text_output = self._format_report(mode, trades, metrics, strategy_results)
        
        # Prepare result
        result = {
            'trades': [asdict(t) for t in trades[:50]],  # Limit to 50 trades
            'metrics': metrics,
            'vp_levels': {
                'poc': self.vp_levels.poc,
                'vah': self.vp_levels.vah,
                'val': self.vp_levels.val,
                'hvn_zones': self.vp_levels.hvn_zones,
                'lvn_zones': self.vp_levels.lvn_zones
            }
        }
        
        if mode == 'full':
            result['strategy_breakdown'] = strategy_results
        
        return {
            'success': True,
            'text_output': text_output,
            'chart_path': chart_path,
            'result': result
        }
    
    def _format_report(self, mode: str, trades: List[Trade], 
                       metrics: Dict, strategy_results: Dict) -> str:
        """Format a human-readable backtest report."""
        lines = []
        lines.append("=" * 60)
        lines.append(f"VOLUME PROFILE BACKTEST REPORT")
        lines.append("=" * 60)
        lines.append(f"Symbol: {self.symbol}")
        lines.append(f"Timeframe: {self.timeframe}")
        lines.append(f"Mode: {mode.upper()}")
        lines.append(f"Grade Filter: {self.grade_filter}")
        lines.append(f"Initial Capital: ${self.initial_capital:,.0f}")
        lines.append("")
        
        if mode == 'full':
            lines.append("STRATEGY BREAKDOWN:")
            lines.append("-" * 40)
            for strategy, count in strategy_results.items():
                lines.append(f"  {strategy.upper()}: {count} trades")
            lines.append("")
        
        lines.append("PERFORMANCE METRICS:")
        lines.append("-" * 40)
        lines.append(f"  Total Trades: {metrics['total_trades']}")
        lines.append(f"  Win Rate: {metrics['win_rate']*100:.1f}%")
        lines.append(f"  Profit Factor: {metrics['profit_factor']:.2f}")
        lines.append(f"  Sharpe Ratio: {metrics['sharpe_ratio']:.2f}")
        lines.append(f"  Max Drawdown: {metrics['max_drawdown']*100:.2f}%")
        lines.append(f"  Expected Value: ${metrics['expected_value']:.2f}")
        lines.append(f"  Avg Win: ${metrics['avg_win']:.2f}")
        lines.append(f"  Avg Loss: ${metrics['avg_loss']:.2f}")
        lines.append(f"  Total PnL: ${metrics['total_pnl']:.2f}")
        lines.append("")
        
        lines.append("VOLUME PROFILE LEVELS:")
        lines.append("-" * 40)
        lines.append(f"  POC: {self.vp_levels.poc:.5f}")
        lines.append(f"  VAH: {self.vp_levels.vah:.5f}")
        lines.append(f"  VAL: {self.vp_levels.val:.5f}")
        lines.append(f"  HVN Zones: {len(self.vp_levels.hvn_zones)}")
        lines.append(f"  LVN Zones: {len(self.vp_levels.lvn_zones)}")
        lines.append("")
        
        if trades:
            lines.append("RECENT TRADES (last 10):")
            lines.append("-" * 40)
            for trade in trades[-10:]:
                direction_symbol = "↑" if trade.direction == "long" else "↓"
                pnl_symbol = "+" if trade.pnl >= 0 else ""
                lines.append(f"  {direction_symbol} {trade.grade} | "
                           f"Entry: {trade.entry_price:.5f} | "
                           f"Exit: {trade.exit_price:.5f} | "
                           f"PnL: {pnl_symbol}${trade.pnl:.2f} | "
                           f"{trade.reason[:30]}")
        
        lines.append("")
        lines.append("=" * 60)
        lines.append(f"Report generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        lines.append("=" * 60)
        
        return "\n".join(lines)


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Volume Profile Backtest Engine',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Example usage:
  # Read from JSON file
  python vpbt_engine.py input.json
  
  # Read from stdin
  echo '{"mode": "profile", "symbol": "EURUSD", ...}' | python vpbt_engine.py
  
  # Output to file
  python vpbt_engine.py input.json > output.json
        '''
    )
    parser.add_argument('input', nargs='?', help='Input JSON file path (optional, reads from stdin if not provided)')
    parser.add_argument('--output', '-o', help='Output JSON file path (optional, prints to stdout if not provided)')
    
    args = parser.parse_args()
    
    # Load input data
    try:
        if args.input:
            with open(args.input, 'r') as f:
                config = json.load(f)
        else:
            config = json.load(sys.stdin)
    except json.JSONDecodeError as e:
        result = {
            'success': False,
            'text_output': f"Invalid JSON input: {e}",
            'chart_path': None,
            'result': None
        }
        output = json.dumps(result, indent=2)
        if args.output:
            with open(args.output, 'w') as f:
                f.write(output)
        else:
            print(output)
        sys.exit(1)
    
    # Run backtest
    try:
        engine = BacktestEngine(config)
        result = engine.run()
    except Exception as e:
        result = {
            'success': False,
            'text_output': f"Backtest error: {str(e)}",
            'chart_path': None,
            'result': None
        }
    
    # Output result
    output = json.dumps(result, indent=2)
    if args.output:
        with open(args.output, 'w') as f:
            f.write(output)
    else:
        print(output)


if __name__ == '__main__':
    main()
