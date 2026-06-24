package agent

import (
	"strings"

	"trade/internal/models"
)

func QuickParseIntent(prompt string) *models.UserIntent {
	lower := strings.ToLower(strings.TrimSpace(prompt))

	intent := &models.UserIntent{
		MarketCondition: "neutral",
		RiskProfile:     "medium",
		MaxRiskPerTrade: 2,
	}

	// Detect specific coin mentions
	coinChecks := []struct {
		keywords []string
		symbol   string
	}{
		{[]string{"bitcoin", " btc"}, "BTC"},
		{[]string{"ethereum", " eth"}, "ETH"},
		{[]string{"solana", " sol"}, "SOL"},
		{[]string{" fetch", "fet "}, "FET"},
		{[]string{" tao", "tao "}, "TAO"},
		{[]string{"render", "rndr"}, "RNDR"},
		{[]string{"chainlink", " link"}, "LINK"},
		{[]string{"avalanche", "avax"}, "AVAX"},
		{[]string{"polkadot", " dot", "dot "}, "DOT"},
		{[]string{"dogecoin", "doge"}, "DOGE"},
		{[]string{"polygon", "matic"}, "MATIC"},
		{[]string{"arbitrum", " arb", "arb "}, "ARB"},
		{[]string{"optimism", " op", "op "}, "OP"},
		{[]string{"aptos", " apt", "apt "}, "APT"},
		{[]string{" sui", "sui "}, "SUI"},
		{[]string{"near"}, "NEAR"},
	}
	for _, c := range coinChecks {
		for _, kw := range c.keywords {
			if strings.Contains(lower, kw) {
				intent.Assets = append(intent.Assets, c.symbol)
				break
			}
		}
	}

	// Detect sectors → expand to known coins
	if strings.Contains(lower, "defi") || strings.Contains(lower, "dex") {
		intent.Assets = append(intent.Assets, "UNI", "AAVE", "MKR", "CRV")
	}
	if strings.Contains(lower, "ai ") || strings.HasSuffix(lower, " ai") || strings.Contains(lower, "artificial") || strings.Contains(lower, "agent") {
		intent.Assets = append(intent.Assets, "FET", "TAO", "RNDR", "AGIX", "NEAR")
	}
	if strings.Contains(lower, "meme") || strings.Contains(lower, "memecoin") {
		intent.Assets = append(intent.Assets, "DOGE", "SHIB", "PEPE", "WIF")
	}
	if strings.Contains(lower, "rwa") || strings.Contains(lower, "real world") {
		intent.Assets = append(intent.Assets, "ONDO", "MKR", "CFG")
	}
	if strings.Contains(lower, "layer1") || strings.Contains(lower, "layer 1") || strings.Contains(lower, "infrastructure") || strings.Contains(lower, "l1") {
		intent.Assets = append(intent.Assets, "SOL", "AVAX", "NEAR", "APT", "SUI")
	}
	if strings.Contains(lower, "layer2") || strings.Contains(lower, "layer 2") || strings.Contains(lower, "scaling") || strings.Contains(lower, "l2") {
		intent.Assets = append(intent.Assets, "ARB", "OP", "MATIC", "STRK")
	}
	if strings.Contains(lower, "gaming") || strings.Contains(lower, "gamefi") || strings.Contains(lower, "metaverse") {
		intent.Assets = append(intent.Assets, "IMX", "SAND", "GALA")
	}

	// Deduplicate
	seen := make(map[string]bool)
	unique := make([]string, 0, len(intent.Assets))
	for _, a := range intent.Assets {
		if !seen[a] {
			seen[a] = true
			unique = append(unique, a)
		}
	}
	intent.Assets = unique

	// Detect market condition
	if strings.Contains(lower, "bull") || strings.Contains(lower, "momentum") || strings.Contains(lower, "strong") {
		intent.MarketCondition = "bullish"
	} else if strings.Contains(lower, "bear") || strings.Contains(lower, "weak") || strings.Contains(lower, "dump") {
		intent.MarketCondition = "bearish"
	}

	// Detect risk
	if strings.Contains(lower, "aggres") || strings.Contains(lower, "high risk") || strings.Contains(lower, "degen") {
		intent.RiskProfile = "aggressive"
	} else if strings.Contains(lower, "low risk") || strings.Contains(lower, "safe") || strings.Contains(lower, "conservative") {
		intent.RiskProfile = "low"
	}

	// Detect entry signals
	if strings.Contains(lower, "momentum") || strings.Contains(lower, "breakout") || strings.Contains(lower, "break out") {
		intent.EntrySignals = append(intent.EntrySignals, "momentum")
	}
	if strings.Contains(lower, "oversold") || strings.Contains(lower, "dip") || strings.Contains(lower, "buying") {
		intent.EntrySignals = append(intent.EntrySignals, "oversold")
	}

	return intent
}
