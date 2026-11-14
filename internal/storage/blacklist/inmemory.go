package blacklist

import (
	"context"
	"sync"
	"time"
)

type InMemory struct {
	mu              sync.RWMutex
	tokens          map[string]time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	cleanupInterval time.Duration
	sweeperDone     chan struct{}
}

func New(cleanupInterval time.Duration) *InMemory {
	ctx, cancel := context.WithCancel(context.Background())

	bl := &InMemory{
		tokens:          make(map[string]time.Time),
		ctx:             ctx,
		cancel:          cancel,
		cleanupInterval: cleanupInterval,
		sweeperDone:     make(chan struct{}),
	}

	go bl.startSweeper()

	return bl
}

func (s *InMemory) Add(jti string, exp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[jti] = exp
}

func (s *InMemory) Check(jti string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.tokens[jti]
	return exists
}

func (s *InMemory) startSweeper() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer func() {
		ticker.Stop()
		close(s.sweeperDone)
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *InMemory) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for jti, exp := range s.tokens {
		if now.After(exp) {
			delete(s.tokens, jti)
		}
	}
}

func (s *InMemory) Stop() {
	s.cancel()
	<-s.sweeperDone
}
