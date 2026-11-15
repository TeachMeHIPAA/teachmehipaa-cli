package cmdutil

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
)

// writeExchange prints one line per HTTP attempt, then the response headers
// when asked. Nothing here can hold a body or a request header (ADR-0007).
func writeExchange(w io.Writer, exchange api.Exchange, headers bool) {
	status := "no response"
	if exchange.Status != 0 {
		status = fmt.Sprint(exchange.Status)
	}
	line := fmt.Sprintf("%s %s %s %dms", exchange.Method, exchange.Path, status, exchange.Latency.Milliseconds())
	if exchange.RequestID != "" {
		line += " request-id=" + exchange.RequestID
	}
	if exchange.IdempotencyKey != "" {
		line += " idempotency-key=" + exchange.IdempotencyKey
	}
	fmt.Fprintln(w, line)
	if !headers {
		return
	}
	names := make([]string, 0, len(exchange.Header))
	for name := range exchange.Header {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		for _, value := range exchange.Header[name] {
			if redactedHeaders[http.CanonicalHeaderKey(name)] {
				value = "[redacted]"
			}
			fmt.Fprintf(w, "< %s: %s\n", name, value)
		}
	}
}

// redactedHeaders could carry a credential if a proxy ever echoed one.
var redactedHeaders = map[string]bool{
	"Authorization":       true,
	"Proxy-Authorization": true,
	"Cookie":              true,
	"Set-Cookie":          true,
}

func newUUIDv4() string {
	var b [16]byte
	// crypto/rand.Read never fails on supported platforms.
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	hex := fmt.Sprintf("%x", b)
	return strings.Join([]string{hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32]}, "-")
}
