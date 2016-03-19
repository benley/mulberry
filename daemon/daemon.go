package daemon

import (
	"log"
	"net"

	"github.com/chronos-tachyon/mulberry/config"
)

type Daemon struct {
	F        string
	C        *config.Config
	P        map[string]*Port
	ReloadCh chan struct{}
	StopCh   chan struct{}
}

func New(configFile string) *Daemon {
	return &Daemon{
		F:        configFile,
		C:        nil,
		P:        make(map[string]*Port),
		ReloadCh: make(chan struct{}),
		StopCh:   make(chan struct{}),
	}
}

func (d *Daemon) Reload() {
	d.ReloadCh <- struct{}{}
}

func (d *Daemon) Stop() {
	close(d.StopCh)
}

func (d *Daemon) Loop() {
	d.reloadImpl()
Outer:
	for {
		select {
		case <-d.ReloadCh:
			d.reloadImpl()
		case <-d.StopCh:
			break Outer
		}
	}
	for _, p := range d.P {
		p.Stop()
		p.Await()
	}
}

func (d *Daemon) reloadImpl() {
	log.Printf("info: reloading config")
	cfg, err := config.Load(d.F)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	seen := make(map[string]struct{})
	var remove []string
	for _, port := range cfg.Ports {
		name := port.Listen.String()
		seen[name] = struct{}{}
		if p, found := d.P[name]; found {
			if port.Connect.String() != p.C.String() {
				p.Stop()
				p.Await()
				l, err := net.Listen(port.Listen.Net, port.Listen.Addr)
				if err != nil {
					log.Printf("error: failed to listen: %v", err)
					remove = append(remove, name)
					continue
				}
				p.Start(l, port.Connect)
			}
		} else {
			l, err := net.Listen(port.Listen.Net, port.Listen.Addr)
			if err != nil {
				log.Printf("error: failed to listen: %v", err)
				continue
			}
			p = new(Port)
			p.Start(l, port.Connect)
			d.P[name] = p
		}
	}
	for name, p := range d.P {
		if _, found := seen[name]; !found {
			p.Stop()
			p.Await()
			remove = append(remove, name)
		}
	}
	for _, name := range remove {
		delete(d.P, name)
	}
	d.C = cfg
}
