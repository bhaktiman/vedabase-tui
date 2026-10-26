package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	perPage      = 20
	maxErrorBody = 1 << 16
)

type Client struct {
	base   string
	hc     *http.Client
	ua     string
	verse  *rate.Limiter
	search *rate.Limiter
	toc    *rate.Limiter
	// RetryDelay is the pause before the single retry of a 5xx response.
	RetryDelay time.Duration
}

// NewClient takes the site root (e.g. "https://vedabase.cc"); API paths are added.
// Limiters stay under the server's 120 verse / 30 search requests per minute.
func NewClient(base, userAgent string, hc *http.Client) *Client {
	if hc == nil {
		hc = newHTTPClient(5*time.Second, 5*time.Second, 15*time.Second)
	}
	return &Client{
		base:       strings.TrimRight(base, "/"),
		hc:         hc,
		ua:         userAgent,
		verse:      rate.NewLimiter(rate.Every(time.Minute/100), 5),
		search:     rate.NewLimiter(rate.Every(time.Minute/25), 5),
		toc:        rate.NewLimiter(rate.Every(7*time.Second), 2),
		RetryDelay: 500 * time.Millisecond,
	}
}

// newHTTPClient bounds each phase of a request so a dead network fails in
// seconds; the overall 30 s stays for large answers. It starts from the
// default transport to keep proxy settings and HTTP/2.
func newHTTPClient(dial, tlsHandshake, header time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{Timeout: dial, KeepAlive: 30 * time.Second}).DialContext
	tr.TLSHandshakeTimeout = tlsHandshake
	tr.ResponseHeaderTimeout = header
	return &http.Client{Timeout: 30 * time.Second, Transport: tr}
}

func (c *Client) Verse(ctx context.Context, ref, lang string) (Item, error) {
	_, it, err := c.VerseRaw(ctx, ref, lang)
	return it, err
}

// VerseRaw also returns the response bytes as the server sent them, so a
// cache can keep fields this version does not know yet.
func (c *Client) VerseRaw(ctx context.Context, ref, lang string) ([]byte, Item, error) {
	var it Item
	raw, err := c.getRaw(ctx, c.verse, c.base+"/api/v1/verse/"+ref+"?lang="+url.QueryEscape(lang), &it)
	return raw, it, err
}

func (c *Client) Books(ctx context.Context, lang string) ([]Book, error) {
	var out struct {
		Books []Book `json:"books"`
	}
	if err := c.getJSON(ctx, c.verse, c.base+"/api/v1/books?lang="+url.QueryEscape(lang), &out); err != nil {
		return nil, err
	}
	return out.Books, nil
}

func (c *Client) Search(ctx context.Context, q, book string, page int, lang string) (SearchPage, error) {
	v := url.Values{}
	v.Set("q", q)
	if book != "" {
		v.Set("book", book)
	}
	v.Set("page", strconv.Itoa(page))
	v.Set("per_page", strconv.Itoa(perPage))
	v.Set("lang", lang)
	var sp SearchPage
	err := c.getJSON(ctx, c.search, c.base+"/api/v1/search?"+v.Encode(), &sp)
	return sp, err
}

// TOC fetches a book's table of contents. The server allows about 10 of these
// a minute, so they have a limiter of their own.
func (c *Client) TOC(ctx context.Context, code, lang string) (TOC, error) {
	var t TOC
	err := c.getJSON(ctx, c.toc, c.base+"/api/v1/books/"+url.PathEscape(code)+"/toc?lang="+url.QueryEscape(lang), &t)
	t.sanitize()
	return t, err
}

// Letters lists letters (English only). year <= 0 means all years.
func (c *Client) Letters(ctx context.Context, year, page, perPage int) (Listing, error) {
	return c.listing(ctx, "letters", year, page, perPage)
}

// Transcripts lists lecture transcripts, with the same shape as Letters.
func (c *Client) Transcripts(ctx context.Context, year, page, perPage int) (Listing, error) {
	return c.listing(ctx, "transcripts", year, page, perPage)
}

