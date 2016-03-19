package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/chronos-tachyon/mulberry/mulberrygrove/daemon"
)

var (
	config   = flag.String("config", "", "path to the YAML configuration file to share")
	keyring  = flag.String("keyring", "", "path to the GPG secret keyring to sign with")
	keyid    = flag.String("keyid", "", "hex ID of the GPG identity to sign with")
	bindNet  = flag.String("net", "tcp", "protocol to bind to")
	bindAddr = flag.String("bind", ":8080", "address to bind to")
)

func main() {
	flag.Parse()
	if *config == "" {
		log.Fatalf("fatal: missing required flag: -config")
	}
	if *keyring == "" {
		log.Fatalf("fatal: missing required flag: -keyring")
	}
	if *keyid == "" {
		log.Fatalf("fatal: missing required flag: -keyid")
	}
	kid, err := strconv.ParseUint(*keyid, 16, 64)
	if err != nil {
		log.Fatalf("fatal: failed to parse -keyid %q: %v", *keyid, err)
	}
	d := daemon.New(*config, *keyring, kid)

	l, err := net.Listen(*bindNet, *bindAddr)
	if err != nil {
		log.Fatalf("fatal: failed to listen: %v", err)
	}

	// Ignore SIGHUP
	hupch := make(chan os.Signal)
	signal.Notify(hupch, syscall.SIGHUP)
	defer signal.Stop(hupch)
	go (func() {
		for {
			sig := <-hupch
			if sig == nil {
				break
			}
		}
	})()

	// Shut down gracefully on SIGINT or SIGTERM
	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigch)
	go (func() {
		sig := <-sigch
		log.Printf("info: got signal %v", sig)
		l.Close()
	})()

	log.Printf("info: looping")
	d.Start()
	http.Handle("/config", d)
	err = http.Serve(l, nil)
	operr, ok := err.(*net.OpError)
	if !ok || operr.Op != "accept" && operr.Err.Error() != "use of closed network connection" {
		log.Fatalf("fatal: %v", err)
	}
	d.Stop()
	d.Await()
	log.Printf("info: graceful shutdown")
}
