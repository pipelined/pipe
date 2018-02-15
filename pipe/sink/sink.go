package sink

import (
	"context"

	"github.com/dudk/phono"
)

//Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink(ctx context.Context, in <-chan phono.Message) (errc <-chan error, err error)
}
