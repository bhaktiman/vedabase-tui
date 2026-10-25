package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const repo = "o/r"

func tarGz(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte(content))
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func zipOf(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(name)
	w.Write([]byte(content))
	zw.Close()
	return buf.Bytes()
}

func sumLine(asset string, data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:]) + "  " + asset + "\n"
}

// release serves checksums.txt and assets under /o/r/releases/download/v0.2.0/.
func release(t *testing.T, files map[string][]byte) (*Updater, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := strings.CutPrefix(r.URL.Path, "/"+repo+"/releases/download/v0.2.0/")
		b, found := files[name]
		if !ok || !found {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	exe := filepath.Join(dir, "vedabase")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Updater{Repo: repo, Base: srv.URL, Current: "v0.1.0", Exe: exe, GOOS: "linux", GOARCH: "amd64"}, dir
}

const linuxAsset = "vedabase_linux_amd64.tar.gz"

func linuxRelease(t *testing.T) (*Updater, string, []byte) {
	arc := tarGz(t, "vedabase", "new")
	u, dir := release(t, map[string][]byte{linuxAsset: arc, "checksums.txt": []byte(sumLine(linuxAsset, arc))})
	return u, dir, arc
}

func assertUntouched(t *testing.T, u *Updater, dir string) {
	t.Helper()
	if b, _ := os.ReadFile(u.Exe); string(b) != "old" {
		t.Errorf("exe = %q, want old", b)
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".vedabase-update-") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

func latestServer(t *testing.T, status int, loc string, hits *atomic.Int32) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if loc != "" {
			w.Header().Set("Location", loc)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAvailable(t *testing.T) {
	tests := []struct {
		name, current string
		status        int
		loc, want     string
		wantErr       bool
		noHit         bool
	}{
		{"newer", "v0.1.0", 302, "/o/r/releases/tag/v0.2.0", "v0.2.0", false, false},
		{"absolute", "v0.1.0", 302, "https://github.com/o/r/releases/tag/v0.2.0", "v0.2.0", false, false},
		{"same", "v0.2.0", 302, "/o/r/releases/tag/v0.2.0", "", false, false},
		{"dev", "dev", 302, "/o/r/releases/tag/v0.2.0", "", false, true},
		{"traversal", "v0.1.0", 302, "/o/r/releases/tag/v0.2.0/../x", "", true, false},
		{"latest", "v0.1.0", 302, "/o/r/releases/tag/latest", "", true, false},
		{"ok200", "v0.1.0", 200, "", "", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := latestServer(t, tc.status, tc.loc, &hits)
			u := &Updater{Repo: repo, Base: srv.URL, Current: tc.current}
			got, err := u.Available(context.Background())
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("got %q, %v; want %q, err=%v", got, err, tc.want, tc.wantErr)
			}
			if tc.noHit && hits.Load() != 0 {
				t.Error("server was hit")
			}
		})
	}
}

func TestNewer(t *testing.T) {
	tests := []struct {
		cur, latest string
		want        bool
	}{
		{"v0.1.0", "v0.1.1", true},
		{"v0.9.9", "v0.10.0", true},
		{"v0.10.0", "v0.9.9", false},
		{"v0.99.99", "v1.0.0", true},
		{"v1.0.0", "v1.0.0", false},
		{"dev", "v1.0.0", false},
		{"v1.0.0", "dev", false},
		{"v1.0", "v1.0.1", false},
		{"garbage", "junk", false},
	}
	for _, tc := range tests {
		if got := Newer(tc.cur, tc.latest); got != tc.want {
			t.Errorf("Newer(%q,%q) = %v", tc.cur, tc.latest, got)
		}
	}
}

func TestApplyHappyPath(t *testing.T) {
	u, dir, arc := linuxRelease(t)
	var done, total int64
	if err := u.Apply(context.Background(), "v0.2.0", func(d, tt int64) { done, total = d, tt }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(u.Exe); string(b) != "new" {
		t.Errorf("exe = %q", b)
	}
	if st, _ := os.Stat(u.Exe); runtime.GOOS != "windows" && st.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v", st.Mode())
	}
	if done != int64(len(arc)) || total != int64(len(arc)) {
		t.Errorf("progress done=%d total=%d, want %d", done, total, len(arc))
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Errorf("dir has %d entries, want 1", len(ents))
	}
}

func TestApplyChecksumMismatch(t *testing.T) {
	arc := tarGz(t, "vedabase", "new")
	u, dir := release(t, map[string][]byte{linuxAsset: arc, "checksums.txt": []byte(sumLine(linuxAsset, []byte("other")))})
	err := u.Apply(context.Background(), "v0.2.0", nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
	assertUntouched(t, u, dir)
}

func TestApplyFailures(t *testing.T) {
	good := tarGz(t, "vedabase", "new")
	noBin := tarGz(t, "other", "new")
	tests := map[string]map[string][]byte{
		"not in checksums": {linuxAsset: good, "checksums.txt": []byte(sumLine("vedabase_linux_arm64.tar.gz", good))},
		"no binary":        {linuxAsset: noBin, "checksums.txt": []byte(sumLine(linuxAsset, noBin))},
		"nested binary":    {linuxAsset: tarGz(t, "dir/vedabase", "x"), "checksums.txt": []byte(sumLine(linuxAsset, tarGz(t, "dir/vedabase", "x")))},
		"asset 404":        {"checksums.txt": []byte(sumLine(linuxAsset, good))},
		"checksums 404":    {linuxAsset: good},
	}
	for name, files := range tests {
		t.Run(name, func(t *testing.T) {
			u, dir := release(t, files)
			if err := u.Apply(context.Background(), "v0.2.0", nil); err == nil {
				t.Fatal("want error")
			}
			assertUntouched(t, u, dir)
		})
	}
}

func TestApplyReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not apply")
	}
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	u, dir, _ := linuxRelease(t)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	err := u.Apply(context.Background(), "v0.2.0", nil)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want ErrPermission", err)
	}
	if b, _ := os.ReadFile(u.Exe); string(b) != "old" {
		t.Errorf("exe = %q", b)
	}
}

