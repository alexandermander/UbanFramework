package hub

import (
	"errors"
	"net"
	"slices"
	"sync"

	"userve/internal/device"
	"userve/internal/protocol"
)

var ErrClientAlreadyConnected = errors.New("an EFI client is already connected")

type Snapshot struct {
	ID             int
	Addr           string
	Ready          bool
	ConnectMessage string
	LastError      string
	Connected      bool
	Active         bool
}

type Sender interface {
	Send(command protocol.Command, payload []byte) error
}

type Hub struct {
	mu          sync.RWMutex
	nextID      int
	client      *device.Client
	state       Snapshot
	subscribers map[int]func(device.Event)
	nextSubID   int
}

func New() *Hub {
	return &Hub{
		subscribers: make(map[int]func(device.Event)),
	}
}

func (h *Hub) Accept(conn net.Conn) error {
	h.mu.Lock()

	if h.client != nil {
		h.mu.Unlock()
		return ErrClientAlreadyConnected
	}

	h.nextID++
	client := device.New(h.nextID, conn)
	h.client = client
	h.state = Snapshot{
		ID:        client.ID(),
		Addr:      client.Addr(),
		Connected: true,
		Active:    true,
	}
	h.mu.Unlock()
	client.Start(h.handleEvent)
	return nil
}

func (h *Hub) Subscribe(fn func(device.Event)) func() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextSubID++
	id := h.nextSubID
	h.subscribers[id] = fn
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subscribers, id)
	}
}

func (h *Hub) ActiveSnapshot() (Snapshot, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.client == nil {
		return Snapshot{}, false
	}
	return h.state, true
}

func (h *Hub) Snapshots() []Snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.client == nil {
		return nil
	}
	return []Snapshot{h.state}
}

func (h *Hub) SetActive(id int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.client != nil && h.state.ID == id
}

func (h *Hub) ActiveSender() (Sender, int, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.client == nil {
		return nil, 0, false
	}
	return h.client, h.state.ID, true
}

func (h *Hub) Close() {
	h.mu.Lock()
	client := h.client
	h.mu.Unlock()

	if client != nil {
		client.Close()
	}
}

func (h *Hub) handleEvent(event device.Event) {
	h.mu.Lock()
	switch event.Type {
	case device.EventReady:
		h.state.Ready = true
		h.state.ConnectMessage = event.Message
	case device.EventProtocolErr:
		if event.Err != nil {
			h.state.LastError = event.Err.Error()
		}
	case device.EventDisconnected:
		h.state.Connected = false
		h.client = nil
	case device.EventConnected:
		h.state.Connected = true
	}
	subscribers := h.subscriberListLocked()
	clearClient := false
	if event.Type == device.EventDisconnected {
		clearClient = true
	}
	h.mu.Unlock()

	for _, subscriber := range subscribers {
		subscriber(event)
	}

	if clearClient {
		h.mu.Lock()
		h.client = nil
		h.state = Snapshot{}
		h.mu.Unlock()
	}
}

func (h *Hub) subscriberListLocked() []func(device.Event) {
	ids := make([]int, 0, len(h.subscribers))
	for id := range h.subscribers {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	list := make([]func(device.Event), 0, len(ids))
	for _, id := range ids {
		list = append(list, h.subscribers[id])
	}
	return list
}
