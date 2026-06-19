#!/bin/bash
# Run backtest using historical kline data
# Usage: bash run_backtest.sh [SYMBOL]

SYMBOL="${1:-BTCUSDT}"
echo "Running backtest for $SYMBOL ..."
echo ""

go run ./cmd/backtest/ "$SYMBOL" 2>&1 | tee backtest_report.txt

echo ""
echo "Report saved to backtest_report.txt"
