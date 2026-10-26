// Package service joins the API client with the on-disk caches: items are
// cache-first, the library tree and HTML pages refresh on a schedule.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/library"
	"vedabase-tui/internal/store"
)

const (
	booksBlob   = "books.json"
	sitemapBlob = "sitemap-en-books.xml"
	// DefaultBase is the site root used when Service.Base is empty.
	DefaultBase = "https://vedabase.cc"

	weekly     = 7 * 24 * time.Hour
	pageExpiry = 30 * 24 * time.Hour
)

type Service struct {
	Client *api.Client
	Items  store.ItemCache
	Blobs  store.BlobCache
	Now    func() time.Time
	// Base is the site root the sitemap is fetched from; empty means DefaultBase.
	Base string
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) sitemapURL() string {
	b := s.Base
	if b == "" {
		b = DefaultBase
	}
	return strings.TrimRight(b, "/") + "/" + sitemapBlob
}

// langFor is the language a ref is read in: letters and transcripts exist in
// English only, whatever the reader asked for.
func langFor(ref, lang string) string {
	if strings.HasPrefix(ref, "letters/") || strings.HasPrefix(ref, "transcripts/") {
		return "en"
	}
	return lang
}

// Item returns the cleaned item, from the cache when present, else fetched
// and stored as the raw response (Clean runs on every read so its rules can
// change). A failed fetch while offline falls back to the English cache of
// earlier versions.
func (s *Service) Item(ctx context.Context, ref, lang string) (api.Item, bool, error) {
	lang = langFor(ref, lang)
	if raw, ok := s.Items.GetRaw(lang, ref); ok {
		var it api.Item
		if json.Unmarshal(raw, &it) == nil {
			// Cached under the requested ref: the API may answer with a
			// canonical form, and Cached looks the item up by the requested one.
			it.Ref = ref
			return api.Clean(it), true, nil
		}
	}
	raw, it, err := s.Client.VerseRaw(ctx, ref, lang)
	if err != nil {
		if lang == "en" && errors.Is(err, api.ErrOffline) {
			if old, ok := s.Items.GetOld(ref); ok {
				old.Ref = ref
				return api.Clean(old), true, nil
			}
		}
		return api.Item{}, false, err
	}
	// A failed cache write must not hide an item we already have.
	_ = s.Items.PutRaw(lang, ref, raw)
	return api.Clean(it), false, nil
}

func (s *Service) Cached(ref, lang string) bool { return s.Items.Has(langFor(ref, lang), ref) }

// Books is cached for a week; on a refresh error the stale copy is used.
func (s *Service) Books(ctx context.Context, lang string) ([]api.Book, error) {
	blob := booksBlobFor(lang)
	data, saved, ok := s.Blobs.Get(blob)
	if ok && s.now().Sub(saved) < weekly {
		if books, err := decodeBooks(data); err == nil {
			return books, nil
		}
	}
	books, err := s.Client.Books(ctx, lang)
	if err != nil {
		if ok {
			if old, derr := decodeBooks(data); derr == nil {
				return old, nil
			}
		}
		return nil, err
	}
	if raw, merr := json.Marshal(struct {
		Books []api.Book `json:"books"`
	}{books}); merr == nil {
		_ = s.Blobs.Put(blob, raw)
	}
	return books, nil
}

// booksBlobFor keeps English at the name earlier versions used.
func booksBlobFor(lang string) string {
	if lang == "en" || lang == "" {
		return booksBlob
	}
	return "books-" + unsafeChars.ReplaceAllString(lang, "-") + ".json"
}

func decodeBooks(data []byte) ([]api.Book, error) {
	var out struct {
		Books []api.Book `json:"books"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out.Books, nil
}

// libraryBooks prefers any cached books list, however old: the root order
// rarely changes and Library must not hit the network when it has a tree.
func (s *Service) libraryBooks(ctx context.Context) ([]api.Book, error) {
	if data, _, ok := s.Blobs.Get(booksBlob); ok {
		if books, err := decodeBooks(data); err == nil {
			return books, nil
		}
	}
	return s.Books(ctx, "en")
}

// Library returns the tree. A cached sitemap is used at once (stale reports
// it is older than a week); with none cached it is downloaded with progress.
func (s *Service) Library(ctx context.Context, progress func(done, total int64)) (*library.Library, bool, error) {
	books, err := s.libraryBooks(ctx)
	if err != nil {
		return nil, false, err
	}
	if data, saved, ok := s.Blobs.Get(sitemapBlob); ok {
		if lib, perr := library.Parse(bytes.NewReader(data), books); perr == nil {
			return lib, s.now().Sub(saved) >= weekly, nil
		}
	}
	data, err := s.Client.Fetch(ctx, s.sitemapURL(), progress)
	if err != nil {
		return nil, false, err
	}
	lib, err := library.Parse(bytes.NewReader(data), books)
	if err != nil {
		return nil, false, err
	}
	_ = s.Blobs.Put(sitemapBlob, data)
	return lib, false, nil
}

// RefreshLibrary re-downloads the sitemap; the old copy stays if it fails.
func (s *Service) RefreshLibrary(ctx context.Context) (*library.Library, error) {
	books, err := s.libraryBooks(ctx)
	if err != nil {
		return nil, err
	}
	data, err := s.Client.Fetch(ctx, s.sitemapURL(), nil)
	if err != nil {
		return nil, err
	}
	lib, err := library.Parse(bytes.NewReader(data), books)
	if err != nil {
		return nil, err
	}
	_ = s.Blobs.Put(sitemapBlob, data)
	return lib, nil
}

// Search is never cached.
func (s *Service) Search(ctx context.Context, q, book string, page int, lang string) (api.SearchPage, error) {
	return s.Client.Search(ctx, q, book, page, lang)
}

// Page returns scraped HTML, refetched after 30 days; if the refetch fails
// the stale copy is returned (except when ctx was cancelled).
func (s *Service) Page(ctx context.Context, pageURL string) ([]byte, error) {
	name := pageBlobName(pageURL)
	data, saved, ok := s.Blobs.Get(name)
	if ok && s.now().Sub(saved) < pageExpiry {
		return data, nil
	}
	fresh, err := s.Client.Fetch(ctx, pageURL, nil)
	if err != nil {
		if ok && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return data, nil
		}
		return nil, err
	}
	_ = s.Blobs.Put(name, fresh)
	return fresh, nil
}

// pageBlobName keys a page by path and sorted query, so ?year=1967 and
// ?year=1968 of the same list page are separate blobs.
func pageBlobName(pageURL string) string {
	p, q := pageURL, ""
	if u, err := url.Parse(pageURL); err == nil {
		p = u.Path
		vals := u.Query()
		keys := make([]string, 0, len(vals))
		for k := range vals {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, v := range vals[k] {
				q += "_" + unsafeChars.ReplaceAllString(k+"-"+v, "-")
			}
		}
	}
	p = strings.Trim(p, "/")
	if p == "" {
		p = "index"
	}
	if q != "" {
		p += "_" + q
	}
	return "pages/" + strings.ReplaceAll(p, "..", "_") + ".html"
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)
