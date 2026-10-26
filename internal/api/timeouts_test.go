package api

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestDefaultClientTimeouts(t *testing.T) {
	c := NewClient("https://example.invalid", "ua", nil)
	if c.hc.Timeout != 30*time.Second {
		t.Fatalf("overall timeout = %v", c.hc.Timeout)
	}
	tr, ok := c.hc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T", c.hc.Transport)
	}
	if tr.TLSHandshakeTimeout != 5*time.Second || tr.ResponseHeaderTimeout != 15*time.Second {
		t.Fatalf("tls %v, header %v", tr.TLSHandshakeTimeout, tr.ResponseHeaderTimeout)
	}
	if tr.DialContext == nil || tr.Proxy == nil || !tr.ForceAttemptHTTP2 {
		t.Fatal("dialer, proxy or HTTP/2 lost from the cloned default transport")
	}
}

func TestSilentServerFailsAtHeaderTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close() // accepts, never answers
		}
	}()
	hc := newHTTPClient(time.Second, time.Second, 150*time.Millisecond)
	c := NewClient("http://"+ln.Addr().String(), "ua", hc)
	c.RetryDelay = time.Millisecond
	start := time.Now()
	_, err = c.Books(context.Background(), "en")
	if err == nil {
		t.Fatal("expected an error")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("took %v", d)
	}
}
