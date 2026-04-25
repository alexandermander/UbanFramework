package control

import (
	"net"
	"testing"

	"userve/internal/commands"
	"userve/internal/hub"
	"userve/internal/stream"
)

func TestStatusWithNoClient(t *testing.T) {
	service := NewService(hub.New(), commands.New(hub.New()), stream.NewTailSink(5), nil)
	resp := service.HandleControlRequest(ControlRequest{Command: "status"})
	if !resp.OK {
		t.Fatal("expected status to succeed")
	}
	if len(resp.Lines) != 1 || resp.Lines[0] != "No active connection." {
		t.Fatalf("unexpected status lines: %#v", resp.Lines)
	}
}

func TestStatusWithClient(t *testing.T) {
	h := hub.New()
	server, client := net.Pipe()
	defer client.Close()

	if err := h.Accept(server); err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	service := NewService(h, commands.New(h), stream.NewTailSink(5), nil)
	resp := service.HandleControlRequest(ControlRequest{Command: "status"})
	if !resp.OK {
		t.Fatal("expected status to succeed")
	}
	if len(resp.Lines) < 3 {
		t.Fatalf("unexpected status lines: %#v", resp.Lines)
	}
}
