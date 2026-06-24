package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"trade/internal/models"
)

const skillHubSystemPrompt = `You are TradeLoom AI — a unified crypto analyst powered by 5 Bitget Skill Hub specializations. Analyze the provided market data and return a trading decision.

Frameworks:
1. Technical Analyst: RSI<35 oversold >75 overbought, MACD cross signals trend, SMA7/SMA25 for structure, volume confirms trend quality
2. Sentiment Analyst: F&G <25 extreme fear (contrarian buy) >75 extreme greed (caution), divergence between sentiment and price is meaningful
3. Market Intel: volume flows = capital rotation, market breadth tells health, surging volume = smart money
4. News & Narrative: focus on structural catalysts, correlate news with price, discard noise
5. Macro Analyst: risk-on = favor longs, risk-off = caution, correlate crypto with broad risk appetite

Output: ONLY valid JSON. BUY>=50, WATCHLIST 30-49, NO_TRADE<30.`

const skillHubBriefPrompt = `You are a chief crypto market strategist. Use Bitget Skill Hub frameworks to write data-driven market briefs. Output ONLY valid JSON.`

const skillHubAnalysisPrompt = `You are a crypto market analyst using Bitget Skill Hub frameworks (Sentiment Analyst, News Briefing, Macro Analyst). Synthesize the provided data into a structured JSON report. Output ONLY valid JSON.`

type LLMClient struct {
	apiKey  string
	baseURL string
	model   string
	httpCli *http.Client
}

func NewLLMClient(apiKey, baseURL, model string) *LLMClient {
	return &LLMClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpCli: &http.Client{Timeout: 90 * time.Second},
	}
}

func (l *LLMClient) chatCompletion(system, user string) (string, error) {
	reqBody := map[string]interface{}{
		"model": l.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", l.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+l.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpCli.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("llm API returned %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("llm response parse failed: %w", err)
	}
	if len(raw.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices: %s", string(body))
	}

	return raw.Choices[0].Message.Content, nil
}

func (l *LLMClient) extractJSON(content string) string {
	start := strings.Index(content, "{")
	if start < 0 {
		return content
	}
	end := strings.LastIndex(content, "}")
	if end < start {
		return content
	}
	return content[start : end+1]
}

func (l *LLMClient) ParseIntent(userPrompt string) (*models.UserIntent, error) {
	systemPrompt := `Map the user's trading request to JSON. AI→[FET,TAO,RNDR,WLD] L1→[SOL,AVAX,NEAR] RWA→[ONDO] DEFI→[UNI]. Output: {"assets":["TICKER"],"market_condition":"bullish|bearish|neutral","entry_signals":[],"exit_signals":[],"risk_profile":"low|medium|aggressive","max_risk_per_trade":2}. Only return valid JSON.`

	content, err := l.chatCompletion(systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	content = l.extractJSON(content)

	var intent models.UserIntent
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		return nil, fmt.Errorf("llm intent parse failed: %w\nraw: %s", err, content)
	}

	return &intent, nil
}

