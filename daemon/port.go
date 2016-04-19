package daemon

import (
	"log"
	"net"
	"sync"

	"github.com/chronos-tachyon/mulberry/config"
)

type listenport struct {
	wg sync.WaitGroup
	mu sync.Mutex
	pn string
	l  net.Listener
	t  config.Address
	sp []*socketpair
}

func newListenPort(portName string, l net.Listener, c config.Address) *listenport {
	p := &listenport{
		pn: portName,
		l:  l,
		t:  c,
	}
	p.wg.Add(1)
	go p.loop()
	return p
}

func (p *listenport) Alter(c config.Address) {
	p.mu.Lock()
	splist := p.sp
	p.sp = nil
	p.t = c
	p.mu.Unlock()
	for _, sp := range splist {
		sp.Close()
	}
}

func (p *listenport) Close() error {
	withLock(&p.mu, func() {
		closeSocket("listener", p.l)
	})
	p.wg.Wait()
	return nil
}

func (p *listenport) loop() {
	defer p.wg.Done()

	for {
		x, err := p.l.Accept()
		if err != nil {
			operr, ok := err.(*net.OpError)
			if !ok || operr.Op != "accept" || operr.Err.Error() != "use of closed network connection" {
				log.Printf("error: failed to accept: %v", err)
			}
			break
		}
		acceptsTotal.WithLabelValues(p.pn).Inc()

		withLock(&p.mu, func() {
			sp := newSocketPair(p.pn, x, p.t)
			spnewlist := make([]*socketpair, 0, len(p.sp)+1)
			spnewlist = append(spnewlist, sp)
			for _, sp := range p.sp {
				if sp.IsRunning() {
					spnewlist = append(spnewlist, sp)
				}
			}
			p.sp = spnewlist
		})
	}

	closeSocket("listener", p.l)
	p.mu.Lock()
	splist := p.sp
	p.l = nil
	p.sp = nil
	p.mu.Unlock()

	for _, sp := range splist {
		sp.Close()
	}
}
