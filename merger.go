package pipe

import (
	"sync"
)

// errorMerger allows to listen to multiple error channels.
type errorMerger struct {
	wg        sync.WaitGroup
	errorChan chan error
}

// add error channels from all components into one.
func (m *errorMerger) add(errcList ...<-chan error) {
	// function to wait for error channel
	m.wg.Add(len(errcList))
	for _, ec := range errcList {
		go m.listen(ec)
	}
}

// listen blocks until error is received or channel is closed.
func (m *errorMerger) listen(ec <-chan error) {
	if err, ok := <-ec; ok {
		select {
		case m.errorChan <- err:
		default:
		}
	}
	m.wg.Done()
}

// start waits for all underlying error channels to be closed and then
// closes the output error channels.
func (m *errorMerger) start() {
	m.wg.Wait()
	close(m.errorChan)
}

// TODO: merge all errors
// TODO: distinguish context timeout error
func (m *errorMerger) await() {
	// wait until all groutines stop.
	for {
		// only the first error is propagated.
		if _, ok := <-m.errorChan; !ok {
			break
		}
	}
}
