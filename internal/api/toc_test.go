package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func serveFile(t *testing.T, file string, seen *string) *Client {
	t.Helper()
	b, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = r.URL.RequestURI()
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "ua", nil)
}

func TestTOCDecodes(t *testing.T) {
	var seen string
	c := serveFile(t, "toc-bg.json", &seen)
	toc, err := c.TOC(context.Background(), "bg", "en")
	if err != nil {
		t.Fatal(err)
	}
	if seen != "/api/v1/books/bg/toc?lang=en" {
		t.Errorf("request = %q", seen)
	}
	if toc.Book != "bg" || len(toc.Children) == 0 {
		t.Fatalf("toc = %+v", toc.Book)
	}
	ch1 := toc.Children[0]
	for _, n := range toc.Children {
		if n.Ref == "bg/1" {
			ch1 = n
		}
	}
	if ch1.Title == "" || ch1.Label != "" || len(ch1.Children) == 0 {
		t.Errorf("container = %+v", ch1.Ref)
	}
	leaf := ch1.Children[0]
	if leaf.Ref != "bg/1/1" || leaf.Label != "1.1" || leaf.Title != "" || leaf.Children != nil {
		t.Errorf("leaf = %+v", leaf)
	}
}

func TestTOCNullsAndSanitize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"book":"x","title":"T\u001b[31m","children":[{"ref":null,"title":null,"label":"a","children":null}]}`))
	}))
	defer srv.Close()
	toc, err := NewClient(srv.URL, "ua", nil).TOC(context.Background(), "x", "en")
	if err != nil {
		t.Fatal(err)
	}
	if toc.Title != "T[31m" {
		t.Errorf("title = %q", toc.Title)
	}
	n := toc.Children[0]
	if n.Ref != "" || n.Title != "" || n.Label != "a" || n.Children != nil {
		t.Errorf("node = %+v", n)
	}
}

func TestLettersListing(t *testing.T) {
	var seen string
	c := serveFile(t, "letters-1967.json", &seen)
	l, err := c.Letters(context.Background(), 1967, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if seen != "/api/v1/letters?page=1&per_page=3&year=1967" {
		t.Errorf("request = %q", seen)
	}
	if l.Total != 264 || !l.HasMore || l.Undated != 57 || len(l.Years) != 28 || l.Years[19] != (YearCount{1969, 760}) {
		t.Errorf("listing = %+v", l)
	}
	if len(l.Items) != 3 {
		t.Fatalf("items = %d", len(l.Items))
	}
	e := l.Items[1]
	if e.Ref != "letters/letter-to-janardana" || e.IsoDate != "1967-01-22" || e.Year != 1967 || e.Location != "" || len(e.Recipients) != 1 {
		t.Errorf("entry = %+v", e)
	}
}

func TestListingWithoutYear(t *testing.T) {
	var seen string
	c := serveFile(t, "letters-1967.json", &seen)
	if _, err := c.Transcripts(context.Background(), 0, 2, 100); err != nil {
		t.Fatal(err)
	}
	if seen != "/api/v1/transcripts?page=2&per_page=100" {
		t.Errorf("request = %q", seen)
	}
}

func TestTOCHasItsOwnLimiter(t *testing.T) {
	c := serveFile(t, "toc-iso.json", nil)
	for range 2 { // the burst
		if _, err := c.TOC(context.Background(), "iso", "en"); err != nil {
			t.Fatal(err)
		}
	}
	if c.toc == c.verse || c.toc == c.search {
		t.Fatal("TOC shares a limiter")
	}
	if got := c.verse.Tokens(); got < 4.9 {
		t.Errorf("verse limiter lost tokens to TOC calls: %v", got)
	}
}
