package pipe

import "sync"

type merger struct {
	wg        sync.WaitGroup
	errorChan chan error
}

// merge error channels from all components into one.
func (m *merger) merge(errcList ...<-chan error) {
	// function to wait for error channel
	m.wg.Add(len(errcList))
	for _, ec := range errcList {
		go m.done(ec)
	}
}

func (m *merger) wait() {
	// wait and close out
	m.wg.Wait()
	close(m.errorChan)
}

// done blocks until error is received or channel is closed.
func (m *merger) done(ec <-chan error) {
	if err, ok := <-ec; ok {
		select {
		case m.errorChan <- err:
		default:
		}
	}
	m.wg.Done()
}
