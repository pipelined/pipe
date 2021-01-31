package runtime

import (
	"context"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type (
	// Message is a main structure for pipe transport
	Message struct {
		Signal            signal.Floating // Buffer of message.
		mutable.Mutations                 // Mutators for pipe.
	}

	// Sender sends the message.
	Sender interface {
		Send(context.Context, Message) bool
		Close()
	}

	// Receiver receives the message.
	Receiver interface {
		Receive(context.Context) (Message, bool)
	}

	// Link implements both Sender and Receiver. It's used to link two pipe
	// components.
	Link interface {
		Sender
		Receiver
	}

	syncLink struct {
		closed  bool
		message Message
	}

	asyncLink chan Message
)

// SyncLink is a link that connects two components executed in the same
// goroutine.
func SyncLink() Link {
	return &syncLink{}
}

// AsyncLink is a link that connects two components executed in different
// goroutines.
func AsyncLink() Link {
	return asyncLink(make(chan Message, 1))
}

func (l *syncLink) Send(_ context.Context, m Message) bool {
	if l.closed {
		return false
	}
	l.message = m
	return true
}

func (l *syncLink) Receive(context.Context) (Message, bool) {
	if l.closed {
		return l.message, false
	}
	return l.message, true
}

func (l *syncLink) Close() {
	l.closed = true
	return
}

func (l asyncLink) Send(ctx context.Context, m Message) bool {
	select {
	case <-ctx.Done():
		return false
	case l <- m:
		return true
	}
}

func (l asyncLink) Receive(ctx context.Context) (Message, bool) {
	var (
		m  Message
		ok bool
	)
	select {
	case <-ctx.Done():
	case m, ok = <-l:
	}
	return m, ok
}

func (l asyncLink) Close() {
	close(l)
	return
}