func TestApplyWindows(t *testing.T) {
	const asset = "vedabase_windows_amd64.zip"
	arc := zipOf(t, "vedabase.exe", "new")
	u, _ := release(t, map[string][]byte{asset: arc, "checksums.txt": []byte(sumLine(asset, arc))})
	u.GOOS = "windows"
	if err := u.Apply(context.Background(), "v0.2.0", nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(u.Exe); string(b) != "new" {
		t.Errorf("exe = %q", b)
	}
	if b, _ := os.ReadFile(u.Exe + ".old"); string(b) != "old" {
		t.Errorf(".old = %q", b)
	}
	cleanup(u.Exe, "windows", time.Now())
	if _, err := os.Stat(u.Exe + ".old"); !os.IsNotExist(err) {
		t.Errorf(".old still present: %v", err)
	}
}

func TestApplyBadTag(t *testing.T) {
	var hits atomic.Int32
	srv := latestServer(t, 200, "", &hits)
	u := &Updater{Repo: repo, Base: srv.URL, Exe: filepath.Join(t.TempDir(), "vedabase")}
	for _, tag := range []string{"../../evil", "latest", "v1.0", ""} {
		if err := u.Apply(context.Background(), tag, nil); err == nil {
			t.Errorf("tag %q: want error", tag)
		}
	}
	if hits.Load() != 0 {
		t.Error("server was hit")
	}
}

func TestCleanupStaleTemps(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "vedabase")
	stale, fresh := filepath.Join(dir, ".vedabase-update-old"), filepath.Join(dir, ".vedabase-update-new")
	other := filepath.Join(dir, "keep.txt")
	for _, p := range []string{exe, stale, fresh, other} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, past, past); err != nil {
		t.Fatal(err)
	}
	Cleanup(exe)
	for p, want := range map[string]bool{stale: false, fresh: true, other: true, exe: true} {
		if _, err := os.Stat(p); (err == nil) != want {
			t.Errorf("%s exists=%v, want %v", filepath.Base(p), err == nil, want)
		}
	}
}

func TestCleanupOldOnlyOnWindows(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "vedabase")
	if err := os.WriteFile(exe+".old", []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	cleanup(exe, "linux", time.Now())
	if _, err := os.Stat(exe + ".old"); err != nil {
		t.Fatalf("a user's vedabase.old was removed off Windows: %v", err)
	}
	cleanup(exe, "windows", time.Now())
	if _, err := os.Stat(exe + ".old"); err == nil {
		t.Fatal(".old kept on Windows")
	}
}

func TestInstallPartialKeepsTemp(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "vedabase.exe")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := &Updater{Exe: exe}
	realRename, realWait, realTries := rename, renameWait, renameTries
	t.Cleanup(func() { rename, renameWait, renameTries = realRename, realWait, realTries })
	renameWait, renameTries = 0, 2
	rename = func(from, to string) error {
		if to == exe { // installing the new copy and restoring the old one both fail
			return errors.New("locked")
		}
		return os.Rename(from, to)
	}
	err := u.install([]byte("new"), true)
	if !errors.Is(err, ErrPartial) {
		t.Fatalf("err = %v, want ErrPartial", err)
	}
	var tmp string
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".vedabase-update-") {
			tmp = filepath.Join(dir, e.Name())
		}
	}
	if b, _ := os.ReadFile(tmp); tmp == "" || string(b) != "new" {
		t.Fatalf("temp copy lost: %q", tmp)
	}
	if b, _ := os.ReadFile(exe + ".old"); string(b) != "old" {
		t.Fatalf(".old = %q", b)
	}
	if msg := err.Error(); !strings.Contains(msg, tmp) || !strings.Contains(msg, exe+".old") || !strings.Contains(msg, "vedabase.exe") {
		t.Fatalf("message %q lacks the paths", msg)
	}
}
