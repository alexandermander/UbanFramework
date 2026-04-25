package stream

import (
	"sync"

	"userve/internal/device"
)

type TailSink struct {
	mu    sync.Mutex
	limit int
	lines []string
}

func NewTailSink(limit int) *TailSink {
	return &TailSink{
		limit: limit,
		lines: make([]string, 0, limit),
	}
}

func (s *TailSink) Handle(event device.Event) {
	if event.Type != device.EventOutput {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.limit > 0 && len(s.lines) == s.limit {
		copy(s.lines, s.lines[1:])
		s.lines = s.lines[:s.limit-1]
	}
	s.lines = append(s.lines, event.Message)
}

func (s *TailSink) Lines(limit int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit == 0 || limit > len(s.lines) {
		limit = len(s.lines)
	}
	start := len(s.lines) - limit
	out := make([]string, limit)
	copy(out, s.lines[start:])
	return out
}
