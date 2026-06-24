package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"trade/internal/models"
)

type MemoryStore struct {
	mu                sync.RWMutex
	strategies        map[string]*models.Strategy
	trades            []models.Trade
	decisionLog       []models.DecisionRecord
	logFile           *os.File
	LastOpportunities []models.Opportunity `json:"last_opportunities"`
	LastPrompt        string               `json:"last_prompt"`
	AccountBalance    float64              `json:"account_balance"`
	Conversation      []models.ChatMessage `json:"conversation"`
	statePath         string
}

type storeSnapshot struct {
	LastOpportunities []models.Opportunity `json:"last_opportunities"`
	LastPrompt        string               `json:"last_prompt"`
	AccountBalance    float64              `json:"account_balance"`
	Conversation      []models.ChatMessage `json:"conversation"`
	Trades            []models.Trade       `json:"trades"`
	DecisionLog       []models.DecisionRecord `json:"decision_log"`
}

func NewMemoryStore(logPath string, statePath string, defaultCapital float64) *MemoryStore {
	if defaultCapital <= 0 {
		defaultCapital = 1000
	}
	s := &MemoryStore{
		strategies:     make(map[string]*models.Strategy),
		AccountBalance: defaultCapital,
		statePath:      statePath,
	}

	// Load persisted state
	if statePath != "" {
		s.loadState()
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		s.logFile = f
		info, _ := f.Stat()
		if info.Size() == 0 {
				fmt.Fprintln(f, "timestamp,symbol,direction,price,quantity,pnl,balance_change,status")
			}
	}

	return s
}

func (s *MemoryStore) loadState() {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		return // file doesn't exist yet
	}
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return
	}
	s.LastOpportunities = snap.LastOpportunities
	s.LastPrompt = snap.LastPrompt
	if snap.AccountBalance > 0 {
		s.AccountBalance = snap.AccountBalance
	}
	s.Conversation = snap.Conversation
	s.trades = snap.Trades
	s.decisionLog = snap.DecisionLog
}

func (s *MemoryStore) persist() {
	if s.statePath == "" {
		return
	}
	snap := storeSnapshot{
		LastOpportunities: s.LastOpportunities,
		LastPrompt:        s.LastPrompt,
		AccountBalance:    s.AccountBalance,
		Conversation:      s.Conversation,
		Trades:            s.trades,
		DecisionLog:       s.decisionLog,
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return
	}
	// Ensure directory exists
	lastSlash := -1
	for i := len(s.statePath) - 1; i >= 0; i-- {
		if s.statePath[i] == '/' {
			lastSlash = i
			break
		}
	}
	if lastSlash >= 0 {
		os.MkdirAll(s.statePath[:lastSlash], 0755)
	}
	os.WriteFile(s.statePath, data, 0644)
}

func (s *MemoryStore) SaveStrategy(strategy *models.Strategy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strategies[strategy.ID] = strategy
	s.persist()
}

func (s *MemoryStore) GetStrategy(id string) *models.Strategy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.strategies[id]
}

func (s *MemoryStore) GetAllStrategies() []*models.Strategy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*models.Strategy, 0, len(s.strategies))
	for _, v := range s.strategies {
		result = append(result, v)
	}
	return result
}

func (s *MemoryStore) AddTrade(trade models.Trade) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trades = append(s.trades, trade)
	s.persist()
}

func (s *MemoryStore) GetOpenPositions() []models.Trade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var open []models.Trade
	for _, t := range s.trades {
		if t.Status == "OPEN" {
			open = append(open, t)
		}
	}
	return open
}

