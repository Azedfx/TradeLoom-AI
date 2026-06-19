package backtest

import (
	"fmt"
	"os"
	"time"

	"trade/internal/bitget"
)

type Trade struct {
	EntryTime   time.Time
	ExitTime    time.Time
	EntryPrice  float64
	ExitPrice   float64
	PnLPct      float64
	Direction   string
}

type Report struct {
	Symbol      string
	Trades      []Trade
	WinRate     float64
	TotalReturn float64
	MaxDrawdown float64
}

type Backtest struct {
	md *bitget.MarketData
}

func New(md *bitget.MarketData) *Backtest {
	return &Backtest{md: md}
}

func (bt *Backtest) Run(symbol string, days int) *Report {
	klines, err := bt.md.GetKlines(symbol, "1day", days)
	if err != nil || len(klines) < 30 {
		return &Report{Symbol: symbol}
	}

	var trades []Trade
	inPosition := false
	entryPrice := 0.0
	entryTime := time.Time{}

	for i := 30; i < len(klines); i++ {
		window := klines[:i]
		current := klines[i]

		rsi14, _ := bitget.CalculateRSI(window, 14)
		trend := bitget.DetermineTrend(window)
		change24h := (current.Close - klines[i-1].Close) / klines[i-1].Close

		buySignal := trend == "bullish" && rsi14 > 40 && rsi14 < 70 && change24h > 0

		exitSignal := false
		if inPosition {
			pnlPct := (current.Close - entryPrice) / entryPrice
			if trend == "bearish" {
				exitSignal = true
			} else if rsi14 > 80 {
				exitSignal = true
			} else if pnlPct > 0.05 {
				exitSignal = true
			} else if pnlPct < -0.02 {
				exitSignal = true
			}

			if exitSignal {
				pnl := (current.Close - entryPrice) / entryPrice
				trades = append(trades, Trade{
					EntryTime:  entryTime,
					ExitTime:   time.UnixMilli(current.Timestamp),
					EntryPrice: entryPrice,
					ExitPrice:  current.Close,
					PnLPct:     pnl,
					Direction:  "long",
				})
				inPosition = false
			}
		}

		if buySignal && !inPosition {
			entryPrice = current.Close
			entryTime = time.UnixMilli(current.Timestamp)
			inPosition = true
		}
	}

	if inPosition {
		last := klines[len(klines)-1]
		pnl := (last.Close - entryPrice) / entryPrice
		trades = append(trades, Trade{
			EntryTime:  entryTime,
			ExitTime:   time.UnixMilli(last.Timestamp),
			EntryPrice: entryPrice,
			ExitPrice:  last.Close,
			PnLPct:     pnl,
			Direction:  "long",
		})
	}

	// Calculate metrics
	wins := 0
	totalReturn := 1.0
	maxDrawdown := 0.0
	peak := 1.0

	for _, t := range trades {
		if t.PnLPct > 0 {
			wins++
		}
		totalReturn *= (1 + t.PnLPct)
		if totalReturn > peak {
			peak = totalReturn
		}
		dd := (peak - totalReturn) / peak
		if dd > maxDrawdown {
			maxDrawdown = dd
		}
	}

	winRate := 0.0
	if len(trades) > 0 {
		winRate = float64(wins) / float64(len(trades)) * 100
	}

	return &Report{
		Symbol:      symbol,
		Trades:      trades,
		WinRate:     winRate,
		TotalReturn: (totalReturn - 1) * 100,
		MaxDrawdown: maxDrawdown * 100,
	}
}

func (r *Report) String() string {
	s := fmt.Sprintf("========================================\n")
	s += fmt.Sprintf("TradeLoom AI - Backtest Report\n")
	s += fmt.Sprintf("========================================\n")
	s += fmt.Sprintf("Symbol: %s\n", r.Symbol)
	s += fmt.Sprintf("Generated: %s\n", time.Now().UTC().Format(time.RFC3339))
	s += fmt.Sprintf("\n")
	s += fmt.Sprintf("----------------------------------------\n")
	s += fmt.Sprintf("Results\n")
	s += fmt.Sprintf("----------------------------------------\n")
	s += fmt.Sprintf("Total Trades: %d\n", len(r.Trades))
	if len(r.Trades) > 0 {
		s += fmt.Sprintf("Win Rate: %.1f%% (%d/%d)\n", r.WinRate, int(r.WinRate*float64(len(r.Trades))/100), len(r.Trades))
		s += fmt.Sprintf("Total Return: %+.2f%%\n", r.TotalReturn)
		s += fmt.Sprintf("Max Drawdown: %.2f%%\n", r.MaxDrawdown)
	}
	s += fmt.Sprintf("\n")
	s += fmt.Sprintf("----------------------------------------\n")
	s += fmt.Sprintf("Trade Log\n")
	s += fmt.Sprintf("----------------------------------------\n")
	s += fmt.Sprintf("timestamp,symbol,direction,entry_price,exit_price,pnl_pct\n")
	for _, t := range r.Trades {
		s += fmt.Sprintf("%s,%s,%s,%.2f,%.2f,%+.2f%%\n",
			t.EntryTime.Format("2006-01-02 15:04:05"),
			r.Symbol, t.Direction, t.EntryPrice, t.ExitPrice, t.PnLPct*100)
	}
	s += fmt.Sprintf("\n")
	s += fmt.Sprintf("Note: Backtest uses technical signals only (RSI, trend, momentum).\n")
	s += fmt.Sprintf("Real decisions also include sentiment, news, and macro data.\n")
	return s
}

func (r *Report) Save(path string) error {
	return os.WriteFile(path, []byte(r.String()), 0644)
}
