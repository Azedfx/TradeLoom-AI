#!/bin/bash

# TradeLoom AI - MVP Demo Script
# Requirements: curl, jq

API_BASE="http://localhost:8040/api/v1"

echo "--------------------------------------------------"
echo "🚀 TradeLoom AI: Starting MVP Flow Demo"
echo "--------------------------------------------------"

# Step 1: Send a prompt
echo "Step 1: Sending user prompt..."
PROMPT="Find a bullish RWA opportunity like ONDO and execute a 2% risk trade."
echo "Prompt: '$PROMPT'"

STRATEGY=$(curl -s --connect-timeout 5 -X POST "$API_BASE/strategies" \
  -H "Content-Type: application/json" \
  -d "{\"prompt\": \"$PROMPT\"}")

STRATEGY_ID=$(echo "$STRATEGY" | jq -r '.id')
SYMBOL=$(echo "$STRATEGY" | jq -r '.target_symbol')

if [ "$STRATEGY_ID" == "null" ] || [ -z "$STRATEGY_ID" ]; then
    echo "❌ Error creating strategy:"
    echo "$STRATEGY"
    exit 1
fi

echo "✅ Strategy Created!"
echo "ID: $STRATEGY_ID"
echo "Target Coin: $SYMBOL"
echo ""

# Step 2: Execute the strategy
echo "Step 2: AI Analyzing Market & Managing Risk..."
echo "(This calls Bitget Skill Hub for Sentiment, News, and Macro data)"
echo "--------------------------------------------------"

EXECUTION=$(curl -s --connect-timeout 10 -X POST "$API_BASE/strategies/$STRATEGY_ID/execute" \
  -H "Content-Type: application/json" \
  -d "{\"symbol\": \"$SYMBOL\"}")

# Check if server returned a valid JSON response
if ! echo "$EXECUTION" | jq -e . >/dev/null 2>&1; then
    echo "❌ Server Error during execution (Non-JSON response):"
    echo "$EXECUTION"
    exit 1
fi

# Step 3: Show the AI's Explanation
echo "Step 3: AI Decision & Explanation"
echo "--------------------------------------------------"
echo "$EXECUTION" | jq -r '.explain // "No explanation available"'

# Step 4: Show Risk Management
echo ""
echo "Step 4: Risk Management Details"
echo "--------------------------------------------------"
RM_NOTE=$(echo "$EXECUTION" | jq -r '.risk_management.note // empty')
if [ -n "$RM_NOTE" ]; then
    echo "$EXECUTION" | jq -r '.risk_management | "Capital: $\(.capital // 0)\nMax Risk (2%): $\(.max_risk // 0)\nCalculated Size: \(.size // 0) units\nNote: \(.note)"'
else
    echo "$EXECUTION" | jq -r '.risk_management | "Capital: $\(.capital // 0)\nMax Risk (2%): $\(.max_risk // 0)\nCalculated Size: \(.size // "0") units"'
fi

# Step 5: Show Order Status
echo ""
echo "Step 5: Execution Status"
echo "--------------------------------------------------"
ORDER_STATUS=$(echo "$EXECUTION" | jq -r '.order.status // "NO_TRADE"')
if [ "$ORDER_STATUS" == "paper_trade" ]; then
    echo "💎 Paper Trade Executed Successfully!"
    echo "Symbol: $SYMBOL"
    echo "Action: BUY"
elif [ "$ORDER_STATUS" == "NO_TRADE" ]; then
    echo "✋ Decision: NO TRADE (Confidence too low or conditions not met)"
else
    echo "❌ Execution Result: $ORDER_STATUS"
fi

echo ""
echo "--------------------------------------------------"
echo "📈 TradeLoom AI is now monitoring your position..."
echo "Background Monitor: ACTIVE (Check logs for auto-exit updates)"
echo "--------------------------------------------------"
