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

func TestSanitizeNumericControlRefs(t *testing.T) {
	for in, want := range map[string]string{
		"a&#27;b":                     "ab",
		"a&#x1b;b":                    "ab",
		"a&#X1B;b":                    "ab",
		"a&#7;b":                      "ab",
		"a&#127;b":                    "ab",
		"a&#155;b":                    "ab",
		"a&#x9b;b":                    "ab",
		"a&#27b":                      "ab",
		"a&#x1bzz":                    "azz",
		"a&#0027;b":                   "ab",
		"a&#&#27;27;b":                "ab",
		"a&#2\x007;b":                 "ab",
		"x &#27;]52;c;Y2xpcA==&#7; y": "x ]52;c;Y2xpcA== y",
		"keep &amp; &#8212; &#10; &#9; &#x41; &#160;": "keep &amp; &#8212; &#10; &#9; &#x41; &#160;",
	} {
		if got := SanitizeText(in); got != want {
			t.Errorf("SanitizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanNumericControlRefsEveryField(t *testing.T) {
	bad := "x&#27;]0;PWN&#7;y"
	it := Clean(Item{Ref: bad, Lang: bad, Book: bad, URL: bad, Devanagari: bad, VerseText: bad,
		Synonyms: bad, Translation: bad, Purport: bad, Format: bad,
		Document: &Document{Type: bad, Date: bad, Location: bad}})
	fields := []string{it.Ref, it.Lang, it.Book, it.URL, it.Devanagari, it.VerseText, it.Synonyms,
		it.Translation, it.Purport, it.Format, it.Document.Type, it.Document.Date, it.Document.Location}
	for i, f := range fields {
		if f != "x]0;PWNy" {
			t.Errorf("field %d = %q", i, f)
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
