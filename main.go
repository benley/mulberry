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

	"github.com/chronos-tachyon/mulberry/config"
	"github.com/chronos-tachyon/mulberry/daemon"
)

var (
	sourceFlag     = flag.String("source", "", "where to get a config from; one of: file, zookeeper")
	filePathFlag   = flag.String("file_path", "", "filesystem path to the YAML configuration file")
	zkServersFlag  = flag.String("zookeeper_servers", "", "the ZooKeeper cluster containing the config file; default $ZOOKEEPER_SERVERS")
	zkConfigFlag   = flag.String("zookeeper_config", "/mulberry/config", "ZooKeeper path to the YAML configuration file")
	listenNetFlag  = flag.String("net", "tcp", "protocol for HTTP metrics server to listen on")
	listenAddrFlag = flag.String("addr", ":8643", "address for HTTP metrics server to listen on")
)

func main() {
	flag.Parse()
	if *sourceFlag == "" {
		log.Fatalf("fatal: missing required flag: -source=")
	}

	var source config.Source
	switch *sourceFlag {
	case "file":
		if *filePathFlag == "" {
			log.Fatalf("fatal: missing required flag: -file_path=")
		}
		source = config.NewFileSource(*filePathFlag)
	case "zookeeper":
		signal.Ignore(syscall.SIGHUP)
		source = config.NewZooKeeperSource(*zkServersFlag, *zkConfigFlag)
	default:
		log.Fatalf("fatal: unknown flag value: -source=%q", *sourceFlag)
	}

	l, err := net.Listen(*listenNetFlag, *listenAddrFlag)
	if err != nil {
		log.Fatalf("fatal: failed to listen: %v", err)
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigch)
	go (func() {
		sig := <-sigch
		log.Printf("info: got signal %v", sig)
		l.Close()
	})()

	log.Printf("info: serving on %s", l.Addr())
	http.Handle("/metrics", prometheus.Handler())
	d := daemon.New(source)
	d.Start()
	if err := http.Serve(l, nil); err != nil {
		operr, ok := err.(*net.OpError)
		if !ok || operr.Op != "accept" || operr.Err.Error() != "use of closed network connection" {
			log.Fatalf("fatal: %v", err)
		}
	}
	d.Stop()
	log.Printf("info: graceful shutdown")
}
