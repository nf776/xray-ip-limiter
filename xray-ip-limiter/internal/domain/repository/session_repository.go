package repository

import (
	"context"
	"time"
)

type SessionRepository interface {
	AddUserIP(ctx context.Context, userID, ip string, window time.Duration) (uniqueIPCount int, err error)
	GetUserIPs(ctx context.Context, userID string) ([]string, error)
}

type LockRepository interface {
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (acquired bool, err error)
	ReleaseLock(ctx context.Context, key string) error
}
