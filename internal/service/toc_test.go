package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

type tocEnv struct {
	svc   *Service
	srv   *httptest.Server
	toc   atomic.Int32
	list  atomic.Int32
	fail  atomic.Bool
	now   time.Time
	paths chan string
}

func newTOCEnv(t *testing.T) *tocEnv {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("..", "api", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	e := &tocEnv{now: time.Now(), paths: make(chan string, 16)}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/books/bg/toc", func(w http.ResponseWriter, r *http.Request) {
		e.toc.Add(1)
		if e.fail.Load() {
			http.Error(w, "slow down", 429)
			return
		}
		w.Write(read("toc-bg.json"))
	})
	mux.HandleFunc("/api/v1/letters", func(w http.ResponseWriter, r *http.Request) {
		e.list.Add(1)
		if e.fail.Load() {
			http.Error(w, "slow down", 429)
			return
		}
		select {
		case e.paths <- r.URL.RawQuery:
		default:
		}
		w.Write(read("letters-1967.json"))
	})
	e.srv = httptest.NewServer(mux)
	t.Cleanup(e.srv.Close)
	dir := t.TempDir()
	c := api.NewClient(e.srv.URL, "test", nil)
	c.RetryDelay = time.Millisecond
	c.SetTOCLimit(time.Millisecond, 1)
	e.svc = &Service{
		Client: c,
		Items:  store.ItemCache{Dir: filepath.Join(dir, "items-v2")},
		Blobs:  store.BlobCache{Dir: filepath.Join(dir, "blobs")},
		Now:    func() time.Time { return e.now },
		Base:   e.srv.URL,
	}
	return e
}

func TestTOCFreshness(t *testing.T) {
	e := newTOCEnv(t)
	ctx := context.Background()
	if _, _, ok := e.svc.CachedTOC("bg", "en"); ok {
		t.Fatal("cached before any fetch")
	}
	toc, err := e.svc.TOC(ctx, "bg", "en")
	if err != nil || len(toc.Children) == 0 {
		t.Fatalf("toc = %d children, err %v", len(toc.Children), err)
	}
	if e.toc.Load() != 1 {
		t.Fatalf("requests = %d", e.toc.Load())
	}
	if _, err := e.svc.TOC(ctx, "bg", "en"); err != nil || e.toc.Load() != 1 {
		t.Fatalf("fresh hit made a request (%d) or failed: %v", e.toc.Load(), err)
	}
	if got, fresh, ok := e.svc.CachedTOC("bg", "en"); !ok || !fresh || len(got.Children) != len(toc.Children) {
		t.Errorf("CachedTOC = fresh %v ok %v", fresh, ok)
	}
	if _, _, ok := e.svc.CachedTOC("bg", "es"); ok {
		t.Error("other language shares the cache")
	}
	if _, err := os.Stat(filepath.Join(e.svc.Blobs.Dir, "toc", "en", "bg.json")); err != nil {
		t.Errorf("blob name: %v", err)
	}

	e.now = e.now.Add(25 * time.Hour)
	if _, fresh, ok := e.svc.CachedTOC("bg", "en"); !ok || fresh {
		t.Errorf("after 25h: fresh %v ok %v", fresh, ok)
	}
	e.fail.Store(true)
	stale, err := e.svc.TOC(ctx, "bg", "en")
	if err != nil || len(stale.Children) != len(toc.Children) {
		t.Fatalf("stale on error: %d children, err %v", len(stale.Children), err)
	}
	if e.toc.Load() != 2 {
		t.Errorf("requests = %d, want a refresh attempt", e.toc.Load())
	}
}

func TestTOCStaleRefreshed(t *testing.T) {
	e := newTOCEnv(t)
	ctx := context.Background()
	if _, err := e.svc.TOC(ctx, "bg", "en"); err != nil {
		t.Fatal(err)
	}
	e.now = e.now.Add(25 * time.Hour)
	if _, err := e.svc.TOC(ctx, "bg", "en"); err != nil || e.toc.Load() != 2 {
		t.Fatalf("refresh: requests %d err %v", e.toc.Load(), err)
	}
	e.now = time.Now() // blob times are real; the fake clock follows
	if _, fresh, _ := e.svc.CachedTOC("bg", "en"); !fresh {
		t.Error("refresh did not renew the copy")
	}
}

func TestTOCErrorWithoutCache(t *testing.T) {
	e := newTOCEnv(t)
	e.fail.Store(true)
	if _, err := e.svc.TOC(context.Background(), "bg", "en"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTOCDoesNotSpendVerseTokens(t *testing.T) {
	e := newTOCEnv(t)
	if _, err := e.svc.TOC(context.Background(), "bg", "en"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for range 5 {
		if _, err := e.svc.LetterYearPage(context.Background(), 1967, 1); err != nil {
			t.Fatal(err)
		}
		e.now = e.now.Add(8 * 24 * time.Hour) // expire, so each call hits the network
	}
	if time.Since(start) > time.Second {
		t.Error("verse limiter was drained by the TOC call")
	}
}

func TestLetterYearPageCache(t *testing.T) {
	e := newTOCEnv(t)
	ctx := context.Background()
	l, err := e.svc.LetterYearPage(ctx, 1967, 1)
	if err != nil || len(l.Items) != 3 {
		t.Fatalf("page: %d items, err %v", len(l.Items), err)
	}
	if q := <-e.paths; q != "page=1&per_page=100&year=1967" {
		t.Errorf("query = %q", q)
	}
	if _, err := e.svc.LetterYearPage(ctx, 1967, 1); err != nil || e.list.Load() != 1 {
		t.Fatalf("fresh hit made a request: %d %v", e.list.Load(), err)
	}
	if _, err := os.Stat(filepath.Join(e.svc.Blobs.Dir, "list", "letters", "1967-1.json")); err != nil {
		t.Errorf("blob name: %v", err)
	}
	e.now = e.now.Add(6 * 24 * time.Hour)
	e.svc.LetterYearPage(ctx, 1967, 1)
	if e.list.Load() != 1 {
		t.Error("6-day-old page refetched")
	}
	e.now = e.now.Add(2 * 24 * time.Hour)
	e.fail.Store(true)
	stale, err := e.svc.LetterYearPage(ctx, 1967, 1)
	if err != nil || len(stale.Items) != 3 {
		t.Fatalf("stale on error: %v", err)
	}
	e.fail.Store(false)
	before := e.list.Load()
	if _, err := e.svc.LetterYearPage(ctx, 1967, 1); err != nil || e.list.Load() != before+1 {
		t.Fatalf("refresh: %d (was %d) %v", e.list.Load(), before, err)
	}
}

func TestLetterYears(t *testing.T) {
	e := newTOCEnv(t)
	ctx := context.Background()
	years, undated, err := e.svc.LetterYears(ctx)
	if err != nil || len(years) != 28 || undated != 57 {
		t.Fatalf("years %d undated %d err %v", len(years), undated, err)
	}
	if _, _, err := e.svc.LetterYears(ctx); err != nil || e.list.Load() != 1 {
		t.Fatalf("fresh hit: %d %v", e.list.Load(), err)
	}
	if _, err := os.Stat(filepath.Join(e.svc.Blobs.Dir, "list", "letters", "years.json")); err != nil {
		t.Errorf("blob name: %v", err)
	}
	e.now = e.now.Add(8 * 24 * time.Hour)
	e.fail.Store(true)
	years, undated, err = e.svc.LetterYears(ctx)
	if err != nil || len(years) != 28 || undated != 57 {
		t.Fatalf("stale on error: %d %d %v", len(years), undated, err)
	}
}
