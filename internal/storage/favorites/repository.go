package favorites

import (
	"sync"
)

type InMemoryRepository struct {
	mu      sync.RWMutex
	byUser  map[int64]map[int64]bool
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		byUser: make(map[int64]map[int64]bool),
	}
}

func (r *InMemoryRepository) Add(userID, adID int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.byUser[userID] == nil {
		r.byUser[userID] = make(map[int64]bool)
	}
	r.byUser[userID][adID] = true
	return r.countByAdLocked(adID), nil
}

func (r *InMemoryRepository) Remove(userID, adID int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.byUser[userID] != nil {
		delete(r.byUser[userID], adID)
	}
	return r.countByAdLocked(adID), nil
}

func (r *InMemoryRepository) ListAdIDs(userID int64) []int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	set := r.byUser[userID]
	res := make([]int64, 0, len(set))
	for adID := range set {
		res = append(res, adID)
	}
	return res
}

func (r *InMemoryRepository) CountByUser(userID int64) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byUser[userID])
}

func (r *InMemoryRepository) countByAdLocked(adID int64) int64 {
	var cnt int64
	for _, set := range r.byUser {
		if set[adID] {
			cnt++
		}
	}
	return cnt
}
