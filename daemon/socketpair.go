package daemon

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	liveConnectionsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "mulberry",
		Name:      "live_connections_total",
		Help:      "Number of active connections through the proxy for each port.",
	}, []string{"port"})
)

func init() {
	prometheus.MustRegister(liveConnectionsTotal)
}

type SocketPair struct {
	port string
	x    net.Conn
	y    net.Conn
	wg   sync.WaitGroup
}

func (s *SocketPair) Start(portName string, x, y net.Conn) {
	s.port = portName
	s.x = x
	s.y = y
	s.wg = sync.WaitGroup{}
	s.wg.Add(3)
	go s.loop()
}

func (s *SocketPair) Stop() {
	closeSocket("origin", s.x)
	closeSocket("destination", s.y)
}

func (s *SocketPair) IsRunning() bool {
	_, err := s.x.Write(nil)
	return err == nil
}

func (s *SocketPair) Await() {
	s.wg.Wait()
}

func (s *SocketPair) loop() {
	liveConnectionsTotal.WithLabelValues(s.port).Inc()
	go loopOneDirection(s.x, s.y, &s.wg)
	loopOneDirection(s.y, s.x, &s.wg)
	s.Stop()
	liveConnectionsTotal.WithLabelValues(s.port).Dec()
	s.wg.Done()
}

func loopOneDirection(in net.Conn, out net.Conn, w *sync.WaitGroup) {
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
	w.Done()
}
