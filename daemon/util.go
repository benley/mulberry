package daemon

import (
	"io"
	"log"
	"net"
)

func closeSocket(role string, sock io.Closer) {
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
