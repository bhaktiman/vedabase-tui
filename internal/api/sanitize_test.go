package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSanitizeText(t *testing.T) {
	for in, want := range map[string]string{
		"\x1b[31mred\x1b[0m": "[31mred[0m",
		"\u009b2J":           "2J",
		"a\tb\nc":            "a\tb\nc",
		"कृष्ण":              "कृष्ण",
		"a\x7fb\x00c\x07d":   "abcd",
		"ś ā 🙏":              "ś ā 🙏",
	} {
		if got := SanitizeText(in); got != want {
			t.Errorf("SanitizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanSanitizes(t *testing.T) {
	it := Clean(Item{Translation: "ok\x1b]0;pwned\x07", Document: &Document{Location: "x\x1by"}})
	if strings.ContainsAny(it.Translation, "\x1b\x07") || it.Document.Location != "xy" {
		t.Errorf("item = %+v", it)
	}
}

func TestStatusErrorSanitized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"code":"x","message":"bad\u001b[2Jthing"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "vedabase-tui/test", srv.Client())
	c.RetryDelay = time.Millisecond
	_, err := c.Verse(context.Background(), "bg/1/1")
	if err == nil || strings.Contains(err.Error(), "\x1b") {
		t.Errorf("err = %q", err)
	}
}
