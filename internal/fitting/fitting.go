package fitting

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

	// New is a func that returns new fitting.
	New func() Fitting

	// Fitting implements both Sender and Receiver. It's used to conenct
	// two pipe components.
	Fitting interface {
		Sender
		Receiver
	}

	syncFitting struct {
		closed  bool
		message Message
	}

	asyncFitting struct {
		messageChan chan Message
	}
)

// Sync is a fitting that connects two components executed in the same
// goroutine.
func Sync() Fitting {
	return &syncFitting{}
}

// Async is a fitting that connects two components executed in different
// goroutines.
func Async() Fitting {
	return &asyncFitting{
		messageChan: make(chan Message, 1),
	}
}

func (l *syncFitting) Send(_ context.Context, m Message) bool {
	if l.closed {
		return false
	}
	l.message = m
	return true
}

func (l *syncFitting) Receive(context.Context) (Message, bool) {
	if l.closed {
		return l.message, false
	}
	return l.message, true
}

func (l *syncFitting) Close() {
	l.closed = true
	return
}

func (l *asyncFitting) Send(ctx context.Context, m Message) bool {
	select {
	case <-ctx.Done():
		return false
	case l.messageChan <- m:
		return true
	}
}

func (l *asyncFitting) Receive(ctx context.Context) (Message, bool) {
	var (
		m  Message
		ok bool
	)
	select {
	case <-ctx.Done():
	case m, ok = <-l.messageChan:
	}
	return m, ok
}

func (l *asyncFitting) Close() {
	close(l.messageChan)
	return
}
