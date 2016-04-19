package daemon

import (
	"log"
	"net"
	"sync"

	"github.com/chronos-tachyon/mulberry/config"
)

type Daemon struct {
	cfgsrc  config.Source
	eventch chan event
	stopch  chan struct{}
	wg      sync.WaitGroup
}

type event struct {
	cfg *config.Config
	err error
}

func New(configSource config.Source) *Daemon {
	d := &Daemon{
		cfgsrc:  configSource,
		eventch: make(chan event),
		stopch:  make(chan struct{}),
	}
	d.wg.Add(1)
	go d.loop()
	configSource.Watch(d.callback)
	return d
}

func (d *Daemon) Close() {
	d.cfgsrc.Close()
	close(d.stopch)
	d.wg.Wait()
	close(d.eventch)
}

func (d *Daemon) callback(cfg *config.Config, err error) {
	d.eventch <- event{cfg, err}
}

func (d *Daemon) loop() {
	ports := make(map[string]*listenport)
Outer:
	for {
		select {
		case <-d.stopch:
			break Outer
		case ev := <-d.eventch:
			if ev.err != nil {
				log.Printf("mulberry: %v", ev.err)
				break
			}
			apply(ports, ev.cfg)
		}
	}
	for _, p := range ports {
		p.Close()
	}
	d.wg.Done()
}

func apply(ports map[string]*listenport, cfg *config.Config) {
	seen := make(map[string]struct{})
	for _, port := range cfg.Ports {
		name := port.Listen.String()
		seen[name] = struct{}{}
		p, found := ports[name]
		if found {
			if port.Connect.String() == p.t.String() {
				continue
			}
			p.Alter(port.Connect)
		} else {
			l, err := net.Listen(port.Listen.Net, port.Listen.Addr)
			if err != nil {
				log.Printf("mulberry: failed to listen: %v", err)
				continue
			}
			ports[name] = newListenPort(port.Name, l, port.Connect)
		}
		restartsTotal.WithLabelValues(port.Name).Inc()
	}
	for name, p := range ports {
		if _, found := seen[name]; !found {
			delete(ports, name)
			p.Close()
		}
	}
}
