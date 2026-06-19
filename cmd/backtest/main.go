package main

import (
	"fmt"
	"log"
	"os"

	"trade/internal/backtest"
	"trade/internal/bitget"
)

func main() {
	symbol := "BTCUSDT"
	if len(os.Args) > 1 {
		symbol = os.Args[1]
	}

	days := 90
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &days)
	}

	md := &bitget.MarketData{}
	bt := backtest.New(md)
	report := bt.Run(symbol, days)

	fmt.Print(report.String())

	if len(report.Trades) == 0 {
		log.Printf("No trades generated — insufficient data for %s", symbol)
		os.Exit(1)
	}
}
