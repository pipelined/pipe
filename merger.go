package pipe

import (
	"sync"
)

type (
	// errorMerger allows to listen to multiple error channels.
	errorMerger struct {
		wg        sync.WaitGroup
		errorChan chan error
	}
)

// add error channels from all components into one.
func (m *errorMerger) add(errc <-chan error) {
	// function to wait for error channel
	m.wg.Add(1)
	go m.listen(errc)
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

// wait waits for all underlying error channels to be closed and then
// closes the output error channels.
func (m *errorMerger) wait() {
	m.wg.Wait()
	close(m.errorChan)
}

// TODO: merge all errors
// TODO: distinguish context timeout error
func (m *errorMerger) drain() {
	// wait until all groutines stop.
	// only the first error is propagated.
	for range m.errorChan {
		break
	}
}
