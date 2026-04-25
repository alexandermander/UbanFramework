package protocol

import (
	"bytes"
	"io"
	"testing"
)

func TestBuildAndReadPacket(t *testing.T) {
	payload := []byte("hello")
	packet := BuildPacket(CmdSendText, payload)

	decoded, err := ReadPacket(bytes.NewReader(packet))
	if err != nil {
		t.Fatalf("ReadPacket returned error: %v", err)
	}
	if decoded.Command != CmdSendText {
		t.Fatalf("unexpected command: got %d", decoded.Command)
	}
	if string(decoded.Payload) != "hello" {
		t.Fatalf("unexpected payload: %q", decoded.Payload)
	}
}

func TestReadPacketEOFOnShortHeader(t *testing.T) {
	_, err := ReadPacket(bytes.NewReader([]byte{1, 2}))
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestReadPacketTruncatedPayload(t *testing.T) {
	data := []byte{byte(CmdEchoText), 0x04, 0x00, 'o', 'k'}
	_, err := ReadPacket(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected payload read error")
	}
}
