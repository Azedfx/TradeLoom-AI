package agent

import (
	"strings"
	"trade/internal/models"
)

var sectorMap = map[string]string{
	"ai":         "FETUSDT",
	"artificial": "FETUSDT",
	"render":     "RNDRUSDT",
	"bittensor":  "TAOUSDT",
	"tao":        "TAOUSDT",
	"rndr":       "RNDRUSDT",
	"defi":       "UNIUSDT",
	"rwa":        "ONDOUSDT",
	"real world": "ONDOUSDT",
	"l1":         "SOLUSDT",
	"layer1":     "SOLUSDT",
	"l2":         "ARBUSDT",
	"layer2":     "ARBUSDT",
	"meme":       "DOGEUSDT",
	"gaming":     "GALAUSDT",
	"infra":      "LINKUSDT",
	"oracle":     "LINKUSDT",
	"storage":    "FILUSDT",
	"depin":      "HNTUSDT",
}

var coinAliases = map[string]string{
	"bitcoin":   "BTC",
	"ethereum":  "ETH",
	"ripple":    "XRP",
	"solana":    "SOL",
	"render":    "RNDR",
	"bittensor": "TAO",
	"ondo":      "ONDO",
	"fetch":     "FET",
	"fetch.ai":  "FET",
	"arbitrum":  "ARB",
	"optimism":  "OP",
	"celestia":  "TIA",
	"worldcoin": "WLD",
}

func ResolveSymbol(assetFilter, defaultSymbol string) string {
	if assetFilter == "" || assetFilter == "all" {
		return defaultSymbol
	}

	parts := strings.Split(assetFilter, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		lower := strings.ToLower(p)

		if sym, ok := sectorMap[lower]; ok {
			return sym
		}

		ticker := p
		if alias, ok := coinAliases[lower]; ok {
			ticker = alias
		}

		sym := strings.ToUpper(ticker) + "USDT"
		if len(sym) > 6 && len(sym) < 20 {
			return sym
		}
	}

	return defaultSymbol
}

func SymbolFromIntent(intent *models.UserIntent, defaultSymbol string) string {
	if len(intent.Assets) == 0 {
		return defaultSymbol
	}
	lower := strings.ToLower(intent.Assets[0])

	if sym, ok := sectorMap[lower]; ok {
		return sym
	}

	ticker := intent.Assets[0]
	if alias, ok := coinAliases[lower]; ok {
		ticker = alias
	}

	sym := strings.ToUpper(ticker) + "USDT"
	if len(sym) > 6 && len(sym) < 20 {
		return sym
	}
	return defaultSymbol
}
