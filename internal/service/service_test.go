package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type env struct {
	dir   string
	langs *[]string // lang query of each request, in order
	svc   *Service
	srv   *httptest.Server
	hits  *atomic.Int32
	now   *time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	hits := &atomic.Int32{}
	var mu sync.Mutex
	langs := &[]string{}
	mux := http.NewServeMux()
	handle := func(path, ctype, file string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			mu.Lock()
			*langs = append(*langs, r.URL.Query().Get("lang"))
			mu.Unlock()
			w.Header().Set("Content-Type", ctype)
			w.Write(fixture(t, file))
		})
	}
	handle("/api/v1/verse/bg/2/13", "application/json", "verse-bg-2-13.json")
	handle("/api/v1/verse/letters/letter-to-lokanatha-2", "application/json", "verse-letter-lokanatha-2.json")
	handle("/api/v1/books", "application/json", "books.json")
	handle("/sitemap-en-books.xml", "application/xml", "sitemap-sample.xml")
	handle("/en/library/bg/2/13/", "text/html", "books.json")
	mux.HandleFunc("/en/library/letters/", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("year=" + r.URL.Query().Get("year")))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	now := time.Now()
	e := &env{dir: dir, langs: langs, srv: srv, hits: hits, now: &now}
	e.svc = &Service{
		Client: api.NewClient(srv.URL, "test", nil),
		Items:  store.ItemCache{Dir: filepath.Join(dir, "items-v2"), OldDir: filepath.Join(dir, "items")},
		Blobs:  store.BlobCache{Dir: filepath.Join(dir, "blobs")},
		Now:    func() time.Time { return *e.now },
		Base:   srv.URL,
	}
	return e
}

func TestItemCachesAfterFetch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	it, cached, err := e.svc.Item(ctx, "bg/2/13", "en")
	if err != nil || cached || it.Ref != "bg/2/13" {
		t.Fatalf("first: %+v cached=%v err=%v", it, cached, err)
	}
	if e.hits.Load() != 1 {
		t.Fatalf("hits = %d", e.hits.Load())
	}
	_, cached, err = e.svc.Item(ctx, "bg/2/13", "en")
	if err != nil || !cached || e.hits.Load() != 1 {
		t.Fatalf("second: cached=%v err=%v hits=%d", cached, err, e.hits.Load())
	}
	if !e.svc.Cached("bg/2/13", "en") {
		t.Error("Cached false")
	}
}

func TestItemCleansCachedLetter(t *testing.T) {
	e := newEnv(t)
	raw := `{"ref":"letters/x","book":"letters","purport":"Letter to: X70-07-14DetroitMy Dear X,Hi."}`
	if err := e.svc.Items.PutRaw("en", "letters/x", []byte(raw)); err != nil {
		t.Fatal(err)
	}
	it, cached, err := e.svc.Item(context.Background(), "letters/x", "en")
	if err != nil || !cached || it.Purport != "Detroit\n\nMy Dear X,\n\nHi." {
		t.Fatalf("%q cached=%v err=%v", it.Purport, cached, err)
	}
	if got, _ := e.svc.Items.GetRaw("en", "letters/x"); string(got) != raw {
		t.Error("cache not raw")
	}
}

func (e *env) path(parts ...string) string {
	return filepath.Join(append([]string{e.dir}, parts...)...)
}

