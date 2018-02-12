package pump

import (
	"context"

	"github.com/dudk/phono/audio"
)

//Pumper provides an interface for sources of samples
type Pumper interface {
	Pump(ctx context.Context) (out <-chan audio.Buffer, errc <-chan error, err error)
}
