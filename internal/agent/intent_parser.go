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

	// Detect specific coin mentions — ordered by specificity
	coinChecks := []struct {
		keywords []string
		symbol   string
	}{
		{[]string{"bitcoin", " btc"}, "BTC"},
		{[]string{"ethereum", " eth"}, "ETH"},
		{[]string{"solana", " sol"}, "SOL"},
		{[]string{" fetch", " fet", "fetch.ai"}, "FET"},
		{[]string{" tao", " tao", "bittensor"}, "TAO"},
		{[]string{"render", " rndr"}, "RNDR"},
		{[]string{"chainlink", " link"}, "LINK"},
		{[]string{"avalanche", "avax"}, "AVAX"},
		{[]string{"polkadot", " dot "}, "DOT"},
		{[]string{"dogecoin", "doge"}, "DOGE"},
		{[]string{"polygon", "matic"}, "MATIC"},
		{[]string{"arbitrum", " arb"}, "ARB"},
		{[]string{"optimism", " op "}, "OP"},
		{[]string{"aptos", " apt"}, "APT"},
		{[]string{" sui", "sui "}, "SUI"},
		{[]string{"near"}, "NEAR"},
		{[]string{"ondo", " ondo"}, "ONDO"},
		{[]string{"worldcoin", "wld"}, "WLD"},
		{[]string{"uniswap", "uni"}, "UNI"},
		{[]string{"aave"}, "AAVE"},
		{[]string{"maker", "mkr"}, "MKR"},
		{[]string{"curve", "crv"}, "CRV"},
		{[]string{"shiba", "shib"}, "SHIB"},
		{[]string{"pepe"}, "PEPE"},
		{[]string{"wif", "dogwifhat"}, "WIF"},
		{[]string{"sei"}, "SEI"},
		{[]string{"injective", "inj"}, "INJ"},
		{[]string{"celestia", "tia"}, "TIA"},
		{[]string{"strk", "starknet"}, "STRK"},
		{[]string{"immutable", "imx"}, "IMX"},
		{[]string{"sandbox", "sand"}, "SAND"},
		{[]string{"gala"}, "GALA"},
		{[]string{"agix", "singularity"}, "AGIX"},
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

	// If no known coins found, try extracting standalone uppercase words as tickers
	if len(intent.Assets) == 0 {
		for _, word := range strings.Fields(lower) {
			word = strings.Trim(word, ",.;:!?()[]{}\"'")
			if len(word) >= 2 && len(word) <= 6 && isAlpha(word) {
				sym := strings.ToUpper(word)
				if sym != "FOR" && sym != "THE" && sym != "AND" && sym != "CHECK" {
					intent.Assets = append(intent.Assets, sym)
				}
			}
		}
	}

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

func isAlpha(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}
