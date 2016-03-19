package daemon

import (
	"io"
	"log"
	"net"
	"sync"
)

type SocketPair struct {
	X net.Conn
	Y net.Conn
	W sync.WaitGroup
}

func (s *SocketPair) Start(x, y net.Conn) {
	s.X = x
	s.Y = y
	s.W = sync.WaitGroup{}
	s.W.Add(3)
	go s.EventLoop()
}

func (s *SocketPair) Stop() {
	closeSocket("origin", s.X)
	closeSocket("destination", s.Y)
}

func (s *SocketPair) IsRunning() bool {
	_, err := s.X.Write(nil)
	return err == nil
}

func (s *SocketPair) Await() {
	s.W.Wait()
}

func (s *SocketPair) EventLoop() {
	go loopOneDirection(s.X, s.Y, &s.W)
	loopOneDirection(s.Y, s.X, &s.W)
	s.Stop()
	s.W.Done()
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
