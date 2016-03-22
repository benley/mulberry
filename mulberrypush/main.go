package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strconv"

	"golang.org/x/crypto/openpgp"

	"github.com/chronos-tachyon/mulberry/config"
)

var (
	configFile = flag.String("config", "", "path to the YAML configuration file to push")
	keyring    = flag.String("keyring", "", "path to the GPG secret keyring to sign with")
	keyid      = flag.String("keyid", "", "hex ID of the GPG identity to sign with")
	url        = flag.String("url", "", "URL of the running Mulberry instance to push to")
)

func main() {
	flag.Parse()
	if *configFile == "" {
		log.Fatalf("fatal: missing required flag: -config")
	}
	if *keyring == "" {
		log.Fatalf("fatal: missing required flag: -keyring")
	}
	if *keyid == "" {
		log.Fatalf("fatal: missing required flag: -keyid")
	}
	if *url == "" {
		log.Fatalf("fatal: missing required flag: -url")
	}
	keyId, err := strconv.ParseUint(*keyid, 16, 64)
	if err != nil {
		log.Fatalf("fatal: failed to parse -keyid: %v", err)
	}

	f, err := os.Open(*keyring)
	if err != nil {
		log.Fatalf("fatal: failed to open -keyring file: %v", err)
	}
	keyRing, err := openpgp.ReadKeyRing(f)
	f.Close()
	if err != nil {
		log.Fatalf("fatal: failed to parse -keyring file: %v", err)
	}
	keys := keyRing.KeysById(keyId)
	if len(keys) < 1 {
		log.Fatalf("fatal: no key found with ID %08X", keyId)
	}
	signer := keys[0].Entity

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("fatal: failed to read -config: %v", err)
	}
	yamldata := cfg.Save()

	var mpbuf bytes.Buffer
	mp := multipart.NewWriter(&mpbuf)

	yamlhdr := make(textproto.MIMEHeader)
	yamlhdr.Set("Content-Type", "text/yaml; charset=utf-8")
	yamlpart, err := mp.CreatePart(yamlhdr)
	if err != nil {
		log.Fatalf("fatal: mp.CreatePart(yamlhdr): %v", err)
	}
	if _, err := yamlpart.Write(yamldata); err != nil {
		log.Fatalf("fatal: mp.CreatePart(yamlhdr).Write(): %v", err)
	}

	sighdr := make(textproto.MIMEHeader)
	sighdr.Set("Content-Type", "application/pgp-signature")
	sigpart, err := mp.CreatePart(sighdr)
	if err != nil {
		log.Fatalf("fatal: mp.CreatePart(sighdr): %v", err)
	}
	if err := openpgp.ArmoredDetachSignText(sigpart, signer, bytes.NewReader(yamldata), nil); err != nil {
		log.Fatalf("fatal: openpgp sign: %v", err)
	}

	if err := mp.Close(); err != nil {
		log.Fatalf("fatal: mp.Close(): %v", err)
	}
	mpdata := mpbuf.Bytes()

	mt := "multipart/signed; boundary=" + mp.Boundary()
	resp, err := http.Post(*url, mt, bytes.NewReader(mpdata))
	if err != nil {
		log.Fatalf("fatal: failed to upload config: %v", err)
	}
	fmt.Println(resp.Status)
	io.Copy(os.Stdout, resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		os.Exit(1)
	}
}
