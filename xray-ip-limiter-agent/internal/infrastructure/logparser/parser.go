package logparser

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"xray-ip-limiter-agent/internal/domain"

	"github.com/nxadm/tail"
)

var _ domain.LogParser = (*TailParser)(nil)

var logRegex = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d+) from (?:tcp:)?(\d+\.\d+\.\d+\.\d+):\d+ accepted .* email: (.+)$`)

type TailParser struct {
	logPath string
	nodeID  string
	tail    *tail.Tail
	stopCh  chan struct{}
}

func NewTailParser(logPath, nodeID string) *TailParser {
	return &TailParser{
		logPath: logPath,
		nodeID:  nodeID,
		stopCh:  make(chan struct{}),
	}
}

func (p *TailParser) Start(ctx context.Context, handler func(domain.LogEntry)) error {
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

func (p *TailParser) Stop() {
	if p.tail != nil {
		p.tail.Stop()
	}
	close(p.stopCh)
}

func (p *TailParser) parseLine(line string) (*domain.LogEntry, error) {
	matches := logRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("no match")
	}

	timestamp, _ := time.Parse("2006/01/02 15:04:05", matches[1][:19])

	return &domain.LogEntry{
		Timestamp: timestamp,
		IP:        matches[2],
		Email:     strings.TrimSpace(matches[3]),
		NodeID:    p.nodeID,
	}, nil
}

func (p *TailParser) isLocalIP(ip string) bool {
	return strings.HasPrefix(ip, "127.") || ip == "::1" || ip == "0.0.0.0"
}
