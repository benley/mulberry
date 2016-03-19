package daemon

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/openpgp"

	"github.com/chronos-tachyon/mulberry/config"
)

const boundary = `2dcadde02f30b24cb66cd69ca3b20cc361a64e713babf1762c645470bbde`

type Daemon struct {
	configPath  string
	keyringPath string
	keyId       uint64
	mutex       *sync.RWMutex
	cond        *sync.Cond
	wg          *sync.WaitGroup
	config      *config.Config
	time        time.Time
	stopch      chan struct{}
}

func New(configFile string, keyringFile string, keyId uint64) *Daemon {
	mu := &sync.RWMutex{}
	return &Daemon{
		configPath:  configFile,
		keyringPath: keyringFile,
		keyId:       keyId,
		mutex:       mu,
		cond:        sync.NewCond(mu.RLocker()),
		wg:          &sync.WaitGroup{},
		stopch:      make(chan struct{}),
	}
}

func (d *Daemon) Start() {
	d.wg.Add(1)
	go d.loop()
}

func (d *Daemon) Stop() {
	close(d.stopch)
}

func (d *Daemon) Await() {
	d.wg.Wait()
}

func (d *Daemon) loop() {
	d.update()
	t := time.NewTicker(1 * time.Second)
Outer:
	for {
		select {
		case <-t.C:
			d.update()

		case <-d.stopch:
			t.Stop()
			break Outer
		}
	}
	d.wg.Done()
}

func (d *Daemon) update() {
	cfg, err := config.Load(d.configPath)
	if err != nil {
		log.Printf("error: %v", err)
	}

	d.mutex.Lock()
	if !bytes.Equal(cfg.Save(), d.config.Save()) {
		d.config = cfg
		d.time = time.Now().UTC()
		d.cond.Broadcast()
		log.Printf("info: updated config")
	}
	d.mutex.Unlock()
}

func (d *Daemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	x := r.URL.Query().Get("wait")
	if x == "" {
		x = "0"
	}
	wait, err := strconv.ParseBool(x)
	if err != nil {
		http.Error(w, "Invalid value for 'wait' query parameter", 400)
		return
	}

	f, err := os.Open(d.keyringPath)
	if err != nil {
		log.Printf("error: failed to open: %s: %v", d.keyringPath, err)
		http.Error(w, "500 internal server error", 500)
		return
	}
	keyring, err := openpgp.ReadKeyRing(f)
	f.Close()
	if err != nil {
		log.Printf("error: failed to parse: %s: %v", d.keyringPath, err)
		http.Error(w, "500 internal server error", 500)
		return
	}
	keys := keyring.KeysById(d.keyId)
	if len(keys) < 1 {
		log.Printf("error: no key found with ID %08x", d.keyId)
		http.Error(w, "500 internal server error", 500)
		return
	}
	signer := keys[0].Entity

	d.mutex.RLock()
	if wait {
		d.cond.Wait()
	}
	yamldata := d.config.Save()
	when := d.time
	d.mutex.RUnlock()

	var mpbuf bytes.Buffer
	mp := multipart.NewWriter(&mpbuf)
	mp.SetBoundary(boundary)

	yamlhdr := make(textproto.MIMEHeader)
	yamlhdr.Set("Content-Type", "text/yaml; charset=utf-8")
	yamlpart, err := mp.CreatePart(yamlhdr)
	if err != nil {
		log.Printf("error: mp.CreatePart(yamlhdr): %v", err)
		http.Error(w, "500 internal server error", 500)
		return
	}
	if _, err := yamlpart.Write(yamldata); err != nil {
		log.Printf("error: mp.CreatePart(yamlhdr).Write(): %v", err)
		http.Error(w, "500 internal server error", 500)
		return
	}

	sighdr := make(textproto.MIMEHeader)
	sighdr.Set("Content-Type", "application/pgp-signature")
	sigpart, err := mp.CreatePart(sighdr)
	if err != nil {
		log.Printf("error: mp.CreatePart(sighdr): %v", err)
		http.Error(w, "500 internal server error", 500)
		return
	}
	if err := openpgp.ArmoredDetachSignText(sigpart, signer, bytes.NewReader(yamldata), nil); err != nil {
		log.Printf("error: openpgp sign: %v", err)
		http.Error(w, "500 internal server error", 500)
		return
	}

	if err := mp.Close(); err != nil {
		log.Printf("error: mp.Close(): %v", err)
		http.Error(w, "500 internal server error", 500)
		return
	}
	mpdata := mpbuf.Bytes()

	w.Header().Set("Content-Type", "multipart/signed")
	w.Header().Set("ETag", etagFor(mpdata))
	http.ServeContent(w, r, "", when, bytes.NewReader(mpdata))
}

func etagFor(data []byte) string {
	hash := sha1.Sum(data)
	b64hash := base64.StdEncoding.EncodeToString(hash[:])
	return "\"" + b64hash + "\""
}
