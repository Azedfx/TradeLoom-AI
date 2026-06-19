package main

import (
	"fmt"
	"log"
	"time"

	"trade/internal/bitget"
)

type simulatedTrade struct {
	EntryTime   time.Time
	ExitTime    time.Time
	EntryPrice  float64
	ExitPrice   float64
	PnLPct      float64
	Direction   string
}

func main() {
	symbol := "BTCUSDT"
	if len(symbol) == 0 {
		symbol = "BTCUSDT"
	}

	fmt.Println("========================================")
	fmt.Println("TradeLoom AI - Backtest Report")
	fmt.Println("========================================")
	fmt.Printf("Symbol: %s\n", symbol)
	fmt.Printf("Generated: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Println()

	md := &bitget.MarketData{}
	klines, err := md.GetKlines(symbol, "1day", 90)
	if err != nil {
		log.Fatalf("failed to fetch klines: %v", err)
	}

	if len(klines) < 30 {
		log.Fatalf("not enough kline data: got %d, need at least 30", len(klines))
	}

	fmt.Printf("Date range: %s → %s (%d candles)\n",
		time.UnixMilli(klines[0].Timestamp).Format("2006-01-02"),
		time.UnixMilli(klines[len(klines)-1].Timestamp).Format("2006-01-02"),
		len(klines))
	fmt.Println()

	var trades []simulatedTrade
	inPosition := false
	entryPrice := 0.0
	entryTime := time.Time{}

	for i := 30; i < len(klines); i++ {
		window := klines[:i]
		current := klines[i]

		rsi14, _ := bitget.CalculateRSI(window, 14)
		trend := bitget.DetermineTrend(window)
		change24h := (current.Close - klines[i-1].Close) / klines[i-1].Close

		// Simplified entry: bullish trend + RSI between 40-70 + positive momentum
		buySignal := trend == "bullish" && rsi14 > 40 && rsi14 < 70 && change24h > 0

		// Exit: bearish trend, RSI overbought, or -2% stop loss / +5% take profit
		exitSignal := false
		exitReason := ""
		if inPosition {
			pnlPct := (current.Close - entryPrice) / entryPrice
			if trend == "bearish" {
				exitSignal = true
				exitReason = "trend bearish"
			} else if rsi14 > 80 {
				exitSignal = true
				exitReason = "RSI overbought"
			} else if pnlPct > 0.05 {
				exitSignal = true
				exitReason = "take profit +5%"
			} else if pnlPct < -0.02 {
				exitSignal = true
				exitReason = "stop loss -2%"
			}

			if exitSignal {
				pnl := (current.Close - entryPrice) / entryPrice
				trades = append(trades, simulatedTrade{
					EntryTime:  entryTime,
					ExitTime:   time.UnixMilli(current.Timestamp),
					EntryPrice: entryPrice,
					ExitPrice:  current.Close,
					PnLPct:     pnl,
					Direction:  "long",
				})
				inPosition = false
				fmt.Printf("  EXIT  %s @ %.2f | %s | PnL: %+.2f%%\n",
					time.UnixMilli(current.Timestamp).Format("2006-01-02"),
					current.Close, exitReason, pnl*100)
			}
		}

		if buySignal && !inPosition {
			entryPrice = current.Close
			entryTime = time.UnixMilli(current.Timestamp)
			inPosition = true
			fmt.Printf("  ENTER %s @ %.2f | RSI: %.0f | Trend: %s\n",
				entryTime.Format("2006-01-02"), entryPrice, rsi14, trend)
		}
	}

	// Close any open position at last price
	if inPosition {
		last := klines[len(klines)-1]
		pnl := (last.Close - entryPrice) / entryPrice
		trades = append(trades, simulatedTrade{
			EntryTime:  entryTime,
			ExitTime:   time.UnixMilli(last.Timestamp),
			EntryPrice: entryPrice,
			ExitPrice:  last.Close,
			PnLPct:     pnl,
			Direction:  "long",
		})
		fmt.Printf("  EXIT  %s @ %.2f | end of data | PnL: %+.2f%%\n",
			time.UnixMilli(last.Timestamp).Format("2006-01-02"), last.Close, pnl*100)
	}

	// Report
	fmt.Println()
	fmt.Println("----------------------------------------")
	fmt.Println("Backtest Results")
	fmt.Println("----------------------------------------")
	fmt.Printf("Total Trades: %d\n", len(trades))

	if len(trades) > 0 {
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

		winRate := float64(wins) / float64(len(trades)) * 100
		fmt.Printf("Win Rate: %.1f%% (%d/%d)\n", winRate, wins, len(trades))
		fmt.Printf("Total Return: %+.2f%%\n", (totalReturn-1)*100)
		fmt.Printf("Max Drawdown: %.2f%%\n", maxDrawdown*100)
	}

	fmt.Println()
	fmt.Println("----------------------------------------")
	fmt.Println("Trade Log (CSV format)")
	fmt.Println("----------------------------------------")
	fmt.Println("timestamp,symbol,direction,entry_price,exit_price,pnl_pct")
	for i, t := range trades {
		fmt.Printf("%s,%s,%s,%.2f,%.2f,%+.2f%%\n",
			t.EntryTime.Format("2006-01-02 15:04:05"),
			symbol, t.Direction, t.EntryPrice, t.ExitPrice, t.PnLPct*100)
		_ = i
	}
	fmt.Println()
	fmt.Println("Note: Backtest uses technical signals only (RSI, trend, momentum).")
	fmt.Println("Real decisions also include sentiment, news, and macro data.")
}
