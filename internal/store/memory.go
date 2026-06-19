package store

import (
	"sync"
	"time"

	"trade/internal/models"
)

type MemoryStore struct {
	mu         sync.RWMutex
	strategies map[string]*models.Strategy
	trades     []models.Trade
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		strategies: make(map[string]*models.Strategy),
	}
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
	defer s.mu.Unlock()
	for i := range s.trades {
		if s.trades[i].ID == id {
			s.trades[i].Status = "CLOSED"
			s.trades[i].PnL = pnl
			return
		}
	}
}

func (s *MemoryStore) LogTrade(symbol, side string, size, price, pnl float64, status string) {
	s.AddTrade(models.Trade{
		ID:        time.Now().Format("20060102150405"),
		Symbol:    symbol,
		Side:      side,
		Size:      size,
		Price:     price,
		PnL:       pnl,
		Status:    status,
		CreatedAt: time.Now(),
	})
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
