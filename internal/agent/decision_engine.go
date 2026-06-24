package agent

import (
	"fmt"
	"strings"

	"trade/internal/bitget"
	"trade/internal/models"
)

type DecisionEngine struct {
	marketData *bitget.MarketData
}

func NewDecisionEngine(md *bitget.MarketData) *DecisionEngine {
	return &DecisionEngine{marketData: md}
}

func (d *DecisionEngine) EvaluateWithConfidence(market *models.MarketPerception) *models.ConfidenceScore {
	signals := []models.SignalScore{}

	// 1. Sentiment analysis (up to +30)
	sig := d.scoreSentiment(market)
	signals = append(signals, sig)

	// 2. Technical trend (up to +30)
	sig2 := d.scoreTechnical(market)
	signals = append(signals, sig2)

	// 3. Volume / momentum (up to +20)
	sig3 := d.scoreVolume(market)
	signals = append(signals, sig3)

	// 4. News (up to +20)
	sig4 := d.scoreNews(market)
	signals = append(signals, sig4)

	// 5. Macro (up to +10 bonus)
	sig5 := d.scoreMacro(market)
	signals = append(signals, sig5)

	total := 0.0
	parts := []string{}
	for _, s := range signals {
		total += s.Score
		if s.Score > 0 {
			parts = append(parts, fmt.Sprintf("%s: +%.0f (%s)", s.Name, s.Score, s.Reason))
		}
	}

	decision := "NO_TRADE"
	if total >= 70 {
		decision = "BUY"
	} else if total >= 50 {
		decision = "WATCHLIST"
	}

	reasoning := fmt.Sprintf("Confidence Score: %.0f/100 → Decision: %s\n", total, decision)
	if len(parts) > 0 {
		reasoning += strings.Join(parts, "\n")
	} else {
		reasoning += "No clear signals detected"
	}

	return &models.ConfidenceScore{
		Total:     total,
		Signals:   signals,
		Decision:  decision,
		Reasoning: reasoning,
	}
}

func (d *DecisionEngine) scoreSentiment(market *models.MarketPerception) models.SignalScore {
	score := 0.0
	reason := "no signal"

	if market.Sentiment == nil {
		return models.SignalScore{Name: "Sentiment", Score: 0, Reason: reason}
	}

	if v, ok := market.Sentiment["score"].(float64); ok && v > 0.6 {
		score = 30
		reason = fmt.Sprintf("sentiment score %.2f (bullish)", v)
	} else if v, ok := market.Sentiment["score"].(float64); ok && v > 0.4 {
		score = 15
		reason = fmt.Sprintf("sentiment score %.2f (neutral)", v)
	} else if v, ok := market.Sentiment["score"].(float64); ok {
		reason = fmt.Sprintf("sentiment score %.2f (bearish)", v)
	}

	if fg, ok := market.Sentiment["fear_greed"].(float64); ok && fg > 50 {
		score += 5
		reason += fmt.Sprintf("; fear & greed: greedy (%.0f)", fg)
	} else if fg, ok := market.Sentiment["fear_greed"].(float64); ok {
		reason += fmt.Sprintf("; fear & greed: fearful (%.0f)", fg)
	}

	if ls, ok := market.Sentiment["long_short_ratio"].(float64); ok && ls > 1.2 {
		score += 5
		reason += "; long/short ratio bullish"
	}

	if score > 30 {
		score = 30
	}

	return models.SignalScore{Name: "Sentiment", Score: score, Reason: reason}
}

