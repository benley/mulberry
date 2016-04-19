package config

import (
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type FileSource struct {
	path      string
	stopch    chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	callbacks []func(*Config, error)
	current   *Config
}

func NewFileSource(path string) *FileSource {
	return &FileSource{path: path}
}

func (fs *FileSource) Start() {
	fs.stopch = make(chan struct{})
	fs.wg.Add(1)
	go fs.loop()
}

func (fs *FileSource) Stop() {
	close(fs.stopch)
	fs.wg.Wait()
}

func (fs *FileSource) Watch(callback func(*Config, error)) {
	fs.mu.Lock()
	fs.callbacks = append(fs.callbacks, callback)
	cfg := fs.current
	fs.mu.Unlock()
	if cfg != nil {
		callback(cfg, nil)
	}
}

func (fs *FileSource) loop() {
	hupch := make(chan os.Signal, 1)
	signal.Notify(hupch, syscall.SIGHUP)
	looping := true
	for looping {
		cfg, err := parseFile(fs.path)
		fs.send(cfg, err)

		select {
		case <-fs.stopch:
			looping = false
		case <-hupch:
			// pass
		}
	}
	signal.Stop(hupch)
	close(hupch)
	fs.wg.Done()
}

func (fs *FileSource) send(cfg *Config, err error) {
	fs.mu.Lock()
	callbacks := make([]func(*Config, error), len(fs.callbacks))
	copy(callbacks, fs.callbacks)
	if cfg != nil {
		fs.current = cfg
	}
	fs.mu.Unlock()
	for _, callback := range callbacks {
		callback(cfg, err)
	}
}

func parseFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

var _ Source = &FileSource{}
