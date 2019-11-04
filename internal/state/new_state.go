package state

import "context"

type (
	state struct {
		stateType
		events   <-chan event
		errors   <-chan error
		messages <-chan string
		params   <-chan Params
	}
)

type stateType uint

const (
	ready stateType = iota + 1
	running
	paused
	stopped
)

func (h *Handle) ready() state {
	return state{
		stateType: ready,
		events:    h.events,
		params:    h.params,
	}
}

func (h *Handle) running() state {
	return state{
		stateType: running,
		events:    h.events,
		params:    h.params,
		messages:  h.messages,
		errors:    h.merger.errors,
	}
}

func (h *Handle) paused() state {
	return state{
		stateType: paused,
		events:    h.events,
		params:    h.params,
		errors:    h.merger.errors,
	}
}

func (h *Handle) transition(s state, e event) (state, error) {
	switch s.stateType {
	case ready:
		switch ev := e.(type) {
		case stop:
			return state{
				stateType: stopped,
				events:    s.events,
			}, nil
		case run:
			ctx, cancelFn := context.WithCancel(ev.Context)
			h.cancelFn = cancelFn
			h.merger = mergeErrors(h.startFn(ev.BufferSize, ctx.Done(), h.messages))
			return h.running(), nil
		}
	case running:
		switch e.(type) {
		case stop:
			return state{
				stateType: stopped,
			}, h.cancel()
		case pause:
			return h.paused(), nil
		}
	case paused:
		switch e.(type) {
		case stop:
			return state{
				stateType: stopped,
			}, h.cancel()
		case resume:
			return h.running(), nil
		}
	}
	return s, ErrInvalidState
}

func (s stateType) String() string {
	switch s {
	case ready:
		return "state.Ready"
	case running:
		return "state.Running"
	case paused:
		return "state.Paused"
	default:
		return "state.Unknown"
	}
}
