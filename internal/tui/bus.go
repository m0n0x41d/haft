package tui

import tea "charm.land/bubbletea/v2"

// Bus carries events between the coordinator goroutine and the TUI.
type Bus struct {
	ch   chan tea.Msg
	done chan struct{}
}

// NewBus creates an event bus with a buffered channel.
func NewBus(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	return &Bus{
		ch:   make(chan tea.Msg, bufferSize),
		done: make(chan struct{}),
	}
}

// Send publishes an event to the bus.
// Returns false if the bus is closed (program exiting).
// Uses select to avoid blocking forever if TUI has stopped reading.
func (b *Bus) Send(msg tea.Msg) bool {
	select {
	case b.ch <- msg:
		return true
	case <-b.done:
		return false
	}
}

// Listen blocks until an event is available or the bus is closed.
func (b *Bus) Listen() tea.Msg {
	select {
	case msg, ok := <-b.ch:
		if !ok {
			return CoordinatorDoneMsg{}
		}
		return msg
	case <-b.done:
		return CoordinatorDoneMsg{}
	}
}

// Close signals all senders and listeners to stop.
func (b *Bus) Close() {
	select {
	case <-b.done:
		// already closed
	default:
		close(b.done)
	}
}