func TestItemStoredPerLanguage(t *testing.T) {
	e := newEnv(t)
	if _, _, err := e.svc.Item(context.Background(), "bg/2/13", "es"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(e.path("items-v2", "es", "bg", "2", "13.json")); err != nil {
		t.Fatal(err)
	}
	if e.svc.Cached("bg/2/13", "en") || !e.svc.Cached("bg/2/13", "es") {
		t.Error("Cached ignores language")
	}
	if got := *e.langs; len(got) != 1 || got[0] != "es" {
		t.Errorf("langs = %v", got)
	}
}

func TestLetterAlwaysEnglish(t *testing.T) {
	e := newEnv(t)
	it, _, err := e.svc.Item(context.Background(), "letters/letter-to-lokanatha-2", "es")
	if err != nil {
		t.Fatal(err)
	}
	if got := *e.langs; len(got) != 1 || got[0] != "en" {
		t.Errorf("langs = %v", got)
	}
	if _, err := os.Stat(e.path("items-v2", "en", "letters", "letter-to-lokanatha-2.json")); err != nil {
		t.Fatal(err)
	}
	if !e.svc.Cached("letters/letter-to-lokanatha-2", "es") {
		t.Error("Cached should resolve to en for letters")
	}
	if it.Document == nil || it.Document.Address != "Vrindaban" || !strings.HasPrefix(it.Purport, "My Dear Lokanatha") {
		t.Errorf("item = %+v", it)
	}
}

func oldItem(t *testing.T, e *env, ref string) {
	t.Helper()
	if err := store.WriteFileAtomic(e.path("items", filepath.FromSlash(ref)+".json"),
		[]byte(`{"ref":"`+ref+`","book":"bg","translation":"OLD"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestOldCacheNotUsedWhileOnline(t *testing.T) {
	e := newEnv(t)
	oldItem(t, e, "bg/2/13")
	it, cached, err := e.svc.Item(context.Background(), "bg/2/13", "en")
	if err != nil || cached || it.Translation == "OLD" || e.hits.Load() != 1 {
		t.Fatalf("it=%q cached=%v err=%v hits=%d", it.Translation, cached, err, e.hits.Load())
	}
}

func TestOldCacheServedOffline(t *testing.T) {
	e := newEnv(t)
	oldItem(t, e, "bg/2/13")
	e.srv.Close()
	it, cached, err := e.svc.Item(context.Background(), "bg/2/13", "en")
	if err != nil || !cached || it.Translation != "OLD" {
		t.Fatalf("it=%q cached=%v err=%v", it.Translation, cached, err)
	}
	if _, err := os.Stat(e.path("items-v2", "en", "bg", "2", "13.json")); err == nil {
		t.Error("old item copied forward")
	}
	if _, _, err := e.svc.Item(context.Background(), "bg/2/13", "es"); !errors.Is(err, api.ErrOffline) {
		t.Errorf("es served from the old cache: err=%v", err)
	}
}

func TestCachedKeepsUnknownFields(t *testing.T) {
	e := newEnv(t)
	raw := `{"ref":"bg/2/13","translation":"T","future_field":{"a":1}}`
	if err := e.svc.Items.PutRaw("en", "bg/2/13", []byte(raw)); err != nil {
		t.Fatal(err)
	}
	if _, cached, err := e.svc.Item(context.Background(), "bg/2/13", "en"); err != nil || !cached {
		t.Fatalf("cached=%v err=%v", cached, err)
	}
	b, err := os.ReadFile(e.path("items-v2", "en", "bg", "2", "13.json"))
	if err != nil || string(b) != raw {
		t.Fatalf("on disk %q err=%v", b, err)
	}
}

func TestFetchedItemKeepsRawBytes(t *testing.T) {
	e := newEnv(t)
	if _, _, err := e.svc.Item(context.Background(), "bg/2/13", "en"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(e.path("items-v2", "en", "bg", "2", "13.json"))
	if err != nil || string(b) != string(fixture(t, "verse-bg-2-13.json")) {
		t.Fatalf("stored bytes differ (err=%v)", err)
	}
}

func TestLibraryFirstLaunch(t *testing.T) {
	e := newEnv(t)
	var called bool
	lib, stale, err := e.svc.Library(context.Background(), func(done, total int64) { called = true })
	if err != nil || stale || !called {
		t.Fatalf("err=%v stale=%v progress=%v", err, stale, called)
	}
	if lib.Find("bg") == nil {
		t.Error("no bg root")
	}
	if _, _, ok := e.svc.Blobs.Get("sitemap-en-books.xml"); !ok {
		t.Error("sitemap not stored")
	}
}

func TestLibraryStale(t *testing.T) {
	e := newEnv(t)
	if _, _, err := e.svc.Library(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	before := e.hits.Load()
	*e.now = e.now.Add(8 * 24 * time.Hour)
	lib, stale, err := e.svc.Library(context.Background(), nil)
	if err != nil || !stale || lib == nil {
		t.Fatalf("err=%v stale=%v", err, stale)
	}
	if e.hits.Load() != before {
		t.Error("request made")
	}
	if _, err := e.svc.RefreshLibrary(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.hits.Load() != before+1 {
		t.Errorf("refresh hits = %d", e.hits.Load()-before)
	}
}

func TestLibraryOfflineNoCache(t *testing.T) {
	e := newEnv(t)
	e.srv.Close()
	_, _, err := e.svc.Library(context.Background(), nil)
	if !errors.Is(err, api.ErrOffline) {
		t.Fatalf("err = %v", err)
	}
}

func TestLibraryOfflineWithCache(t *testing.T) {
	e := newEnv(t)
	if _, _, err := e.svc.Library(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	e.srv.Close()
	lib, _, err := e.svc.Library(context.Background(), nil)
	if err != nil || lib.Find("bg") == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestBooksStaleFallback(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.svc.Books(ctx, "en"); err != nil {
		t.Fatal(err)
	}
	*e.now = e.now.Add(8 * 24 * time.Hour)
	e.srv.Close()
	books, err := e.svc.Books(ctx, "en")
	if err != nil || len(books) == 0 {
		t.Fatalf("books=%d err=%v", len(books), err)
	}
}

func TestPageExpiry(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	u := e.srv.URL + "/en/library/bg/2/13/"
	if _, err := e.svc.Page(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Page(ctx, u); err != nil || e.hits.Load() != 1 {
		t.Fatalf("fresh refetched: hits=%d err=%v", e.hits.Load(), err)
	}
	*e.now = e.now.Add(31 * 24 * time.Hour)
	if _, err := e.svc.Page(ctx, u); err != nil || e.hits.Load() != 2 {
		t.Fatalf("expired not refetched: hits=%d err=%v", e.hits.Load(), err)
	}
	*e.now = e.now.Add(31 * 24 * time.Hour)
	e.srv.Close()
	data, err := e.svc.Page(ctx, u)
	if err != nil || len(data) == 0 {
		t.Fatalf("stale fallback: len=%d err=%v", len(data), err)
	}
}

func TestPageKeysIncludeQuery(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	base := e.srv.URL + "/en/library/letters/"
	a, err := e.svc.Page(ctx, base+"?year=1967")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.svc.Page(ctx, base+"?year=1968")
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != "year=1967" || string(b) != "year=1968" || e.hits.Load() != 2 {
		t.Fatalf("a=%q b=%q hits=%d", a, b, e.hits.Load())
	}
	// both now cached separately
	a2, _ := e.svc.Page(ctx, base+"?year=1967")
	b2, _ := e.svc.Page(ctx, base+"?year=1968")
	if string(a2) != "year=1967" || string(b2) != "year=1968" || e.hits.Load() != 2 {
		t.Fatalf("a2=%q b2=%q hits=%d", a2, b2, e.hits.Load())
	}
	if n1, n2 := pageBlobName(base+"?year=1967"), pageBlobName(base+"?year=1967&page=2"); n1 == n2 {
		t.Error("page param ignored")
	}
}
