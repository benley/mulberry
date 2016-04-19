package daemon

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/chronos-tachyon/mulberry/config"
)

type socketpair struct {
	wg sync.WaitGroup
	mu sync.Mutex
	pn string
	t  config.Address
	x  net.Conn
	y  net.Conn
	st bool
}

func newSocketPair(portName string, x net.Conn, target config.Address) *socketpair {
	sp := &socketpair{
		pn: portName,
		t:  target,
		x:  x,
	}
	sp.wg.Add(1)
	go sp.loop()
	return sp
}

func (sp *socketpair) IsRunning() bool {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.st {
		return false
	}
	_, err := sp.x.Write(nil)
	return err == nil
}

func (sp *socketpair) Close() error {
	withLock(&sp.mu, func() {
		sp.st = true
		closeSocket("origin", sp.x)
		closeSocket("destination", sp.y)
	})
	sp.wg.Wait()
	return nil
}

func (sp *socketpair) loop() {
	defer sp.wg.Done()

	y, err := net.Dial(sp.t.Net, sp.t.Addr)
	if err != nil {
		log.Printf("error: failed to dial: %v", err)
		closeSocket("origin", sp.x)
		dialErrorsTotal.WithLabelValues(sp.pn).Inc()
		return
	}
	connectsTotal.WithLabelValues(sp.pn).Inc()

	var st bool
	withLock(&sp.mu, func() {
		sp.y = y
		if sp.st {
			st = true
			closeSocket("origin", sp.x)
			closeSocket("destination", sp.y)
		}
	})
	if st {
		return
	}

	liveConnectionsTotal.WithLabelValues(sp.pn).Inc()
	sp.wg.Add(2)
	go loopOneDirection(sp.x, sp.y, &sp.wg)
	loopOneDirection(sp.y, sp.x, &sp.wg)
	closeSocket("origin", sp.x)
	closeSocket("destination", sp.y)
	liveConnectionsTotal.WithLabelValues(sp.pn).Dec()
}

func loopOneDirection(in net.Conn, out net.Conn, w *sync.WaitGroup) {
	defer w.Done()

	b := make([]byte, 65536)
	for {
		n, err := in.Read(b)
		if err != nil {
			operr, ok := err.(*net.OpError)
			switch {
			case err == io.EOF:
				// pass
			case ok && operr.Op == "read" && operr.Err.Error() == "use of closed network connection":
				// pass
			default:
				log.Printf("error: failed to read from socket: %v", err)
			}
			break
		}
		_, err = out.Write(b[:n])
		if err != nil {
			log.Printf("error: failed to write to socket: %v", err)
			break
		}
	}
}