func (d *DecisionEngine) scoreTechnical(market *models.MarketPerception) models.SignalScore {
	score := 0.0
	reason := "no signal"

	if market.Technical == nil {
		return models.SignalScore{Name: "Technical", Score: 0, Reason: reason}
	}

	if trend, ok := market.Technical["trend"].(string); ok {
		switch trend {
		case "bullish":
			score = 20
			reason = "trend is bullish"
		case "bearish":
			reason = "trend is bearish"
		default:
			score = 10
			reason = "trend is " + trend
		}
	}

	if change, ok := market.Technical["change24h"].(string); ok {
		var changeF float64
		fmt.Sscanf(change, "%f", &changeF)
		if changeF > 0.02 {
			score += 10
			reason += "; +24h change " + change
		} else if changeF > 0 {
			score += 5
			reason += "; +24h change " + change
		} else {
			reason += "; 24h change " + change
		}
	}

	if rsi, ok := market.Technical["rsi"].(float64); ok {
		if rsi > 60 && rsi < 80 {
			score += 5
			reason += fmt.Sprintf("; RSI %.0f (bullish)", rsi)
		} else if rsi >= 80 {
			reason += fmt.Sprintf("; RSI %.0f (overbought)", rsi)
		} else {
			reason += fmt.Sprintf("; RSI %.0f", rsi)
		}
	}

	if sma7, ok := market.Technical["sma_7"].(float64); ok {
		if sma25Val, ok := market.Technical["sma_25"].(float64); ok {
			if sma7 > sma25Val {
				reason += "; SMA7 above SMA25 (bullish cross)"
			} else if sma7 < sma25Val {
				reason += "; SMA7 below SMA25 (bearish cross)"
			}
		}
	}

	if lastPrice, ok := market.Technical["last_price"].(float64); ok {
		if sma25Val, ok := market.Technical["sma_25"].(float64); ok && sma25Val > 0 {
			if lastPrice > sma25Val {
				reason += "; price above SMA25"
			} else if lastPrice < sma25Val {
				reason += "; price below SMA25"
			}
		}
	}

	if score > 30 {
		score = 30
	}

	return models.SignalScore{Name: "Technical", Score: score, Reason: reason}
}

func (d *DecisionEngine) scoreVolume(market *models.MarketPerception) models.SignalScore {
	score := 0.0
	reason := "no signal"

	if market.Technical == nil {
		return models.SignalScore{Name: "Volume", Score: 0, Reason: reason}
	}

	if volume, ok := market.Technical["quoteVolume"].(string); ok {
		var volF float64
		fmt.Sscanf(volume, "%f", &volF)
		if volF > 500000000 {
			score = 20
			reason = fmt.Sprintf("high volume: %.0f USDT", volF)
		} else if volF > 100000000 {
			score = 10
			reason = fmt.Sprintf("moderate volume: %.0f USDT", volF)
		} else {
			reason = fmt.Sprintf("low volume: %.0f USDT", volF)
		}
	}

	if score > 20 {
		score = 20
	}

	return models.SignalScore{Name: "Volume", Score: score, Reason: reason}
}

func (d *DecisionEngine) scoreNews(market *models.MarketPerception) models.SignalScore {
	score := 0.0
	reason := "no signal"

	if market.News == nil {
		return models.SignalScore{Name: "News", Score: 0, Reason: reason}
	}

	if mentions, ok := market.News["positive_mentions"].(float64); ok && mentions > 10 {
		score = 15
		reason = fmt.Sprintf("positive mentions: %.0f", mentions)
	} else if mentions, ok := market.News["positive_mentions"].(float64); ok && mentions > 5 {
		score = 10
		reason = fmt.Sprintf("positive mentions: %.0f", mentions)
	}

	if sentiment, ok := market.News["sentiment"].(string); ok && sentiment == "positive" {
		score += 5
		reason += "; news sentiment positive"
	}

	if score > 20 {
		score = 20
	}

	return models.SignalScore{Name: "News", Score: score, Reason: reason}
}

func (d *DecisionEngine) scoreMacro(market *models.MarketPerception) models.SignalScore {
	score := 0.0
	reason := "no signal"

	if market.Macro == nil {
		return models.SignalScore{Name: "Macro", Score: 0, Reason: reason}
	}

	if v, ok := market.Macro["score"].(float64); ok && v > 0.6 {
		score = 5
		reason = fmt.Sprintf("macro score %.2f (favorable)", v)
	} else if v, ok := market.Macro["score"].(float64); ok && v > 0.4 {
		score = 2
		reason = fmt.Sprintf("macro score %.2f (neutral)", v)
	} else if v, ok := market.Macro["score"].(float64); ok {
		reason = fmt.Sprintf("macro score %.2f (unfavorable)", v)
	}

	if appetite, ok := market.Macro["risk_appetite"].(string); ok {
		switch appetite {
		case "risk-on":
			score += 5
			reason += "; risk appetite: risk-on"
		case "mixed":
			score += 2
			reason += "; risk appetite: mixed"
		default:
			reason += "; risk appetite: risk-off"
		}
	}

	if score > 10 {
		score = 10
	}

	return models.SignalScore{Name: "Macro", Score: score, Reason: reason}
}

