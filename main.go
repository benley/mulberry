package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/chronos-tachyon/mulberry/daemon"
)

var (
	config = flag.String("config", "", "path to the YAML configuration file")
	listenNet = flag.String("net", "tcp", "protocol to listen on")
	listenAddr = flag.String("addr", ":8643", "address to listen on")
)

func main() {
	flag.Parse()
	if *config == "" {
		log.Fatalf("fatal: missing required flag: -config")
	}

	l, err := net.Listen(*listenNet, *listenAddr)
	if err != nil {
		log.Fatalf("fatal: failed to listen: %v", err)
	}

	d := daemon.New(*config)

	hupch := make(chan os.Signal)
	signal.Notify(hupch, syscall.SIGHUP)
	defer signal.Stop(hupch)
	go (func() {
		for {
			sig := <-hupch
			if sig == nil {
				break
			}
			log.Printf("info: got signal %v", sig)
			d.Reload()
		}
	})()

	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigch)
	go (func() {
		sig := <-sigch
		log.Printf("info: got signal %v", sig)
		l.Close()
		d.Stop()
		close(hupch)
	})()

	log.Printf("info: serving on %s", l.Addr())
	d.Start()
	http.Handle("/metrics", prometheus.Handler())
	if err := http.Serve(l, nil); err != nil {
		operr, ok := err.(*net.OpError)
		if !ok || operr.Op != "accept" || operr.Err.Error() != "use of closed network connection" {
			log.Fatalf("fatal: %v", err)
		}
	}
	d.Await()
	log.Printf("info: graceful shutdown")
}
