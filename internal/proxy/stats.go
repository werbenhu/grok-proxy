package proxy

import (
	"sync"
	"time"
)

type Statistics struct {
	TotalRequests  int64  `json:"totalRequests"`
	ActiveRequests int64  `json:"activeRequests"`
	LastError      string `json:"lastError,omitempty"`
	LastRequestAt  string `json:"lastRequestAt,omitempty"`
}

type statisticsStore struct {
	mu    sync.RWMutex
	value Statistics
}

func (s *statisticsStore) begin() func() {
	s.mu.Lock()
	s.value.TotalRequests++
	s.value.ActiveRequests++
	s.value.LastRequestAt = time.Now().UTC().Format(time.RFC3339)
	s.mu.Unlock()
	return func() { s.mu.Lock(); s.value.ActiveRequests--; s.mu.Unlock() }
}
func (s *statisticsStore) failed(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.value.LastError = err.Error()
	s.mu.Unlock()
}
func (s *statisticsStore) snapshot() Statistics { s.mu.RLock(); defer s.mu.RUnlock(); return s.value }
