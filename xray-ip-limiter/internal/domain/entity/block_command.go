package entity

import (
	"errors"
	"time"
)

var (
	ErrNoIPsToBlock       = errors.New("no IP addresses to block")
	ErrInvalidDuration    = errors.New("block duration must be positive")
	ErrEmptyUserIDInBlock = errors.New("user id cannot be empty in block command")
)

type BlockCommand struct {
	userID     string
	blockedIPs []string
	duration   time.Duration
	createdAt  time.Time
}

func NewBlockCommand(userID string, ips []string, duration time.Duration) (*BlockCommand, error) {
	if userID == "" {
		return nil, ErrEmptyUserIDInBlock
	}

	if len(ips) == 0 {
		return nil, ErrNoIPsToBlock
	}

	if duration <= 0 {
		return nil, ErrInvalidDuration
	}

	ipsCopy := make([]string, len(ips))
	copy(ipsCopy, ips)

	return &BlockCommand{
		userID:     userID,
		blockedIPs: ipsCopy,
		duration:   duration,
		createdAt:  time.Now(),
	}, nil
}

func (b *BlockCommand) UserID() string {
	return b.userID
}

func (b *BlockCommand) BlockedIPs() []string {
	ipsCopy := make([]string, len(b.blockedIPs))
	copy(ipsCopy, b.blockedIPs)
	return ipsCopy
}

func (b *BlockCommand) Duration() time.Duration {
	return b.duration
}

func (b *BlockCommand) DurationSeconds() int {
	return int(b.duration.Seconds())
}

func (b *BlockCommand) CreatedAt() time.Time {
	return b.createdAt
}

func (b *BlockCommand) Equals(other *BlockCommand) bool {
	if other == nil {
		return false
	}

	if b.userID != other.userID || b.duration != other.duration {
		return false
	}

	if len(b.blockedIPs) != len(other.blockedIPs) {
		return false
	}

	ipsMap := make(map[string]bool)
	for _, ip := range b.blockedIPs {
		ipsMap[ip] = true
	}

	for _, ip := range other.blockedIPs {
		if !ipsMap[ip] {
			return false
		}
	}

	return true
}
