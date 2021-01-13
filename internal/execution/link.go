package execution

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

	Sender interface {
		Send(context.Context, Message) bool
		Close()
	}

	Receiver interface {
		Receive(context.Context) (Message, bool)
	}

	Link interface {
		Sender
		Receiver
	}

	syncLink struct {
		message Message
	}

	asyncLink chan Message
)

func SyncLink() Link {
	return &syncLink{}
}

func AsyncLink() Link {
	return asyncLink(make(chan Message, 1))
}

func (l *syncLink) Send(_ context.Context, m Message) bool {
	l.message = m
	return true
}

func (l *syncLink) Receive(context.Context) (Message, bool) {
	return l.message, true
}

func (l *syncLink) Close() {
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