func (d *DecisionEngine) Explain(symbol string, score *models.ConfidenceScore) string {
	var b strings.Builder

	if score.Decision == "BUY" {
		fmt.Fprintf(&b, "Buying %s because:\n", symbol)
	} else if score.Decision == "NO_TRADE" {
		fmt.Fprintf(&b, "No trade found.\nMarket conditions are weak.\n")
		fmt.Fprintf(&b, "\nConfidence Score: %.0f/100\n", score.Total)
		return b.String()
	} else {
		fmt.Fprintf(&b, "Decision on %s (%s):\n", symbol, score.Decision)
	}

	for _, s := range score.Signals {
		if s.Score == 0 {
			continue
		}
		bullet := d.signalToBullet(s)
		if bullet != "" {
			fmt.Fprintf(&b, "  - %s\n", bullet)
		}
	}

	fmt.Fprintf(&b, "\nConfidence Score: %.0f/100\n", score.Total)

	return b.String()
}

func (d *DecisionEngine) signalToBullet(s models.SignalScore) string {
	reason := s.Reason

	cls := strings.ToLower(s.Name)
	switch {
	case strings.Contains(reason, "sentiment score"):
		score := extractFloat(reason, "score")
		if score > 0.6 {
			return fmt.Sprintf("Sentiment Score: %.0f — bullish", score*100)
		}
		return fmt.Sprintf("Sentiment Score: %.0f", score*100)
	case strings.Contains(reason, "fear & greed"):
		fg := extractFloat(reason, "greedy")
		if fg > 0 {
			return fmt.Sprintf("Fear & Greed: greedy (%.0f)", fg)
		}
		return "Fear & Greed: fearful"
	case strings.Contains(reason, "long/short"):
		return "Long/Short ratio: bullish"
	case cls == "technical":
		var tech strings.Builder

		trendPart := "Unknown"
		if strings.Contains(reason, "trend is bullish") {
			trendPart = "Bullish"
		} else if strings.Contains(reason, "trend is bearish") {
			trendPart = "Bearish"
		} else if strings.Contains(reason, "trend is neutral") {
			trendPart = "Neutral"
		}

		tech.WriteString(fmt.Sprintf("Technical: %s", trendPart))

		rsiVal := extractFloat(reason, "RSI ")
		if rsiVal > 0 {
			tech.WriteString(fmt.Sprintf("\n    RSI: %.0f", rsiVal))
		}

		if strings.Contains(reason, "bullish cross") {
			tech.WriteString("\n    MACD: Bullish Cross")
		} else if strings.Contains(reason, "bearish cross") {
			tech.WriteString("\n    MACD: Bearish Cross")
		}

		if strings.Contains(reason, "price above SMA25") {
			tech.WriteString("\n    Above SMA25")
		} else if strings.Contains(reason, "price below SMA25") {
			tech.WriteString("\n    Below SMA25")
		}

		return tech.String()
	case strings.Contains(reason, "24h change"):
		ch := extractFloat(reason, "change")
		if ch < 0 {
			return fmt.Sprintf("24h change: %.2f%% (bearish)", ch*100)
		}
		return fmt.Sprintf("24h change: +%.2f%%", ch*100)
	case strings.Contains(reason, "high volume"):
		return "Volume: high — increased activity detected"
	case strings.Contains(reason, "moderate volume"):
		return "Volume: moderate"
	case strings.Contains(reason, "positive mentions"):
		return "Positive AI narrative detected"
	case strings.Contains(reason, "news sentiment positive"):
		return "News sentiment: positive"
	default:
		if reason != "" && reason != "no signal" {
			return s.Name + ": " + reason
		}
		return ""
	}
}

func extractFloat(s, prefix string) float64 {
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return 0
	}
	rest := s[idx+len(prefix):]
	var val float64
	fmt.Sscanf(rest, "%f", &val)
	return val
}


