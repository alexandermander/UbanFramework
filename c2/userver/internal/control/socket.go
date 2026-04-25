package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

type ControlRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type ControlResponse struct {
	OK    bool     `json:"ok"`
	Lines []string `json:"lines,omitempty"`
}

type Handler interface {
	HandleControlRequest(req ControlRequest) ControlResponse
}

func ListenSocket(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create control socket directory: %w", err)
	}

	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("control socket path is a directory: %s", path)
		}
		conn, dialErr := net.Dial("unix", path)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("control socket already in use: %s", path)
		}
		if removeErr := os.Remove(path); removeErr != nil {
			return nil, fmt.Errorf("failed to remove stale control socket: %w", removeErr)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to inspect control socket path: %w", err)
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if chmodErr := os.Chmod(path, 0o600); chmodErr != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("failed to secure control socket: %w", chmodErr)
	}
	return listener, nil
}

func HandleConn(conn net.Conn, handler Handler) {
	defer conn.Close()

	var req ControlRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(ControlResponse{
			OK:    false,
			Lines: []string{fmt.Sprintf("ERR invalid control request: %v", err)},
		})
		return
	}

	resp := handler.HandleControlRequest(req)
	_ = json.NewEncoder(conn).Encode(resp)
}
