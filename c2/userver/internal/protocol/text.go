package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf16"
)

func DecodeRemoteTextPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}

	if text, ok := decodeUTF16Payload(payload); ok {
		return text
	}

	if IsASCIIBytes(payload) {
		return string(payload)
	}

	return "hex:" + strings.ToUpper(hex.EncodeToString(payload))
}

func IsASCII(text string) bool {
	for _, r := range text {
		if r > 127 {
			return false
		}
	}
	return true
}

func IsASCIIBytes(payload []byte) bool {
	for _, b := range payload {
		if b > 127 {
			return false
		}
	}
	return true
}

func decodeUTF16Payload(payload []byte) (string, bool) {
	if len(payload)%2 != 0 {
		return "", false
	}
	if !looksLikeUTF16(payload) {
		return "", false
	}

	words := make([]uint16, len(payload)/2)
	for i := 0; i < len(words); i++ {
		words[i] = binary.LittleEndian.Uint16(payload[i*2:])
	}

	text := string(utf16.Decode(words))
	if !looksReadableText(text) {
		return "", false
	}

	return text, true
}

func looksLikeUTF16(payload []byte) bool {
	for i := 1; i < len(payload); i += 2 {
		if payload[i] == 0 {
			return true
		}
	}
	return false
}

func looksReadableText(text string) bool {
	if text == "" {
		return true
	}

	for _, r := range text {
		switch {
		case r == '\n', r == '\r', r == '\t':
			continue
		case unicode.IsPrint(r):
			continue
		default:
			return false
		}
	}

	return true
}