func (l *LLMClient) EvaluateTrade(
	symbol string,
	tech map[string]interface{},
	sentimentJSON, newsJSON, macroJSON json.RawMessage,
) (*models.ConfidenceScore, error) {
	trend, _ := tech["trend"].(string)
	rsi, _ := tech["rsi"].(float64)
	change24h, _ := tech["change24h"].(string)
	volume, _ := tech["quoteVolume"].(string)
	price, _ := tech["last_price"].(float64)
	sma7, _ := tech["sma_7"].(float64)
	sma25, _ := tech["sma_25"].(float64)

	macdLabel := "Neutral"
	if sma7 > 0 && sma25 > 0 {
		if sma7 > sma25 {
			macdLabel = "Bullish Cross"
		} else {
			macdLabel = "Bearish Cross"
		}
	}

	fearGreed := 50.0
	sentimentLabel := "Neutral"
	sentScore := 0.5
	if len(sentimentJSON) > 0 {
		var s struct {
			Score     float64 `json:"score"`
			FearGreed float64 `json:"fear_greed"`
			Label     string  `json:"label"`
		}
		if json.Unmarshal(sentimentJSON, &s) == nil {
			fearGreed = s.FearGreed
			sentimentLabel = s.Label
			sentScore = s.Score
		}
	}

	newsSummary := "No news context available"
	if len(newsJSON) > 0 {
		var n struct {
			Summary string `json:"summary"`
		}
		if json.Unmarshal(newsJSON, &n) == nil && n.Summary != "" {
			newsSummary = n.Summary
		}
	}

	riskAppetite := "mixed"
	avgVolume := 0.0
	if len(macroJSON) > 0 {
		var m struct {
			RiskAppetite string  `json:"risk_appetite"`
			TopVol       float64 `json:"top_volume_usdt"`
		}
		if json.Unmarshal(macroJSON, &m) == nil {
			riskAppetite = m.RiskAppetite
			avgVolume = m.TopVol
		}
	}

	userPrompt := fmt.Sprintf(`Analyze %s using Skill Hub frameworks.

Symbol: %s Trend: %s RSI: %.1f 24h: %s Vol: %sUSDT Price: $%.2f MACD: %s (SMA7:%.2f SMA25:%.2f)
F&G: %.0f/100 (%s) Sentiment: %.2f
News: %s
Macro: Risk %s AvgVol: $%.0f

{"decision":"BUY|WATCHLIST|NO_TRADE","confidence":<0-100>,"reasoning":"<2-3 sentences>","signals":[{"name":"Technical","score":<0-30>,"reason":""},{"name":"Sentiment","score":<0-30>,"reason":""},{"name":"Volume","score":<0-20>,"reason":""},{"name":"News","score":<0-20>,"reason":""},{"name":"Macro","score":<0-10>,"reason":""}]}`,
		symbol, trend, rsi, change24h, volume, price,
		macdLabel, sma7, sma25,
		fearGreed, sentimentLabel, sentScore,
		newsSummary,
		riskAppetite, avgVolume,
	)

	content, err := l.chatCompletion(skillHubSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("evaluate trade llm: %w", err)
	}

	content = l.extractJSON(content)

	var result struct {
		Decision   string              `json:"decision"`
		Confidence float64             `json:"confidence"`
		Reasoning  string              `json:"reasoning"`
		Signals    []models.SignalScore `json:"signals"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("evaluate trade parse: %w\nraw: %s", err, content)
	}

	if result.Decision == "" {
		result.Decision = "NO_TRADE"
	}
	if result.Confidence < 0 || result.Confidence > 100 {
		result.Confidence = 0
	}
	if result.Signals == nil {
		result.Signals = []models.SignalScore{}
	}

	dimNames := []struct {
		name     string
		maxScore float64
	}{
		{"Technical", 30},
		{"Sentiment", 30},
		{"Volume", 20},
		{"News", 20},
		{"Macro", 10},
	}
	present := map[string]bool{}
	for _, s := range result.Signals {
		present[s.Name] = true
	}
	for _, d := range dimNames {
		if !present[d.name] {
			result.Signals = append(result.Signals, models.SignalScore{
				Name: d.name, Score: d.maxScore * 0.5, Reason: "insufficient data",
			})
		}
	}

	totalScore := 0.0
	for _, s := range result.Signals {
		totalScore += s.Score
	}
	if result.Confidence <= 0 {
		result.Confidence = totalScore
	}

	fullReasoning := fmt.Sprintf("Confidence Score: %.0f/100 → Decision: %s\n", result.Confidence, result.Decision)
	for _, s := range result.Signals {
		if s.Score > 0 {
			fullReasoning += fmt.Sprintf("%s: +%.0f (%s)\n", s.Name, s.Score, s.Reason)
		}
	}
	if result.Reasoning != "" {
		fullReasoning += fmt.Sprintf("Skill Hub Synthesis: %s", result.Reasoning)
	}

	return &models.ConfidenceScore{
		Total:     result.Confidence,
		Signals:   result.Signals,
		Decision:  result.Decision,
		Reasoning: fullReasoning,
	}, nil
}

type MarketAnalysisResult struct {
	Sentiment map[string]interface{} `json:"sentiment"`
	News      map[string]interface{} `json:"news"`
	Macro     map[string]interface{} `json:"macro"`
}

func (l *LLMClient) AnalyzeMarketWithData(symbol, sentimentData, newsData, macroData string) (*MarketAnalysisResult, error) {
	prompt := fmt.Sprintf(`Analyze %s and return JSON.

SENTIMENT: %s
NEWS: %s
MACRO: %s

{"sentiment":{"score":<0-1>,"fear_greed":<0-100>,"long_short_ratio":1,"summary":""},"news":{"score":<0-1>,"positive_mentions":0,"sentiment":"positive|neutral|negative","summary":""},"macro":{"score":<0-1>,"risk_appetite":"risk-on|mixed|risk-off","summary":""}}`,
		symbol, sentimentData, newsData, macroData)

	content, err := l.chatCompletion(skillHubAnalysisPrompt, prompt)
	if err != nil {
		return nil, err
	}

	content = l.extractJSON(content)

	var result MarketAnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		result = MarketAnalysisResult{
			Sentiment: map[string]interface{}{"score": 0.5, "fear_greed": 50, "summary": "analysis unavailable"},
			News:      map[string]interface{}{"score": 0.5, "sentiment": "neutral", "summary": "analysis unavailable"},
			Macro:     map[string]interface{}{"score": 0.5, "risk_appetite": "mixed", "summary": "analysis unavailable"},
		}
	}

	return &result, nil
}

func (l *LLMClient) GenerateMarketBrief(symbol, sentimentData, newsData, macroData, trendingData string) (string, error) {
	prompt := fmt.Sprintf(`Generate a daily market brief.

TRENDING: %s
SENTIMENT: %s
NEWS: %s
MACRO: %s

{"market_mood":"","top_opportunity":"","narrative":"","key_risk":"","stance":"Accumulate|Selective Buy|Hold|Reduce|Avoid"}`,
		trendingData, sentimentData, newsData, macroData)

	content, err := l.chatCompletion(skillHubBriefPrompt, prompt)
	if err != nil {
		return "", err
	}

	content = l.extractJSON(content)
	return content, nil
}
