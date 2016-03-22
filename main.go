package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/openpgp"

	"github.com/chronos-tachyon/mulberry/config"
	"github.com/chronos-tachyon/mulberry/daemon"
)

var (
	configFile  = flag.String("config", "", "path to the initial YAML configuration file")
	keyringFile = flag.String("keyring", "", "path to the GPG public keyring used to verify new configs")
	listenNet   = flag.String("net", "tcp", "protocol to listen on")
	listenAddr  = flag.String("addr", ":8643", "address to listen on")
)

func main() {
	flag.Parse()
	if *configFile == "" {
		log.Fatalf("fatal: missing required flag: -config")
	}

	var keyring openpgp.EntityList
	if *keyringFile != "" {
		f, err := os.Open(*keyringFile)
		if err != nil {
			log.Fatalf("fatal: failed to open -keyring: %v", err)
		}
		keyring, err = openpgp.ReadKeyRing(f)
		f.Close()
		if err != nil {
			log.Fatalf("fatal: failed to read -keyring: %v", err)
		}
	}

	l, err := net.Listen(*listenNet, *listenAddr)
	if err != nil {
		log.Fatalf("fatal: failed to listen: %v", err)
	}

	d := daemon.New(*configFile)

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

	applyFunc := func(cfg *config.Config) error {
		f, err := os.OpenFile(*configFile+".NEW", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return fmt.Errorf("failed to rewrite -config: %v", err)
		}
		_, err = f.Write(cfg.Save())
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to rewrite -config: %v", err)
		}
		err = f.Close()
		if err != nil {
			return fmt.Errorf("failed to rewrite -config: %v", err)
		}
		os.Remove(*configFile+"~")
		err = os.Rename(*configFile, *configFile+"~")
		if err != nil {
			linkerr := err.(*os.LinkError)
			errno, ok := linkerr.Err.(syscall.Errno)
			if !ok || errno != syscall.ENOENT {
				return fmt.Errorf("failed to backup -config: %v", err)
			}
		}
		err = os.Rename(*configFile+".NEW", *configFile)
		if err != nil {
			os.Rename(*configFile+"~", *configFile)
			return fmt.Errorf("failed to replace -config: %v", err)
		}
		d.Apply(cfg)
		return nil
	}

	log.Printf("info: serving on %s", l.Addr())
	d.Start()
	http.Handle("/metrics", prometheus.Handler())
	http.Handle("/upload", &UploadHandler{keyring, applyFunc})
	if err := http.Serve(l, nil); err != nil {
		operr, ok := err.(*net.OpError)
		if !ok || operr.Op != "accept" || operr.Err.Error() != "use of closed network connection" {
			log.Fatalf("fatal: %v", err)
		}
	}
	d.Await()
	log.Printf("info: graceful shutdown")
}
