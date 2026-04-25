package commands

import (
	"os"
	"path/filepath"
	"testing"

	"userve/internal/hub"
	"userve/internal/protocol"
)

type stubSender struct {
	command protocol.Command
	payload []byte
	err     error
}

func (s *stubSender) Send(command protocol.Command, payload []byte) error {
	s.command = command
	s.payload = append([]byte(nil), payload...)
	return s.err
}

type stubHub struct {
	sender hub.Sender
	ok     bool
}

func (h stubHub) ActiveSender() (hub.Sender, int, bool) {
	return h.sender, 1, h.ok
}

func TestEchoUsesEchoCommand(t *testing.T) {
	stub := &stubSender{}
	service := New(stubHub{sender: stub, ok: true})

	ok, _ := service.Echo("test")
	if !ok {
		t.Fatal("expected echo to succeed")
	}
	if stub.command != protocol.CmdEchoText {
		t.Fatalf("expected echo command %d, got %d", protocol.CmdEchoText, stub.command)
	}
}

func TestRawUsesSendTextCommand(t *testing.T) {
	stub := &stubSender{}
	service := New(stubHub{sender: stub, ok: true})

	ok, _ := service.SendRaw("help")
	if !ok {
		t.Fatal("expected raw to succeed")
	}
	if stub.command != protocol.CmdSendText {
		t.Fatalf("expected raw command %d, got %d", protocol.CmdSendText, stub.command)
	}
}

func TestExecuteUploadedUsesExecCommand(t *testing.T) {
	stub := &stubSender{}
	service := New(stubHub{sender: stub, ok: true})

	ok, _ := service.ExecuteUploaded()
	if !ok {
		t.Fatal("expected execute to succeed")
	}
	if stub.command != protocol.CmdExecApp {
		t.Fatalf("expected exec command %d, got %d", protocol.CmdExecApp, stub.command)
	}
}

func TestPushUsesPushFileCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.efi")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	stub := &stubSender{}
	service := New(stubHub{sender: stub, ok: true})

	ok, _ := service.Push(path)
	if !ok {
		t.Fatal("expected push to succeed")
	}
	if stub.command != protocol.CmdPushFile {
		t.Fatalf("expected push command %d, got %d", protocol.CmdPushFile, stub.command)
	}
}
