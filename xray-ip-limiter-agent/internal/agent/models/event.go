package models

import "time"

type LogEntry struct {
	IP        string    `json:"ip"`
	Email     string    `json:"user_id"`
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
