package usecase

import "time"

type IPEventInput struct {
	UserID    string    `json:"user_id"`
	IP        string    `json:"ip"`
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
}

type BlockCommandOutput struct {
	UserID     string    `json:"user_id"`
	BlockedIPs []string  `json:"blocked_ips"`
	Action     string    `json:"action"`
	Duration   int       `json:"duration"`
	Timestamp  time.Time `json:"timestamp"`
}
