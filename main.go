package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/chronos-tachyon/mulberry/daemon"
)

var (
	config = flag.String("config", "", "path to the YAML configuration file")
)

func main() {
	flag.Parse()
	if *config == "" {
		log.Fatalf("fatal: missing required flag: -config")
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
		d.Stop()
		close(hupch)
	})()

	log.Printf("info: looping")
	d.Loop()
	log.Printf("info: graceful shutdown")
}
