package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strings"
)

// https://stackoverflow.com/questions/53242837/validating-github-webhook-hmac-signature-in-go
func isValidSignature(r *http.Request, key string) bool {
	// Assuming a non-empty header
	gotHash := strings.SplitN(r.Header.Get("X-Hub-Signature"), "=", 2)
	if gotHash[0] != "sha1" {
		return false
	}
	defer r.Body.Close()

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Cannot read the request body: %s\n", err)
		return false
	}

	hash := hmac.New(sha1.New, []byte(key))
	if _, err := hash.Write(b); err != nil {
		log.Printf("Cannot compute the HMAC for request: %s\n", err)
		return false
	}

	expectedHash := hex.EncodeToString(hash.Sum(nil))
	log.Println("EXPECTED HASH:", expectedHash)
	return gotHash[1] == expectedHash
}
