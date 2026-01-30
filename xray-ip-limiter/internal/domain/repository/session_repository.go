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

type ViolationRepository interface {
	SetViolationStart(ctx context.Context, userID string, ttl time.Duration) (alreadyExists bool, err error)
	GetViolationStart(ctx context.Context, userID string) (startTime time.Time, exists bool, err error)
	ClearViolationStart(ctx context.Context, userID string) error
}
