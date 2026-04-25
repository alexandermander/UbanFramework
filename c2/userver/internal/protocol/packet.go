package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const HeaderSize = 3

type Command byte

const (
	CmdSendText          Command = 1
	CmdGetApps           Command = 2
	CmdConnectSession    Command = 3
	CmdOutputText        Command = 4
	CmdDisconnectSession Command = 5
	CmdPushFile          Command = 6
	CmdExecApp           Command = 7
	CmdEchoText          Command = 8
)

type Packet struct {
	Command Command
	Payload []byte
}

func ReadPacket(r io.Reader) (*Packet, error) {
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, io.EOF
		}
		return nil, err
	}

	payloadLength := int(binary.LittleEndian.Uint16(header[1:]))
	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(r, payload); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("connection closed during payload read, expected %d bytes", payloadLength)
		}
		return nil, err
	}

	return &Packet{
		Command: Command(header[0]),
		Payload: payload,
	}, nil
}

func BuildPacket(command Command, payload []byte) []byte {
	data := make([]byte, HeaderSize+len(payload))
	data[0] = byte(command)
	binary.LittleEndian.PutUint16(data[1:HeaderSize], uint16(len(payload)))
	copy(data[HeaderSize:], payload)
	return data
}
