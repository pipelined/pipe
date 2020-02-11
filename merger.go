package pipe

import "sync"

type merger struct {
	wg     sync.WaitGroup
	errors chan error
}

// merge error channels from all components into one.
func (m *merger) merge(errcList []<-chan error) {
	//function to wait for error channel
	m.wg.Add(len(errcList))
	for _, ec := range errcList {
		go m.done(ec)
	}

	//wait and close out
	go func() {
		m.wg.Wait()
		close(m.errors)
	}()
}

// done blocks until error is received or channel is closed.
func (m *merger) done(ec <-chan error) {
	if err, ok := <-ec; ok {
		select {
		case m.errors <- err:
		default:
		}
	}
	m.wg.Done()
}
