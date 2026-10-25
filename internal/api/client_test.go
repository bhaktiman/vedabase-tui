package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func serveJSON(t *testing.T, routes map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			w.Write(fixture(t, "err-not-found.json"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, name))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "vedabase-tui/test", srv.Client())
	c.RetryDelay = time.Millisecond
	return c
}

func TestVerseDecodes(t *testing.T) {
	c := serveJSON(t, map[string]string{
		"/api/v1/verse/bg/2/13": "verse-bg-2-13.json",
		"/api/v1/verse/lcfl/1":  "verse-lcfl-1.json",
	})
	it, err := c.Verse(context.Background(), "bg/2/13")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(it.Translation, "**As the embodied soul") || it.Document != nil {
		t.Errorf("bg/2/13: translation %q document %v", it.Translation, it.Document)
	}
	it, err = c.Verse(context.Background(), "lcfl/1")
	if err != nil {
		t.Fatal(err)
	}
	if it.Devanagari != "" || it.Translation != "The First Morning Walk: April 18, 1973" {
		t.Errorf("lcfl/1: %q %q", it.Devanagari, it.Translation)
	}
}

func TestDocumentDecodes(t *testing.T) {
	c := serveJSON(t, map[string]string{"/api/v1/verse/letters/letter-to-aatie": "verse-letter-aatie.json"})
	it, err := c.Verse(context.Background(), "letters/letter-to-aatie")
	if err != nil {
		t.Fatal(err)
	}
	if it.Document == nil || it.Document.Date != "October 28th 1967" || it.Document.Location != "Navadvipa" {
		t.Errorf("document = %+v", it.Document)
	}
}

func TestBooks(t *testing.T) {
	c := serveJSON(t, map[string]string{"/api/v1/books": "books.json"})
	bs, err := c.Books(context.Background())
	if err != nil || len(bs) != 24 || bs[0].Code != "bg" {
		t.Fatalf("books = %d, %v", len(bs), err)
	}
}

func TestUnknownFieldsAndNulls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ref":"x/1","devanagari":null,"new_field":{"a":1},"document":{"year":null}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "ua", srv.Client())
	it, err := c.Verse(context.Background(), "x/1")
	if err != nil {
		t.Fatal(err)
	}
	if it.Document == nil || it.Document.Year != 0 || it.Devanagari != "" {
		t.Errorf("item = %+v", it)
	}
}

func TestRequestShape(t *testing.T) {
	var verseOK, searchOK atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Header.Get("User-Agent") != "vedabase-tui/test" {
			t.Errorf("UA = %q", r.Header.Get("User-Agent"))
		}
		switch r.URL.Path {
		case "/api/v1/verse/bg/2/13":
			verseOK.Store(q.Get("lang") == "en")
			w.Write(fixture(t, "verse-bg-2-13.json"))
		case "/api/v1/search":
			searchOK.Store(q.Get("q") == "soul body" && q.Get("book") == "bg" &&
				q.Get("page") == "2" && q.Get("per_page") == "20" && q.Get("lang") == "en")
			w.Write(fixture(t, "search-soul-body.json"))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "vedabase-tui/test", srv.Client())
	if _, err := c.Verse(context.Background(), "bg/2/13"); err != nil {
		t.Fatal(err)
	}
	sp, err := c.Search(context.Background(), "soul body", "bg", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !verseOK.Load() || !searchOK.Load() {
		t.Errorf("request shape wrong: verse %v search %v", verseOK.Load(), searchOK.Load())
	}
	if !sp.HasMore || len(sp.Results) != 2 || sp.Results[0].Ref != "bg/2/17" {
		t.Errorf("search page = %+v", sp)
	}
}

func TestNotFound(t *testing.T) {
	c := serveJSON(t, nil)
	_, err := c.Verse(context.Background(), "bg/99/1")
	if !IsNotFound(err) || !strings.Contains(err.Error(), "No text for bg/99/1") {
		t.Errorf("err = %v", err)
	}
}

func TestRateLimited(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "ua", srv.Client())
	c.RetryDelay = time.Millisecond
	_, err := c.Verse(context.Background(), "bg/1/1")
	var rl *RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter != 7*time.Second || n.Load() != 1 {
		t.Errorf("err = %v, requests = %d", err, n.Load())
	}
}

func TestRetryOnceOn5xx(t *testing.T) {
	var n atomic.Int32
	failFirst := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := n.Add(1)
		if i == 1 || !failFirst {
			w.WriteHeader(500)
			w.Write([]byte(`{"error":{"code":"lookup_failed","message":"boom"}}`))
			return
		}
		w.Write(fixture(t, "verse-bg-2-13.json"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "ua", srv.Client())
	c.RetryDelay = time.Millisecond
	if _, err := c.Verse(context.Background(), "bg/2/13"); err != nil || n.Load() != 2 {
		t.Fatalf("err = %v, requests = %d", err, n.Load())
	}

	n.Store(0)
	failFirst = false
	_, err := c.Verse(context.Background(), "bg/2/13")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 500 || ae.Code != "lookup_failed" || n.Load() != 2 {
		t.Errorf("err = %v, requests = %d", err, n.Load())
	}
}

func TestOffline(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	u := srv.URL
	srv.Close()
	c := NewClient(u, "ua", nil)
	_, err := c.Verse(context.Background(), "bg/1/1")
	if !errors.Is(err, ErrOffline) {
		t.Errorf("err = %v", err)
	}
}

func TestFetchProgress(t *testing.T) {
	body := make([]byte, 10240)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10240")
		w.Write(body)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "ua", srv.Client())
	var calls int
	var lastDone, lastTotal int64
	got, err := c.Fetch(context.Background(), srv.URL+"/sitemap.xml", func(d, tot int64) {
		calls++
		lastDone, lastTotal = d, tot
	})
	if err != nil || len(got) != 10240 {
		t.Fatalf("len %d err %v", len(got), err)
	}
	if calls == 0 || lastDone != 10240 || lastTotal != 10240 {
		t.Errorf("calls %d last (%d,%d)", calls, lastDone, lastTotal)
	}
}

func TestLimiterBlocks(t *testing.T) {
	c := serveJSON(t, map[string]string{"/api/v1/search": "search-soul-body.json"})
	for i := 0; i < 5; i++ {
		if _, err := c.Search(context.Background(), "soul", "", 1); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Search(ctx, "soul", "", 1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("blocked too long: %v", time.Since(start))
	}
}
