package pump

import (
	"context"

	"github.com/dudk/phono"
)

//Pumper provides an interface for sources of samples
type Pumper interface {
	Pump(ctx context.Context) (out <-chan phono.Buffer, errc <-chan error, err error)
}
