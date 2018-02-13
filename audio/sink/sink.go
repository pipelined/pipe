package sink

import (
	"context"

	"github.com/dudk/phono"
)

//Sinker is an interface for final stage in audio pipeline
type Sinker interface {
	Sink(ctx context.Context, in <-chan phono.Buffer) (errc <-chan error, err error)
}
