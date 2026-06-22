package store

import (
	"fmt"
	"os"
	"sync"
	"time"

	"trade/internal/models"
)

type MemoryStore struct {
	mu            sync.RWMutex
	strategies    map[string]*models.Strategy
	trades        []models.Trade
	decisionLog   []models.DecisionRecord
	logFile       *os.File
}

func NewMemoryStore(logPath string) *MemoryStore {
	s := &MemoryStore{
		strategies: make(map[string]*models.Strategy),
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		s.logFile = f
		// Write header if file is empty
		info, _ := f.Stat()
		if info.Size() == 0 {
			fmt.Fprintln(f, "timestamp,symbol,direction,price,quantity,pnl,status")
		}
	}

	return s
}

func (s *MemoryStore) SaveStrategy(strategy *models.Strategy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strategies[strategy.ID] = strategy
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
			t := s.trades[i]
			s.mu.Unlock()

			// Append close row to CSV
			if s.logFile != nil {
				fmt.Fprintf(s.logFile, "%s,%s,close,%.8f,%.8f,%.2f,%s\n",
					time.Now().Format("2006-01-02 15:04:05"),
					t.Symbol, exitPrice, t.Size, pnl, "CLOSED")
			}
			return
		}
	}
	s.mu.Unlock()
}

func (s *MemoryStore) LogTrade(symbol, side string, size, price, pnl float64, status string) {
	t := models.Trade{
		ID:        time.Now().Format("20060102150405"),
		Symbol:    symbol,
		Side:      side,
		Size:      size,
		Price:     price,
		PnL:       pnl,
		Status:    status,
		CreatedAt: time.Now(),
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
	fmt.Fprintf(s.logFile, "%s,%s,%s,%.8f,%.8f,%.2f,%s\n",
		t.CreatedAt.Format("2006-01-02 15:04:05"),
		t.Symbol, t.Side, t.Price, t.Size, t.PnL, t.Status)
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
}

func (s *MemoryStore) GetDecisionLog() []models.DecisionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.DecisionRecord, len(s.decisionLog))
	copy(result, s.decisionLog)
	return result
}
