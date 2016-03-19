package daemon

import (
	"log"
	"net"
	"sync"

	"github.com/chronos-tachyon/mulberry/config"
)

type Port struct {
	L net.Listener
	C config.Address
	S []*SocketPair
	W sync.WaitGroup
}

func (p *Port) Start(l net.Listener, c config.Address) {
	p.L = l
	p.C = c
	p.W = sync.WaitGroup{}
	p.W.Add(1)
	go p.EventLoop()
}

func (p *Port) Stop() {
	closeSocket("listener", p.L)
	for _, s := range p.S {
		s.Stop()
	}
}

func (p *Port) Await() {
	p.W.Wait()
}

func (p *Port) EventLoop() {
	for {
		x, err := p.L.Accept()
		if err != nil {
			operr, ok := err.(*net.OpError)
			if !ok || operr.Op != "accept" || operr.Err.Error() != "use of closed network connection" {
				log.Printf("error: failed to accept: %v", err)
			}
			break
		}

		y, err := net.Dial(p.C.Net, p.C.Addr)
		if err != nil {
			log.Printf("error: failed to dial: %v", err)
			closeSocket("origin", x)
			continue
		}

		newS := make([]*SocketPair, 0, len(p.S)+1)
		s := new(SocketPair)
		s.Start(x, y)
		newS = append(newS, s)
		for _, s := range p.S {
			if s.IsRunning() {
				newS = append(newS, s)
			}
		}

		p.S = newS
	}

	p.Stop()
	for _, s := range p.S {
		s.Stop()
		s.Await()
	}
	p.W.Done()
}
