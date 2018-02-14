package processor

import (
	"context"

	"github.com/dudk/phono"
)

//Processor defines interface for pipe-prcessors
type Processor interface {
	Process(ctx context.Context, in <-chan phono.Buffer) (out chan<- phono.Buffer, errc <-chan error, err error)
}
