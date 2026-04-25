package device

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"userve/internal/protocol"
)

type EventType string

const (
	EventConnected    EventType = "connected"
	EventReady        EventType = "ready"
	EventOutput       EventType = "output"
	EventDisconnected EventType = "disconnected"
	EventProtocolErr  EventType = "protocol_error"
)

type Event struct {
	Type    EventType
	ID      int
	Addr    string
	Message string
	Err     error
}

type Client struct {
	id      int
	conn    net.Conn
	addr    string
	writeCh chan []byte
	done    chan struct{}
	once    sync.Once
	onEvent func(Event)
}

func New(id int, conn net.Conn) *Client {
	return &Client{
		id:      id,
		conn:    conn,
		addr:    conn.RemoteAddr().String(),
		writeCh: make(chan []byte, 16),
		done:    make(chan struct{}),
	}
}

func (c *Client) ID() int {
	return c.id
}

func (c *Client) Addr() string {
	return c.addr
}

func (c *Client) Start(onEvent func(Event)) {
	c.onEvent = onEvent
	onEvent(Event{Type: EventConnected, ID: c.id, Addr: c.addr})
	go c.readLoop(onEvent)
	go c.writeLoop(onEvent)
}

func (c *Client) Send(command protocol.Command, payload []byte) error {
	packet := protocol.BuildPacket(command, payload)
	select {
	case <-c.done:
		return errors.New("device is disconnected")
	case c.writeCh <- packet:
		return nil
	}
}

func (c *Client) Close() {
	c.close(nil)
}

func (c *Client) readLoop(onEvent func(Event)) {
	for {
		packet, err := protocol.ReadPacket(c.conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				c.close(nil)
			} else {
				c.close(err)
			}
			return
		}

		switch packet.Command {
		case protocol.CmdConnectSession:
			onEvent(Event{
				Type:    EventReady,
				ID:      c.id,
				Addr:    c.addr,
				Message: protocol.DecodeRemoteTextPayload(packet.Payload),
			})
		case protocol.CmdOutputText:
			onEvent(Event{
				Type:    EventOutput,
				ID:      c.id,
				Addr:    c.addr,
				Message: protocol.DecodeRemoteTextPayload(packet.Payload),
			})
		case protocol.CmdDisconnectSession:
			c.close(nil)
			return
		default:
			onEvent(Event{
				Type: EventProtocolErr,
				ID:   c.id,
				Addr: c.addr,
				Err:  fmt.Errorf("unexpected packet command %d", packet.Command),
			})
		}
	}
}

func (c *Client) writeLoop(onEvent func(Event)) {
	for {
		select {
		case <-c.done:
			return
		case packet := <-c.writeCh:
			if _, err := c.conn.Write(packet); err != nil {
				onEvent(Event{
					Type: EventProtocolErr,
					ID:   c.id,
					Addr: c.addr,
					Err:  err,
				})
				c.close(err)
				return
			}
		}
	}
}

func (c *Client) close(reason error) {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
		if c.onEvent != nil {
			c.onEvent(Event{
				Type: EventDisconnected,
				ID:   c.id,
				Addr: c.addr,
				Err:  reason,
			})
		}
	})
}
