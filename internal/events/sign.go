// Package events is `tmh events tail`'s forwarder: it turns an Event from the
// event log into a request signed exactly like a TeachMeHIPAA Delivery, so a
// partner's own SDK verifies it with the local Signing secret (ADR-0008).
package events

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"time"
)

// LocalKeyID stands in for a Webhook subscription's key id, which a local
// secret doesn't have.
const LocalKeyID = "local"

// Meta is what the signature headers say about the Event besides its body.
type Meta struct {
	EventID   string
	EventType string
}

// Sign returns the headers of a Delivery of body to pathAndQuery at the given
// time. The key is the secret's UTF-8 bytes, never its base64 decoding, as
// the platform's signer does.
func Sign(secret, pathAndQuery string, at time.Time, body []byte, meta Meta) http.Header {
	timestamp := at.UTC().Format("2006-01-02T15:04:05.000Z")
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("POST\n" + pathAndQuery + "\n" + timestamp + "\n" + bodyHashHex))
	header := http.Header{}
	header.Set("content-type", "application/json")
	header.Set("x-tmh-key-id", LocalKeyID)
	header.Set("x-tmh-timestamp", timestamp)
	header.Set("x-tmh-body-hash", bodyHashHex)
	header.Set("x-tmh-signature", hex.EncodeToString(mac.Sum(nil)))
	header.Set("x-tmh-event-type", meta.EventType)
	header.Set("x-tmh-event-id", meta.EventID)
	return header
}

// NewSigningSecret has the shape of a Webhook subscription's Signing secret:
// 32 random bytes in padded standard base64.
func NewSigningSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b[:]), nil
}
