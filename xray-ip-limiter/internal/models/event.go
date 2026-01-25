package models

import "time"

type UserIPEvent struct {
	UserID    string    `json:"user_id"`
	IP        string    `json:"ip"`
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
}

type BlockCommand struct {
	UserID     string    `json:"user_id"`
	BlockedIPs []string  `json:"blocked_ips"`
	Action     string    `json:"action"`
	Duration   int       `json:"duration"`
	Timestamp  time.Time `json:"timestamp"`
}
