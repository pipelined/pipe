package sink

import (
	"context"

	"github.com/dudk/phono/audio"
)

//Sinker is an interface for final stage in audio pipeline
type Sinker interface {
	Sink(ctx context.Context, in <-chan audio.Buffer) (errc <-chan error, err error)
}
