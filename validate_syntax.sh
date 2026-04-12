#!/bin/bash
# Simple syntax check for Go files

echo "Checking for common syntax errors..."

# Check for unmatched braces
for file in internal/adapter/telegram/handler_qbacktest.go internal/adapter/telegram/handler_quant.go internal/adapter/telegram/handler_quant_backtest.go internal/adapter/telegram/keyboard_trading.go; do
    echo "Checking $file..."
    
    # Check for duplicate function declarations
    grep -o "func (h \*Handler) [a-zA-Z0-9_]*" "$file" | sort | uniq -d
done

echo "Done checking."
