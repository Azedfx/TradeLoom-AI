package models

import "time"

type UserIntent struct {
	Assets          []string `json:"assets"`
	MarketCondition string   `json:"market_condition"`
	EntrySignals    []string `json:"entry_signals"`
	ExitSignals     []string `json:"exit_signals"`
	RiskProfile     string   `json:"risk_profile"`
	MaxRiskPerTrade int      `json:"max_risk_per_trade"`
}

type Strategy struct {
	ID           string        `json:"id"`
	UserPrompt   string        `json:"user_prompt"`
	Rules        StrategyRules `json:"rules"`
	TargetSymbol string        `json:"target_symbol"`
	Active       bool          `json:"active"`
	Entered      bool          `json:"entered"` // New field
	CreatedAt    time.Time     `json:"created_at"`
}

type StrategyRules struct {
	EntryCondition Condition `json:"entry_condition"`
	ExitCondition  Condition `json:"exit_condition"`
	AssetFilter    string    `json:"asset_filter"`
	PositionSize   string    `json:"position_size"`
}

type Condition struct {
	Type          string      `json:"type"`
	Operator      string      `json:"operator"`
	Threshold     float64     `json:"threshold"`
	SubConditions []Condition `json:"sub_conditions,omitempty"`
}

type MarketPerception struct {
	Technical map[string]interface{} `json:"technical"`
	Sentiment map[string]interface{} `json:"sentiment"`
	News      map[string]interface{} `json:"news"`
	Macro     map[string]interface{} `json:"macro"`
}

type Trade struct {
	ID             string    `json:"id"`
	Symbol         string    `json:"symbol"`
	Side           string    `json:"side"`
	Size           float64   `json:"size"`
	Price          float64   `json:"price"`
	PnL            float64   `json:"pnl"`
	ExitPrice      float64   `json:"exit_price"`
	Status         string    `json:"status"` // "OPEN", "CLOSED"
	BalanceChange  float64   `json:"balance_change"` // change to account balance on this trade
	CreatedAt      time.Time `json:"created_at"`
}

type Position struct {
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Size       float64   `json:"size"`
	EntryPrice float64   `json:"entry_price"`
	CreatedAt  time.Time `json:"created_at"`
}

type SignalScore struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

type DecisionRecord struct {
	Time      time.Time     `json:"time"`
	Symbol    string        `json:"symbol"`
	Total     float64       `json:"total"`
	Decision  string        `json:"decision"`
	Sentiment float64       `json:"sentiment"`
	Technical float64       `json:"technical"`
	Volume    float64       `json:"volume"`
	News      float64       `json:"news"`
	Macro     float64       `json:"macro"`
}

type ConfidenceScore struct {
	Total     float64       `json:"total"`
	Signals   []SignalScore `json:"signals"`
	Decision  string        `json:"decision"`
	Reasoning string        `json:"reasoning"`
}

type Opportunity struct {
	Symbol         string  `json:"symbol"`
	Decision       string  `json:"decision"`
	Confidence     float64 `json:"confidence"`
	SentimentScore float64 `json:"sentiment_score"`
	TechScore      float64 `json:"tech_score"`
	VolumeScore    float64 `json:"volume_score"`
	NewsScore      float64 `json:"news_score"`
	MacroScore     float64 `json:"macro_score"`
	Reasoning      string  `json:"reasoning"`
	Price          float64 `json:"price"`
	Trend          string  `json:"trend"`
	RSI            float64 `json:"rsi"`
	MACD           string  `json:"macd"`
	Change24h      string  `json:"change24h"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    string `json:"time"`
}
