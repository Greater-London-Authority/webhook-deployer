package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
)

// https://stackoverflow.com/questions/53242837/validating-github-webhook-hmac-signature-in-go
func isValidSignature(r *http.Request, key string, body []byte) bool {
	// Assuming a non-empty header
	signature := r.Header.Get("X-Hub-Signature")
	gotHash := strings.SplitN(signature, "=", 2)
	if gotHash[0] != "sha1" {
		return false
	}

	if len(gotHash) != 2 {
		log.Printf("Invalid X-Hub-Signature: %s\n", signature)
		return false
	}

	hash := hmac.New(sha1.New, []byte(key))
	if _, err := hash.Write(body); err != nil {
		log.Printf("Cannot compute the HMAC for request: %s\n", err)
		return false
	}

	expectedHash := hex.EncodeToString(hash.Sum(nil))
	return gotHash[1] == expectedHash
}
