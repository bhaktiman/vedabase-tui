// Package update checks GitHub for a newer vedabase release and replaces the
// running binary with it.
package update

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var tagRe = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// Updater checks GitHub for a newer release and replaces the running binary with it.
type Updater struct {
	Repo         string       // "bhaktiman/vedabase-tui"
	Base         string       // "https://github.com" when empty; tests point it at httptest
	Current      string       // the running version, e.g. "v0.1.0"; anything not vX.Y.Z never updates
	Exe          string       // path of the binary to replace (already symlink-resolved)
	Client       *http.Client // http.DefaultClient when nil
	GOOS, GOARCH string       // runtime.GOOS/GOARCH when empty
}

func (u *Updater) base() string {
	if u.Base == "" {
		return "https://github.com"
	}
	return strings.TrimRight(u.Base, "/")
}

func (u *Updater) client() *http.Client {
	if u.Client == nil {
		return http.DefaultClient
	}
	return u.Client
}

func (u *Updater) platform() (goos, goarch string) {
	goos, goarch = u.GOOS, u.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}

// Available returns the latest release tag when it is newer than Current, or "".
func (u *Updater) Available(ctx context.Context) (string, error) {
	if !tagRe.MatchString(u.Current) {
		return "", nil
	}
	c := *u.client() // copy: never mutate the caller's client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	url := u.base() + "/" + u.Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		return "", fmt.Errorf("update: %s: expected a redirect, got %s", url, resp.Status)
	}
	loc := resp.Header.Get("Location")
	_, tag, ok := strings.Cut(loc, "/releases/tag/")
	if !ok || !tagRe.MatchString(tag) {
		return "", fmt.Errorf("update: unexpected release location %q", loc)
	}
	if !Newer(u.Current, tag) {
		return "", nil
	}
	return tag, nil
}

// Release reports whether v is a release tag (vMAJOR.MINOR.PATCH) that can be
// updated from, as opposed to a development build such as "dev".
func Release(v string) bool { return tagRe.MatchString(v) }

// Newer reports whether latest is a higher vMAJOR.MINOR.PATCH than current.
func Newer(current, latest string) bool {
	c, ok1 := parseVersion(current)
	l, ok2 := parseVersion(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parseVersion(v string) (p [3]int, ok bool) {
	if !tagRe.MatchString(v) {
		return p, false
	}
	for i, s := range strings.Split(v[1:], ".") {
		n, err := strconv.Atoi(s)
		if err != nil {
			return p, false
		}
		p[i] = n
	}
	return p, true
}

// Executable returns the running binary's path with symlinks resolved.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// staleTemp is how old a leftover .vedabase-update-* file must be before
// Cleanup removes it; a younger one may belong to an update in progress.
const staleTemp = time.Hour

// Cleanup removes leftovers of earlier updates: on Windows the exe+".old" a
// swap leaves behind, and on every platform temp files older than an hour.
// Errors are ignored.
func Cleanup(exe string) { cleanup(exe, runtime.GOOS, time.Now()) }

func cleanup(exe, goos string, now time.Time) {
	if goos == "windows" {
		_ = os.Remove(exe + ".old")
	}
	dir := filepath.Dir(exe)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !strings.HasPrefix(e.Name(), ".vedabase-update-") {
			continue
		}
		if fi, err := e.Info(); err == nil && fi.Mode().IsRegular() && now.Sub(fi.ModTime()) > staleTemp {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
