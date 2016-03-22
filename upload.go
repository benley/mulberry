package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"golang.org/x/crypto/openpgp"

	"github.com/chronos-tachyon/mulberry/config"
)

type UploadHandler struct {
	keyring openpgp.KeyRing
	apply func(*config.Config) error
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Requires method: POST", http.StatusMethodNotAllowed)
		return
	}

	mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, "Cannot parse Content-Type", http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(mt, "multipart/signed") {
		http.Error(w, "Expected 'Content-Type: multipart/signed'", http.StatusUnsupportedMediaType)
		return
	}
	boundary, found := params["boundary"]
	if !found {
		http.Error(w, "'Content-Type: multipart/signed' is missing required param 'boundary'", http.StatusBadRequest)
		return
	}

	mpdata, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		log.Printf("error: failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	mp := multipart.NewReader(bytes.NewReader(mpdata), boundary)
	var yamldata, sigdata []byte
	for {
		part, err := mp.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Failed to parse multipart request body", http.StatusBadRequest)
			return
		}
		partdata, err := ioutil.ReadAll(part)
		if err != nil {
			http.Error(w, "Failed to read multipart request body", http.StatusBadRequest)
			return
		}
		mt, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			http.Error(w, "Failed to parse multipart child's 'Content-Type'", http.StatusBadRequest)
			return
		}
		switch {
		case strings.EqualFold(mt, "text/yaml"):
			if !strings.EqualFold(params["charset"], "utf-8") {
				http.Error(w, "Only support UTF-8 for text/yaml multipart child", http.StatusBadRequest)
				return
			}
			yamldata = partdata

		case strings.EqualFold(mt, "application/pgp-signature"):
			sigdata = partdata
		}
	}
	if yamldata == nil {
		http.Error(w, "Missing required multipart child: text/yaml", http.StatusBadRequest)
		return
	}
	if sigdata == nil {
		http.Error(w, "Missing required multipart child: application/pgp-signature", http.StatusBadRequest)
		return
	}

	signer, err := openpgp.CheckArmoredDetachedSignature(h.keyring, bytes.NewReader(yamldata), bytes.NewReader(sigdata))
	if err != nil {
		http.Error(w, "Failed to validate OpenPGP signature", http.StatusForbidden)
		return
	}
	log.Printf("info: good signature from: %s", signer.PrimaryKey.KeyIdString())

	cfg, err := config.Parse(yamldata)
	if err != nil {
		http.Error(w, "Failed to parse YAML configuration", http.StatusBadRequest)
		return
	}

	err = h.apply(cfg)
	if err != nil {
		log.Printf("error: %v", err)
		http.Error(w, "Failed to apply new config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", "4")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\r\n"))
}