func (c *Client) listing(ctx context.Context, kind string, year, page, perPage int) (Listing, error) {
	v := url.Values{}
	if year > 0 {
		v.Set("year", strconv.Itoa(year))
	}
	v.Set("page", strconv.Itoa(page))
	v.Set("per_page", strconv.Itoa(perPage))
	var l Listing
	err := c.getJSON(ctx, c.verse, c.base+"/api/v1/"+kind+"?"+v.Encode(), &l)
	l.sanitize()
	return l, err
}

// Fetch downloads an absolute URL (site page or sitemap). progress may be nil;
// total is -1 when the server sends no Content-Length.
func (c *Client) Fetch(ctx context.Context, u string, progress func(done, total int64)) ([]byte, error) {
	resp, err := c.do(ctx, c.verse, u, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Asking for gzip ourselves turns off Go's transparent decompression, so
	// ContentLength stays the wire size and progress can be measured against it.
	total := resp.ContentLength
	var read int64
	var buf []byte
	chunk := make([]byte, 32*1024)
	for {
		n, rerr := resp.Body.Read(chunk)
		buf = append(buf, chunk[:n]...)
		read += int64(n)
		if n > 0 && progress != nil {
			progress(read, total)
		}
		if rerr == io.EOF {
			if resp.Header.Get("Content-Encoding") == "gzip" {
				zr, err := gzip.NewReader(bytes.NewReader(buf))
				if err != nil {
					return nil, c.transportErr(ctx, err)
				}
				out, err := io.ReadAll(zr)
				if err != nil {
					return nil, c.transportErr(ctx, err)
				}
				return out, nil
			}
			return buf, nil
		}
		if rerr != nil {
			return nil, c.transportErr(ctx, rerr)
		}
	}
}

func (c *Client) getJSON(ctx context.Context, lim *rate.Limiter, u string, out any) error {
	_, err := c.getRaw(ctx, lim, u, out)
	return err
}

// getRaw decodes the body into out and also returns the bytes.
func (c *Client) getRaw(ctx context.Context, lim *rate.Limiter, u string, out any) ([]byte, error) {
	resp, err := c.do(ctx, lim, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err == nil {
		if err = json.Unmarshal(raw, out); err != nil {
			err = fmt.Errorf("decode response: %w", err)
		}
	}
	if err != nil {
		return nil, c.transportErr(ctx, err)
	}
	return raw, nil
}

// do issues the GET, retries a 5xx once, and returns only 2xx responses.
func (c *Client) do(ctx context.Context, lim *rate.Limiter, u string, gz ...bool) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if err := wait(ctx, lim); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.ua)
		if len(gz) > 0 && gz[0] {
			req.Header.Set("Accept-Encoding", "gzip")
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			return nil, c.transportErr(ctx, err)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		apiErr := statusError(resp)
		resp.Body.Close()
		var ae *APIError
		if attempt == 0 && errors.As(apiErr, &ae) && ae.Status >= 500 {
			select {
			case <-time.After(c.RetryDelay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return nil, apiErr
	}
}

// wait blocks for a limiter token. rate.Limiter.Wait fails early with a
// non-context error when the deadline is too close, so reserve and wait here.
func wait(ctx context.Context, lim *rate.Limiter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r := lim.Reserve()
	d := r.Delay()
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		r.Cancel()
		return ctx.Err()
	}
}

func (c *Client) transportErr(ctx context.Context, err error) error {
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	return fmt.Errorf("%w: %v", ErrOffline, err)
}

func statusError(resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		secs, _ := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
		return &RateLimitedError{RetryAfter: time.Duration(secs) * time.Second}
	}
	ae := &APIError{Status: resp.StatusCode}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil {
		ae.Code, ae.Message = SanitizeText(env.Error.Code), SanitizeText(env.Error.Message)
	}
	if ae.Message == "" {
		ae.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return ae
}
