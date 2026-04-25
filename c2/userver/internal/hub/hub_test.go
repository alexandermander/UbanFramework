package hub

import (
	"net"
	"testing"
)

func TestHubRejectsSecondClient(t *testing.T) {
	h := New()
	serverA, clientA := net.Pipe()
	defer clientA.Close()
	defer serverA.Close()

	if err := h.Accept(serverA); err != nil {
		t.Fatalf("first accept failed: %v", err)
	}

	serverB, clientB := net.Pipe()
	defer clientB.Close()
	defer serverB.Close()

	if err := h.Accept(serverB); err != ErrClientAlreadyConnected {
		t.Fatalf("expected ErrClientAlreadyConnected, got %v", err)
	}
}

func TestHubActiveSnapshotEmpty(t *testing.T) {
	h := New()
	if _, ok := h.ActiveSnapshot(); ok {
		t.Fatal("expected no active snapshot")
	}
}
