package protocol

import "testing"

func TestDecodeRemoteTextPayloadASCII(t *testing.T) {
	if got := DecodeRemoteTextPayload([]byte("status")); got != "status" {
		t.Fatalf("unexpected decoded text: %q", got)
	}
}

func TestDecodeRemoteTextPayloadUTF16(t *testing.T) {
	payload := []byte{'O', 0x00, 'K', 0x00}
	if got := DecodeRemoteTextPayload(payload); got != "OK" {
		t.Fatalf("unexpected decoded text: %q", got)
	}
}

func TestDecodeRemoteTextPayloadHexFallback(t *testing.T) {
	if got := DecodeRemoteTextPayload([]byte{0xFF, 0x00, 0xAA}); got != "hex:FF00AA" {
		t.Fatalf("unexpected fallback text: %q", got)
	}
}
