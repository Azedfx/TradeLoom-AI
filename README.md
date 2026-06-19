# TradeLoom AI

AI-powered crypto trading agent that automates the full strategy loop — perception, decision, execution, and risk management — using real-time market data, on-chain sentiment, news, and macro indicators via the Bitget ecosystem.

## Problem

Manual crypto trading is slow, emotional, and hard to scale. Most retail traders lack the tools to synthesize market data, news sentiment, and macro conditions into a disciplined, repeatable strategy. TradeLoom replaces gut-feel trading with an AI-driven pipeline that evaluates every opportunity consistently.

## Strategy Loop (Perception → Decision → Execution → Risk Management)

1. **Perception** — Gathers real-time data from multiple sources: kline technicals (RSI, SMA, trend, volume), Bitget Skill Hub MCP modules (Fear & Greed sentiment index, CryptoPanic news feed, CPI macro indicators), and LLM-powered synthesis (Qwen 3.6+).
2. **Decision** — The `DecisionEngine` scores each signal across 5 dimensions (sentiment +30, technical trend +30, volume/momentum +20, news narrative +20, macro +10) and produces a confidence score out of 100. Thresholds: ≥70 = BUY, 50-69 = WATCHLIST, <50 = NO TRADE.
3. **Execution** — Places market orders via the Bitget API (paper demo or real). Position size follows the 2% risk-per-trade rule (2% of available USDT balance divided by asset price).
4. **Risk Management** — Active background monitor checks every 10s for exit conditions: take profit at +5%, stop loss at -2%, trend reversal to bearish, or RSI overbought (80+).

## Bitget AI Modules Used

- **Sentiment Index** — Fear & Greed index for market mood (`sentiment_index`)
- **News Feed** — CryptoPanic news aggregation per asset (`news_feed`)
- **Macro Indicators** — CPI and broader macro data (`macro_indicators`)
- **LLM Synthesis** — Qwen 3.6+ parses user intent and synthesizes structured market analysis (`/v1/chat/completions`)

## Features

- Natural language strategy creation ("Find a bullish RWA opportunity like ONDO")
- Multi-source market perception (technicals + sentiment + news + macro)
- AI confidence scoring with explainable bullet-point reasoning
- 2% risk-per-trade position sizing
- Background monitoring with automated TP/SL exits
- Demo (paper) and live trading modes
- REST API with JSON responses

## Architecture

```
trade-agent/
├── cmd/api/main.go              # Entrypoint — wires services, starts server & monitor
├── config/env.go                # Env-based configuration
├── internal/
│   ├── agent/
│   │   ├── decision_engine.go   # Scoring engine & human-readable explanations
│   │   ├── monitor.go           # Background loop: entry evaluation & exit management
│   │   ├── strategy_compiler.go # Compiles UserIntent → StrategyRules
│   │   └── symbols.go           # Coin alias resolution
│   ├── api/
│   │   ├── handlers.go          # HTTP handlers: create/execute strategies, dashboard
│   │   └── routes.go            # Gin route registration
│   ├── bitget/
│   │   ├── executor.go          # Order placement & balance via bgc CLI
│   │   ├── klines.go            # Kline fetching, RSI/SMA/trend calculations
│   │   ├── market_data.go       # Ticker data
│   │   └── mcp.go               # MCP client for Bitget Skill Hub
│   ├── llm/
│   │   └── client.go            # LLM client (ParseIntent, AnalyzeMarketWithData)
│   ├── models/models.go         # All data structs
│   └── store/memory.go          # In-memory store for strategies & trades
├── demo.sh                      # End-to-end demo script
└── go.mod
```

## Quick Start

### Prerequisites

- Go 1.26+
- Bitget API credentials (for live trading) or use demo mode
- `bgc` CLI installed and configured (for Bitget order execution)

### Setup

```bash
git clone <repo-url>
cd trade-agent

# Configure environment
cp .env.example .env
# Edit .env with your API keys

# Build
go build -o trade-agent ./cmd/api

# Run
./trade-agent
```

Server starts on `:8040` by default in demo mode.

### Configuration (.env)

| Variable | Default | Description |
|---|---|---|
| `BITGET_API_KEY` | — | Bitget API key |
| `BITGET_SECRET_KEY` | — | Bitget API secret |
| `ALIBABA_CLOVE_KEY` | — | LLM API key |
| `LLM_BASE_URL` | `https://hackathon.bitgetops.com/v1/chat/completions` | LLM endpoint |
| `LLM_MODEL` | `qwen3.6-plus` | LLM model |
| `TRADE_MODE` | `demo` | `demo` or `live` |
| `DEFAULT_SYMBOL` | `BTCUSDT` | Default trading pair |
| `PORT` | `:8040` | Server port |

## API Endpoints

### Create a Strategy

```
POST /api/v1/strategies
Content-Type: application/json

{"prompt": "Find a bullish RWA opportunity like ONDO and execute a 2% risk trade."}
```

Returns the created strategy with ID and compiled rules.

### Execute a Strategy

```
POST /api/v1/strategies/:id/execute
Content-Type: application/json

{"symbol": "ONDOUSDT"}
```

Returns the full evaluation: confidence score, AI explanation, technical indicators, risk management, and order result.

### Dashboard

```
GET /api/v1/dashboard
```

Returns all strategies, trade history, total PnL, and win rate.

## Demo

```bash
./demo.sh
```

Runs the full 5-step flow: prompt → strategy creation → market analysis → AI decision → risk management → execution.

## Backtest

Backtesting is available as an optional supplementary module. Run the backtest script and review the generated report:

```bash
# TODO: backtest script and notebook
```

The goal is to reproduce historical performance using the same decision engine logic.

## License

MIT
