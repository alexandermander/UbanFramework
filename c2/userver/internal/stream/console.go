package stream

import (
	"fmt"
	"io"
	"sync"

	"userve/internal/device"
)

type ConsoleSink struct {
	mu  sync.Mutex
	out io.Writer
}

func NewConsoleSink(out io.Writer) *ConsoleSink {
	return &ConsoleSink{out: out}
}

func (s *ConsoleSink) Handle(event device.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch event.Type {
	case device.EventConnected:
		fmt.Fprintf(s.out, "[!] Connection #%d connected from %s\n", event.ID, event.Addr)
	case device.EventReady:
		fmt.Fprintf(s.out, "[+] Connection #%d ready: %s\n", event.ID, event.Message)
	case device.EventOutput:
		fmt.Fprintf(s.out, "[OUTPUT #%d] %s\n", event.ID, event.Message)
	case device.EventProtocolErr:
		fmt.Fprintf(s.out, "[ERR #%d] %v\n", event.ID, event.Err)
	case device.EventDisconnected:
		fmt.Fprintf(s.out, "[!] Connection #%d disconnected.\n", event.ID)
	}
}
