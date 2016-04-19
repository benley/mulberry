package daemon

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

func assert(cond bool, pattern string, args ...interface{}) {
	if !cond {
		panic(fmt.Errorf(pattern, args...))
	}
}

func closeSocket(role string, sock io.Closer) {
	if sock == nil {
		return
	}
	err := sock.Close()
	if err == nil {
		return
	}
	operr, ok := err.(*net.OpError)
	if ok && operr.Op == "close" && operr.Err.Error() == "use of closed network connection" {
		return
	}
	log.Printf("warning: failed to close %s: %v", role, err)
}

func withLock(l sync.Locker, f func()) {
	l.Lock()
	defer l.Unlock()
	f()
}

func withoutLock(l sync.Locker, f func()) {
	l.Unlock()
	defer l.Lock()
	f()
}
