package persistence

import (
	"sync"
	"xray-ip-limiter/internal/domain/repository"
)

var _ repository.LimitsRepository = (*MemoryLimitRepository)(nil)

type MemoryLimitRepository struct {
	mu     sync.RWMutex
	limits map[int]int
}

func NewMemoryLimitRepository() *MemoryLimitRepository {
	return &MemoryLimitRepository{limits: make(map[int]int)}
}

func (r *MemoryLimitRepository) GetLimit(userID int) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit, ok := r.limits[userID]
	return limit, ok
}

func (r *MemoryLimitRepository) UpdateLimits(newLimits map[int]int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limits = newLimits
}
