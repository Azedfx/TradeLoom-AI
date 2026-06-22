package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"trade/internal/models"
)

type LLMClient struct {
	apiKey    string
	baseURL   string
	model     string
	httpCli   *http.Client
	briefModel string
}

func NewLLMClient(apiKey, baseURL, model string) *LLMClient {
	return &LLMClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		httpCli:    &http.Client{Timeout: 60 * time.Second},
		briefModel: "qwen3.6-flash",
	}
}

func (l *LLMClient) ParseIntent(userPrompt string) (*models.UserIntent, error) {
	systemPrompt := `
You are a trading strategy compiler. Convert natural language trading ideas into structured JSON.
Be specific about assets.
- If the user asks for "AI coins", pick one from [FET, TAO, RNDR, WLD].
- If the user asks for "RWA", pick [ONDO].
- If the user asks for "L1", pick [SOL, AVAX, NEAR].
- If they ask for a specific coin, use that.

Output format:
{
  "assets": ["TICKER"] (e.g., ["FET"] or ["ONDO"]),
  "market_condition": "bullish" | "bearish" | "neutral" | null,
...
`
	reqBody := map[string]interface{}{
		"model": l.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", l.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+l.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("llm API returned %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("llm response parse failed: %w", err)
	}

	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("llm returned no choices: %s", string(body))
	}

	content := raw.Choices[0].Message.Content

	// strip markdown code fence if present
	if len(content) > 7 && content[:3] == "```" {
		start := 0
		for i := 0; i < len(content); i++ {
			if content[i] == '{' {
				start = i
				break
			}
		}
		end := len(content)
		for i := len(content) - 1; i >= 0; i-- {
			if content[i] == '}' {
				end = i + 1
				break
			}
		}
		if start > 0 && end > start {
			content = content[start:end]
		}
	}

	var intent models.UserIntent
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		return nil, fmt.Errorf("llm intent parse failed: %w\nraw: %s", err, content)
	}

	return &intent, nil
}

type MarketAnalysisResult struct {
	Sentiment map[string]interface{} `json:"sentiment"`
	News      map[string]interface{} `json:"news"`
	Macro     map[string]interface{} `json:"macro"`
}

func (l *LLMClient) AnalyzeMarketWithData(symbol, sentimentData, newsData, macroData string) (*MarketAnalysisResult, error) {
	prompt := fmt.Sprintf(`Analyze %s using this real-time data from Bitget Skill Hub and return a JSON.
Today is June 15, 2026.

[SENTIMENT DATA]
%s

[NEWS DATA]
%s

[MACRO DATA]
%s

Output format:
{
  "sentiment": {
    "score": <0.0-1.0>,
    "fear_greed": <0-100>,
    "long_short_ratio": <float>,
    "summary": "<one sentence>"
  },
  "news": {
    "score": <0.0-1.0>,
    "positive_mentions": <count>,
    "sentiment": "<positive|neutral|negative>",
    "summary": "<one sentence>"
  },
  "macro": {
    "score": <0.0-1.0>,
    "risk_appetite": "<risk-on|mixed|risk-off>",
    "summary": "<one sentence>"
  }
}
Return ONLY valid JSON.`, symbol, sentimentData, newsData, macroData)

	reqBody := map[string]interface{}{
		"model": l.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a crypto market analyst. Synthesize the provided data into a structured JSON report."},
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", l.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+l.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("market analysis request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("market analysis returned %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(body, &raw)

	if len(raw.Choices) == 0 {
		return &MarketAnalysisResult{}, nil
	}

	content := raw.Choices[0].Message.Content
	if len(content) > 7 && content[:3] == "```" {
		start := 0
		for i := 0; i < len(content); i++ {
			if content[i] == '{' {
				start = i
				break
			}
		}
		end := len(content)
		for i := len(content) - 1; i >= 0; i-- {
			if content[i] == '}' {
				end = i + 1
				break
			}
		}
		if start > 0 && end > start {
			content = content[start:end]
		}
	}

	var result MarketAnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return &MarketAnalysisResult{}, nil
	}

	return &result, nil
}

func (l *LLMClient) GenerateMarketBrief(symbol, sentimentData, newsData, macroData, trendingData string) (string, error) {
	prompt := fmt.Sprintf(`Generate a daily crypto market brief using this real-time data.

Current date: June 2026.

[TRENDING COINS]
%s

[SENTIMENT DATA]
%s

[NEWS DATA]
%s

[MACRO DATA]
%s

Return a JSON with these exact fields:
{
  "market_mood": "<one or two words: e.g. Cautiously Bullish, Risk-Off, etc>",
  "top_opportunity": "<coin ticker>",
  "narrative": "<2-3 sentences on what is driving the market right now>",
  "key_risk": "<one sentence about the biggest risk>",
  "stance": "<Accumulate | Selective Buy | Hold | Reduce | Avoid>"
}
Return ONLY valid JSON.`, trendingData, sentimentData, newsData, macroData)

	reqBody := map[string]interface{}{
		"model": l.briefModel,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a chief crypto market strategist. Write concise, data-driven market briefs."},
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", l.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+l.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpCli.Do(req)
	if err != nil {
		return "", fmt.Errorf("brief request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("brief returned %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(body, &raw)

	if len(raw.Choices) == 0 {
		return `{"market_mood":"Unavailable","narrative":"Could not generate brief."}`, nil
	}

	content := raw.Choices[0].Message.Content
	if len(content) > 7 && content[:3] == "```" {
		start := 0
		for i := 0; i < len(content); i++ {
			if content[i] == '{' {
				start = i
				break
			}
		}
		end := len(content)
		for i := len(content) - 1; i >= 0; i-- {
			if content[i] == '}' {
				end = i + 1
				break
			}
		}
		if start > 0 && end > start {
			content = content[start:end]
		}
	}

	return content, nil
}
