package daemon

import (
	"log"
	"net"
	"sync"

	"github.com/chronos-tachyon/mulberry/config"
)

type Port struct {
	listener net.Listener
	connect config.Address
	socketpairs []*SocketPair
	wg sync.WaitGroup
}

func (p *Port) Start(l net.Listener, c config.Address) {
	p.listener = l
	p.connect = c
	p.wg = sync.WaitGroup{}
	p.wg.Add(1)
	go p.EventLoop()
}

func (p *Port) Stop() {
	closeSocket("listener", p.listener)
	for _, s := range p.socketpairs {
		s.Stop()
	}
}

func (p *Port) Await() {
	p.wg.Wait()
}

func (p *Port) EventLoop() {
	for {
		x, err := p.listener.Accept()
		if err != nil {
			operr, ok := err.(*net.OpError)
			if !ok || operr.Op != "accept" || operr.Err.Error() != "use of closed network connection" {
				log.Printf("error: failed to accept: %v", err)
			}
			break
		}

		y, err := net.Dial(p.connect.Net, p.connect.Addr)
		if err != nil {
			log.Printf("error: failed to dial: %v", err)
			closeSocket("origin", x)
			continue
		}

		newS := make([]*SocketPair, 0, len(p.socketpairs)+1)
		s := new(SocketPair)
		s.Start(x, y)
		newS = append(newS, s)
		for _, s := range p.socketpairs {
			if s.IsRunning() {
				newS = append(newS, s)
			}
		}

		p.socketpairs = newS
	}

	p.Stop()
	for _, s := range p.socketpairs {
		s.Stop()
		s.Await()
	}
	p.wg.Done()
}
