package providers

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

// maxHTTPAttempts is the total number of tries (1 initial + retries) for a
// request that fails with a transient network error.
const maxHTTPAttempts = 4

// doWithRetry runs an HTTP request built by build, retrying transient network
// failures (connection resets, EOFs, timeouts) with exponential backoff. build
// must produce a fresh *http.Request on each call because the body is consumed.
func doWithRetry(ctx context.Context, client *http.Client, build func() (*http.Request, error)) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < maxHTTPAttempts; attempt++ {
		req, err := build()
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if ctx.Err() != nil || !isTransient(err) {
			return nil, err
		}
		// Exponential backoff: ~0.5s, 1s, 2s.
		delay := time.Duration(500<<attempt) * time.Millisecond
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// isTransient reports whether an error from client.Do is worth retrying.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	s := err.Error()
	for _, m := range []string{
		"connection reset",
		"broken pipe",
		"unexpected EOF",
		"EOF",
		"connection refused",
		"TLS handshake timeout",
		"i/o timeout",
		"server closed",
		"no such host", // brief DNS blips
	} {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
