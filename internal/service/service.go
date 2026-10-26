// Package service joins the API client with the on-disk caches: items are
// cache-first, the library tree and HTML pages refresh on a schedule.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
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

	weekly = 7 * 24 * time.Hour
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
		if lang == "en" && ctx.Err() == nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) && !api.IsNotFound(err) {
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

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

const (
	tocFresh     = 24 * time.Hour
	listingFresh = weekly
	// letterPerPage is the page size for By year; the API allows up to 100.
	letterPerPage = 100
	lettersYears  = "list/letters/years.json"
)

// cachedJSON serves name from the blob cache while it is younger than ttl,
// else calls fetch and stores the result. When fetch fails, an older copy is
// returned instead, unless the caller gave up or the thing does not exist.
func cachedJSON[T any](ctx context.Context, s *Service, name string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	var zero T
	data, saved, ok := s.Blobs.Get(name)
	var old T
	haveOld := false
	if ok {
		if json.Unmarshal(data, &old) == nil {
			haveOld = true
			if s.now().Sub(saved) < ttl {
				return old, nil
			}
		}
	}
	v, err := fetch()
	if err != nil {
		if haveOld && ctx.Err() == nil && !api.IsNotFound(err) {
			return old, nil
		}
		return zero, err
	}
	if raw, merr := json.Marshal(v); merr == nil {
		_ = s.Blobs.Put(name, raw)
	}
	return v, nil
}

func tocBlob(code, lang string) string {
	return "toc/" + unsafeChars.ReplaceAllString(lang, "-") + "/" + unsafeChars.ReplaceAllString(code, "-") + ".json"
}

// TOC is a book's table of contents, cached for a day; if a refresh fails the
// older copy is used, so the contents also work offline.
func (s *Service) TOC(ctx context.Context, code, lang string) (api.TOC, error) {
	return cachedJSON(ctx, s, tocBlob(code, lang), tocFresh, func() (api.TOC, error) {
		return s.Client.TOC(ctx, code, lang)
	})
}

// CachedTOC reads the stored table of contents without touching the network.
func (s *Service) CachedTOC(code, lang string) (t api.TOC, fresh, ok bool) {
	data, saved, found := s.Blobs.Get(tocBlob(code, lang))
	if !found || json.Unmarshal(data, &t) != nil {
		return api.TOC{}, false, false
	}
	return t, s.now().Sub(saved) < tocFresh, true
}

// LetterYears gives the years that have letters, with their counts, and how
// many letters have no date.
func (s *Service) LetterYears(ctx context.Context) (years []api.YearCount, undated int, err error) {
	l, err := cachedJSON(ctx, s, lettersYears, listingFresh, func() (api.Listing, error) {
		return s.Client.Letters(ctx, 0, 1, 1)
	})
	return l.Years, l.Undated, err
}

// LetterYearPage is one page of a year's letters.
func (s *Service) LetterYearPage(ctx context.Context, year, page int) (api.Listing, error) {
	name := "list/letters/" + strconv.Itoa(year) + "-" + strconv.Itoa(page) + ".json"
	return cachedJSON(ctx, s, name, listingFresh, func() (api.Listing, error) {
		return s.Client.Letters(ctx, year, page, letterPerPage)
	})
}
