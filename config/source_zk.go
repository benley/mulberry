package config

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

type ZooKeeperSource struct {
	servers   []string
	path      string
	stopch    chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	callbacks []func(*Config, error)
	current   *Config
}

func NewZooKeeperSource(zkServers, zkPath string) *ZooKeeperSource {
	if zkServers == "" {
		zkServers = os.Getenv("ZOOKEEPER_SERVERS")
	}
	if zkServers == "" {
		zkServers = "127.0.0.1:2181"
	}
	zks := &ZooKeeperSource{
		servers: strings.Split(zkServers, ","),
		path:    zkPath,
		stopch:  make(chan struct{}),
	}
	zks.wg.Add(1)
	go zks.loop()
	return zks
}

func (zks *ZooKeeperSource) Close() {
	close(zks.stopch)
	zks.wg.Wait()
}

func (zks *ZooKeeperSource) Watch(callback func(*Config, error)) {
	zks.mu.Lock()
	zks.callbacks = append(zks.callbacks, callback)
	current := zks.current
	zks.mu.Unlock()
	if current != nil {
		callback(current, nil)
	}
}

func (zks *ZooKeeperSource) loop() {
	defer zks.wg.Done()

	var i uint
GoConnect:
	conn, _, err := zk.Connect(zks.servers, 1*time.Second)
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	if err != nil {
		if err != zk.ErrNoServer {
			zks.send(nil, err)
		}
		if !zks.backoff(i) {
			return
		}
		i++
		goto GoConnect
	}
	i = 0

GoExist:
	exists, _, ch, err := conn.ExistsW(zks.path)
	switch err {
	case nil:
		i = 0
	case zk.ErrNoServer, zk.ErrSessionMoved:
		if !zks.backoff(i) {
			return
		}
		i++
		goto GoExist
	default:
		zks.send(nil, err)
		conn.Close()
		conn = nil
		if !zks.backoff(i) {
			return
		}
		i++
		goto GoConnect
	}

	if !exists {
		select {
		case <-zks.stopch:
			return
		case ev := <-ch:
			if ev.Type != zk.EventNodeCreated {
				goto GoExist
			}
		}
	}

GoGet:
	configLoadsTotal.Inc()
	data, _, ch, err := conn.GetW(zks.path)
	switch err {
	case nil:
		i = 0
	case zk.ErrNoServer, zk.ErrSessionMoved:
		configReadErrorsTotal.Inc()
		if !zks.backoff(i) {
			return
		}
		i++
		goto GoGet
	case zk.ErrNoNode:
		configReadErrorsTotal.Inc()
		if !zks.backoff(i) {
			return
		}
		i++
		goto GoExist
	default:
		configReadErrorsTotal.Inc()
		zks.send(nil, err)
		conn.Close()
		conn = nil
		if !zks.backoff(i) {
			return
		}
		i++
		goto GoConnect
	}

	cfg, err := Parse(data)
	zks.send(cfg, err)
	if err == nil {
		configSuccessesTotal.Inc()
	} else {
		configParseErrorsTotal.Inc()
	}

	select {
	case <-zks.stopch:
		return
	case ev := <-ch:
		switch ev.Type {
		case zk.EventNodeDeleted:
			goto GoExist
		default:
			goto GoGet
		}
	}
}

func (zks *ZooKeeperSource) backoff(i uint) bool {
	t := time.NewTimer(time.Duration(1<<i) * time.Second)
	select {
	case <-zks.stopch:
		t.Stop()
		return false
	case <-t.C:
		return true
	}
}

func (zks *ZooKeeperSource) send(cfg *Config, err error) {
	zks.mu.Lock()
	callbacks := make([]func(*Config, error), len(zks.callbacks))
	copy(callbacks, zks.callbacks)
	if cfg != nil {
		zks.current = cfg
	}
	zks.mu.Unlock()
	for _, callback := range callbacks {
		callback(cfg, err)
	}
}

var _ Source = &ZooKeeperSource{}
