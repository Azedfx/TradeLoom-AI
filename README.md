# TradeLoom AI

AI crypto trading agent that analyzes real-time market data via the **Bitget Agent Hub** (REST API) and scores every trade through **5 Bitget Skill Hub** analysis frameworks — Technical, Sentiment, Market Intel, News, and Macro — using Qwen LLM synthesis.

## Strategy Loop

1. **Perception** — Fetches real klines (`/api/v2/spot/market/candles`), tickers (`/api/v2/spot/market/tickers`), and volume data from Bitget Agent Hub. No simulated or third-party data (no CoinGecko, no Alternative.me).
2. **Decision** — Qwen receives the data alongside condensed **Skill Hub** analysis methodology (5 system prompts distilled from `~/.claude/skills/{technical-analysis,sentiment-analyst,market-intel,news-briefing,macro-analyst}`) and returns `{"decision":"BUY|WATCHLIST|NO_TRADE","confidence":0-100,"signals":[...]}`. Falls back to rule-based scoring if the LLM is slow.
3. **Execution** — Market orders via Bitget API (demo paper mode). Position size = 2% of available USDT balance ÷ asset price.
4. **Risk Management** — Background monitor checks every 10s for exit conditions: TP +5%, SL -2%, trend reversal, or RSI overbought (80+).

## Bitget Integration

- **Agent Hub** (`api.bitget.com`) — klines (RSI, SMA7, SMA25), tickers (price, 24h change, volume), order book. Accessed via `DialTLS` with Cloudflare IP + SNI to bypass local DNS blocks.
- **Skill Hub** (`~/.claude/skills/*/SKILL.md`) — 5 analysis methodologies embedded as Qwen system prompts in `internal/llm/client.go`:
  - `technical-analysis` — RSI thresholds, MACD cross, SMA structure, volume conviction
  - `sentiment-analyst` — Fear/Greed extremes, crowd positioning divergence
  - `market-intel` — Volume flow = capital rotation, market breadth
  - `news-briefing` — Narrative identification, structural vs noise filtering
  - `macro-analyst` — Risk-on/off verdict, macro-crypto correlation
- **LLM** — Qwen at `hackathon.bitgetops.com/v1/chat/completions` with key `kyZl7zSnzbbGyeBL`

## Features

- Natural language strategy creation ("find ai trades", "check BTC")
- 5-dimension Skill Hub analysis via LLM with explainable reasoning
- Real Bitget market data — no fallbacks or simulations
- Background monitoring with automated TP/SL exits
- Dashboard at `/` with live metrics, console, and trade log
- Demo (paper) trading mode

## Architecture

```
trade-agent/
├── cmd/api/main.go              # Entrypoint — wires services, starts server & monitor
├── config/env.go                # Env-based configuration (PORT, LLM, trade mode)
├── internal/
│   ├── agent/
│   │   ├── decision_engine.go   # LLM-first + rule-based fallback scoring
│   │   ├── monitor.go           # Background loop: entry evaluation & exit management
│   │   ├── strategy_compiler.go # Compiles UserIntent → StrategyRules
│   │   └── symbols.go           # Coin alias resolution
│   ├── api/
│   │   ├── handlers.go          # HTTP handlers + HTML dashboard
│   │   └── routes.go            # Gin routes (no /api/v1 prefix)
│   ├── bitget/
│   │   ├── http.go              # Shared HTTP client with DNS-bypassing DialTLS
│   │   ├── executor.go          # Order placement & balance via bgc CLI
│   │   ├── klines.go            # Kline fetching, RSI/SMA/trend calculation
│   │   ├── market_data.go       # Ticker data from Bitget API
│   │   └── mcp.go               # MCP client (sentiment, news, macro via Bitget data)
│   ├── llm/
│   │   └── client.go            # Qwen client: ParseIntent, EvaluateTrade (with Skill Hub prompts), AnalyzeMarketWithData, GenerateMarketBrief
│   ├── models/models.go         # All data structs
│   └── store/memory.go          # In-memory store for strategies & trades
├── templates/
│   └── dashboard.html           # Dashboard UI with console, metrics, opportunities
├── demo.sh
├── export_trades.sh
├── run_backtest.sh
└── go.mod
```

## Quick Start

### Prerequisites

- Go 1.26+
- Bitget API credentials (for live trading) or use demo mode (default)
- `bgc` CLI installed for order execution (Bitget Agent Hub CLI)

### Setup

```bash
git clone <repo-url>
cd trade-agent
cp .env.example .env  # edit with your API keys
go build -o/tmp/trade-agent-api ./cmd/api
/tmp/trade-agent-api
```

Server starts on `:8040` in demo mode. Open http://localhost:8040.

### Configuration (.env)

| Variable | Default | Description |
|---|---|---|
| `LLM_BASE_URL` | `https://hackathon.bitgetops.com/v1/chat/completions` | Qwen endpoint |
| `LLM_MODEL` | `qwen3.6-flash` | LLM model |
| `TRADE_MODE` | `demo` | `demo` or `live` |
| `DEFAULT_SYMBOL` | `BTCUSDT` | Default trading pair |
| `PORT` | `8040` | Server port (no colon — auto-prepended) |

## API Endpoints

All routes are at the root level (no `/api/v1` prefix).

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Dashboard HTML (browser) or JSON (XHR `Accept: application/json`) |
| `POST` | `/strategies` | Create strategy from natural language |
| `POST` | `/strategies/:id/execute` | Execute strategy for a symbol |
| `GET` | `/strategies/:id/status` | Get strategy status |
| `GET` | `/trades` | List all trades |
| `GET` | `/trades/export` | Export trades as CSV |
| `GET` | `/trending` | Top 20 coins by USDT volume (from Bitget) |
| `GET` | `/fear-greed` | Fear & Greed computed from Bitget ticker data |
| `GET` | `/news` | Market news derived from Bitget trending data |
| `GET` | `/brief` | Daily market brief via LLM |
| `POST` | `/quick-execute` | Quick trade analysis: `{"prompt":"check BTC"}` |

## DNS Note

`api.bitget.com` is DNS-blocked on some networks. TradeLoom uses a `DialTLS` workaround in `internal/bitget/http.go` that connects to Cloudflare's IPs directly (`104.18.14.166`, `104.18.15.166`) with `tls.Config{ServerName: "api.bitget.com"}` (SNI). Verified working.

## License

MIT
