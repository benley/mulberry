package daemon

import (
	"log"
	"net"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/chronos-tachyon/mulberry/config"
)

var (
	configErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "config_errors_total",
		Help:      "Number of failed attempts to read the Mulberry configuration from disk.",
	})
	restartsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "restarts_total",
		Help:      "Number of times each port has been (re)initialized, killing any existing connections.",
	}, []string{"port"})
)

func init() {
	prometheus.MustRegister(configErrorsTotal)
	prometheus.MustRegister(restartsTotal)
}

type Daemon struct {
	configPath string
	cfg        *config.Config
	ports      map[string]*Port
	reloadch   chan struct{}
	stopch     chan struct{}
	wg         sync.WaitGroup
}

func New(configFile string) *Daemon {
	return &Daemon{
		configPath: configFile,
		cfg:        nil,
		ports:      make(map[string]*Port),
		reloadch:   make(chan struct{}),
		stopch:     make(chan struct{}),
	}
}

func (d *Daemon) Start() {
	d.wg.Add(1)
	go d.loop()
}

func (d *Daemon) Reload() {
	d.reloadch <- struct{}{}
}

func (d *Daemon) Stop() {
	close(d.stopch)
}

func (d *Daemon) Await() {
	d.wg.Wait()
}

func (d *Daemon) loop() {
	d.reloadImpl()
Outer:
	for {
		select {
		case <-d.reloadch:
			d.reloadImpl()
		case <-d.stopch:
			break Outer
		}
	}
	for _, p := range d.ports {
		p.Stop()
		p.Await()
	}
	d.wg.Done()
}

func (d *Daemon) reloadImpl() {
	log.Printf("info: reloading config")
	cfg, err := config.Load(d.configPath)
	if err != nil {
		log.Printf("error: %v", err)
		configErrorsTotal.Inc()
		return
	}
	d.Apply(cfg)
}

func (d *Daemon) Apply(cfg *config.Config) {
	seen := make(map[string]struct{})
	var remove []string
	for _, port := range cfg.Ports {
		name := port.Listen.String()
		seen[name] = struct{}{}
		if p, found := d.ports[name]; found {
			if port.Connect.String() != p.connect.String() {
				p.Stop()
				p.Await()
				l, err := net.Listen(port.Listen.Net, port.Listen.Addr)
				if err != nil {
					log.Printf("error: failed to listen: %v", err)
					remove = append(remove, name)
					continue
				}
				p.Start(port.Name, l, port.Connect)
				restartsTotal.WithLabelValues(port.Name).Inc()
			}
		} else {
			l, err := net.Listen(port.Listen.Net, port.Listen.Addr)
			if err != nil {
				log.Printf("error: failed to listen: %v", err)
				continue
			}
			p = new(Port)
			p.Start(port.Name, l, port.Connect)
			restartsTotal.WithLabelValues(port.Name).Inc()
			d.ports[name] = p
		}
	}
	for name, p := range d.ports {
		if _, found := seen[name]; !found {
			p.Stop()
			p.Await()
			remove = append(remove, name)
		}
	}
	for _, name := range remove {
		delete(d.ports, name)
	}
	d.cfg = cfg
}