func (s *MemoryStore) CloseTrade(id string, exitPrice, pnl float64) {
	s.mu.Lock()
	for i := range s.trades {
		if s.trades[i].ID == id {
			s.trades[i].Status = "CLOSED"
			s.trades[i].PnL = pnl
			s.trades[i].ExitPrice = exitPrice
			t := s.trades[i]
			lockedCapital := t.Size * t.Price
			s.AccountBalance += lockedCapital + pnl
			t.BalanceChange = pnl
			s.trades[i].BalanceChange = pnl
			s.mu.Unlock()

			if s.logFile != nil {
				fmt.Fprintf(s.logFile, "%s,%s,close,%.8f,%.8f,%.2f,%.2f,%s\n",
					time.Now().Format("2006-01-02 15:04:05"),
					t.Symbol, exitPrice, t.Size, pnl, t.BalanceChange, "CLOSED")
			}
			s.mu.Lock()
			s.persist()
			s.mu.Unlock()
			return
		}
	}
	s.mu.Unlock()
}

func (s *MemoryStore) LogTrade(symbol, side string, size, price, pnl float64, status string) {
	balanceChange := -size * price
	t := models.Trade{
		ID:            time.Now().Format("20060102150405"),
		Symbol:        symbol,
		Side:          side,
		Size:          size,
		Price:         price,
		PnL:           pnl,
		Status:        status,
		BalanceChange: balanceChange,
		CreatedAt:     time.Now(),
	}
	s.AddTrade(t)
	s.appendToLog(t)
}

func (s *MemoryStore) appendToLog(t models.Trade) {
	if s.logFile == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.logFile, "%s,%s,%s,%.8f,%.8f,%.2f,%.2f,%s\n",
		t.CreatedAt.Format("2006-01-02 15:04:05"),
		t.Symbol, t.Side, t.Price, t.Size, t.PnL, t.BalanceChange, t.Status)
}

func (s *MemoryStore) GetAllTrades() []models.Trade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.Trade, len(s.trades))
	copy(result, s.trades)
	return result
}

func (s *MemoryStore) CalculatePnL() float64 {
	var total float64
	for _, t := range s.GetAllTrades() {
		total += t.PnL
	}
	return total
}

func (s *MemoryStore) CalculateWinRate() float64 {
	trades := s.GetAllTrades()
	if len(trades) == 0 {
		return 0
	}
	wins := 0
	for _, t := range trades {
		if t.PnL > 0 {
			wins++
		}
	}
	return float64(wins) / float64(len(trades)) * 100
}

func (s *MemoryStore) LogDecision(symbol string, total float64, decision string, signals []models.SignalScore) {
	r := models.DecisionRecord{
		Time:     time.Now(),
		Symbol:   symbol,
		Total:    total,
		Decision: decision,
	}
	for _, sig := range signals {
		switch sig.Name {
		case "Sentiment":
			r.Sentiment = sig.Score
		case "Technical":
			r.Technical = sig.Score
		case "Volume":
			r.Volume = sig.Score
		case "News":
			r.News = sig.Score
		case "Macro":
			r.Macro = sig.Score
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisionLog = append(s.decisionLog, r)
	s.persist()
}

func (s *MemoryStore) GetDecisionLog() []models.DecisionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.DecisionRecord, len(s.decisionLog))
	copy(result, s.decisionLog)
	return result
}

func (s *MemoryStore) SetLastOpportunities(opps []models.Opportunity, prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastOpportunities = opps
	s.LastPrompt = prompt
	s.persist()
}

func (s *MemoryStore) GetLastOpportunities() ([]models.Opportunity, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.Opportunity, len(s.LastOpportunities))
	copy(result, s.LastOpportunities)
	return result, s.LastPrompt
}

func (s *MemoryStore) AddChatMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Conversation = append(s.Conversation, models.ChatMessage{
		Role:    role,
		Content: content,
		Time:    time.Now().Format("15:04:05"),
	})
	if len(s.Conversation) > 50 {
		s.Conversation = s.Conversation[len(s.Conversation)-50:]
	}
	s.persist()
}

func (s *MemoryStore) GetConversation() []models.ChatMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.ChatMessage, len(s.Conversation))
	copy(result, s.Conversation)
	return result
}
