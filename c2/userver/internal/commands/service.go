package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"userve/internal/hub"
	"userve/internal/protocol"
)

type activeProvider interface {
	ActiveSender() (hub.Sender, int, bool)
}

type Service struct {
	hub activeProvider
}

func New(hub activeProvider) *Service {
	return &Service{hub: hub}
}

func (s *Service) SendRaw(text string) (bool, string) {
	return s.sendText(protocol.CmdSendText, text)
}

func (s *Service) Echo(text string) (bool, string) {
	return s.sendText(protocol.CmdEchoText, text)
}

func (s *Service) GetApps() (bool, string) {
	return s.send(protocol.CmdGetApps, nil, "OK requested app list")
}

func (s *Service) Disconnect() (bool, string) {
	return s.send(protocol.CmdDisconnectSession, nil, "OK disconnect requested")
}

func (s *Service) ExecuteUploaded() (bool, string) {
	return s.send(protocol.CmdExecApp, nil, "OK execute requested")
}

func (s *Service) Push(path string) (bool, string) {
	filename := filepath.Base(path)
	if filename == "" {
		return false, "ERR missing filename"
	}
	if !protocol.IsASCII(filename) {
		return false, "ERR filename must be ASCII"
	}
	if len(filename) > 255 {
		return false, "ERR filename too long"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Sprintf("ERR failed to read %s: %v", path, err)
	}

	payload := make([]byte, 1+len(filename)+len(data))
	payload[0] = byte(len(filename))
	copy(payload[1:], []byte(filename))
	copy(payload[1+len(filename):], data)

	ok, response := s.send(protocol.CmdPushFile, payload, fmt.Sprintf("Pushed %s to active EFI system.", filename))
	if !ok {
		return false, response
	}
	return true, response
}

func (s *Service) sendText(command protocol.Command, text string) (bool, string) {
	if text == "" {
		return false, "ERR empty command"
	}
	if !protocol.IsASCII(text) {
		return false, "ERR commands must be ASCII"
	}
	return s.send(command, []byte(text), fmt.Sprintf("OK sent command %d", command))
}

func (s *Service) send(command protocol.Command, payload []byte, success string) (bool, string) {
	active, _, ok := s.hub.ActiveSender()
	if !ok {
		return false, "ERR no active connection"
	}
	if err := active.Send(command, payload); err != nil {
		return false, fmt.Sprintf("ERR failed to send command: %v", err)
	}
	return true, success
}
