package daemon

import (
	"log"
	"net"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/chronos-tachyon/mulberry/config"
)

var (
	acceptsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "accepts_total",
		Help:      "Number of times each port has accepted a connection.",
	}, []string{"port"})
	connectsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "connects_total",
		Help:      "Number of times each port has successfully dialed a new connection.",
	}, []string{"port"})
	dialErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "dial_errors_total",
		Help:      "Number of times each port has failed to dial a new connection.",
	}, []string{"port"})
)

func init() {
	prometheus.MustRegister(acceptsTotal)
	prometheus.MustRegister(connectsTotal)
	prometheus.MustRegister(dialErrorsTotal)
}

type Port struct {
	name        string
	listener    net.Listener
	connect     config.Address
	socketpairs []*SocketPair
	wg          sync.WaitGroup
}

func (p *Port) Start(portName string, l net.Listener, c config.Address) {
	p.name = portName
	p.listener = l
	p.connect = c
	p.wg = sync.WaitGroup{}
	p.wg.Add(1)
	go p.loop()
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

func (p *Port) loop() {
	for {
		x, err := p.listener.Accept()
		if err != nil {
			operr, ok := err.(*net.OpError)
			if !ok || operr.Op != "accept" || operr.Err.Error() != "use of closed network connection" {
				log.Printf("error: failed to accept: %v", err)
			}
			break
		}
		acceptsTotal.WithLabelValues(p.name).Inc()

		y, err := net.Dial(p.connect.Net, p.connect.Addr)
		if err != nil {
			log.Printf("error: failed to dial: %v", err)
			closeSocket("origin", x)
			dialErrorsTotal.WithLabelValues(p.name).Inc()
			continue
		}
		connectsTotal.WithLabelValues(p.name).Inc()

		newS := make([]*SocketPair, 0, len(p.socketpairs)+1)
		s := new(SocketPair)
		s.Start(p.name, x, y)
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
