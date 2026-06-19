#!/bin/bash
# Export paper trading log from running server
# Usage: bash export_trades.sh

API_BASE="http://localhost:8040/api/v1"

echo "Fetching trade log from $API_BASE/trades/export ..."
curl -s "$API_BASE/trades/export" -o trade_log.csv

COUNT=$(tail -n +2 trade_log.csv | wc -l | tr -d ' ')
echo "Exported $COUNT trades to trade_log.csv"
echo "---"
head -20 trade_log.csv
