package parser

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"xray-ip-limiter-agent/internal/agent/models"

	"github.com/nxadm/tail"
)

var logRegex = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d+) from (?:tcp:)?(\d+\.\d+\.\d+\.\d+):\d+ accepted .* email: (.+)$`)

type LogParser struct {
	logPath string
	nodeID  string
	tail    *tail.Tail
	stopCh  chan struct{}
}

func NewLogParser(logPath, nodeID string) *LogParser {
	return &LogParser{
		logPath: logPath,
		nodeID:  nodeID,
		stopCh:  make(chan struct{}),
	}
}

func (p *LogParser) Start(ctx context.Context, handler func(models.LogEntry)) error {
	t, err := tail.TailFile(p.logPath, tail.Config{
		Follow:   true,
		ReOpen:   true,
		Poll:     true,
		Location: &tail.SeekInfo{Offset: 0, Whence: 2},
	})
	if err != nil {
		return fmt.Errorf("failed to tail log file: %w", err)
	}

	p.tail = t

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.stopCh:
			return nil
		case line, ok := <-t.Lines:
			if !ok {
				return nil
			}
			if line.Err != nil {
				continue
			}

			entry, err := p.parseLine(line.Text)
			if err != nil {
				continue
			}

			if p.isLocalIP(entry.IP) {
				continue
			}

			handler(*entry)
		}
	}
}

func (p *LogParser) parseLine(line string) (*models.LogEntry, error) {
	matches := logRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("no match")
	}

	timestamp, _ := time.Parse("2006/01/02 15:04:05", matches[1][:19])

	return &models.LogEntry{
		Timestamp: timestamp,
		IP:        matches[2],
		Email:     strings.TrimSpace(matches[3]),
		NodeID:    p.nodeID,
	}, nil
}

func (p *LogParser) isLocalIP(ip string) bool {
	return strings.HasPrefix(ip, "127.") || ip == "::1" || ip == "0.0.0.0"
}

func (p *LogParser) Stop() {
	if p.tail != nil {
		p.tail.Stop()
	}
	close(p.stopCh)
}
