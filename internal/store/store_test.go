package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"vedabase-tui/internal/api"
)

func TestDefaultPathsXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/x/c")
	t.Setenv("XDG_CONFIG_HOME", "/x/k")
	t.Setenv("XDG_DATA_HOME", "/x/d")
	t.Setenv("XDG_STATE_HOME", "/x/s")
	p, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	want := Paths{Cache: "/x/c/vedabase-tui", Data: "/x/d/vedabase-tui", Config: "/x/k/vedabase-tui", State: "/x/s/vedabase-tui"}
	if p != want {
		t.Fatalf("got %+v want %+v", p, want)
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/h")
	p, _ = DefaultPaths()
	if p.Data != "/h/.local/share/vedabase-tui" || p.State != "/h/.local/state/vedabase-tui" {
		t.Fatalf("fallbacks: %+v", p)
	}
}

func TestItemCacheRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "items")
	c := ItemCache{Dir: dir}
	it := api.Item{Ref: "bg/2/13", Book: "bg", Translation: "As the embodied soul..."}
	if err := c.Put(it); err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get("bg/2/13")
	if !ok || !reflect.DeepEqual(got, it) {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	if !c.Has("bg/2/13") || c.Has("cc/madhya/9/74") {
		t.Fatal("Has wrong")
	}
	if _, err := os.Stat(filepath.Join(dir, "bg", "2", "13.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCachesRejectEscapingNames(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cache")
	ic, bc := ItemCache{Dir: dir}, BlobCache{Dir: dir}
	for _, bad := range []string{"../evil", "a/../../evil", "/abs/path", `a\b`, "", ".."} {
		if err := ic.Put(api.Item{Ref: bad}); err == nil {
			t.Errorf("ItemCache.Put(%q) accepted", bad)
		}
		if err := bc.Put(bad, []byte("x")); err == nil {
			t.Errorf("BlobCache.Put(%q) accepted", bad)
		}
		if _, ok := ic.Get(bad); ok {
			t.Errorf("ItemCache.Get(%q) ok", bad)
		}
		if ic.Has(bad) {
			t.Errorf("ItemCache.Has(%q) true", bad)
		}
		if _, _, ok := bc.Get(bad); ok {
			t.Errorf("BlobCache.Get(%q) ok", bad)
		}
	}
	// Even a file that exists outside must not be readable via traversal.
	os.WriteFile(filepath.Join(root, "secret"), []byte("s"), 0o644)
	if _, _, ok := bc.Get("../secret"); ok {
		t.Error("read outside cache dir")
	}
	ents, _ := os.ReadDir(root)
	for _, e := range ents {
		if e.Name() == "evil" || e.Name() == "evil.json" {
			t.Errorf("escaped file %s", e.Name())
		}
	}
}

func TestBlobCacheSavedTime(t *testing.T) {
	c := BlobCache{Dir: t.TempDir()}
	if err := c.Put("pages/a/b.html", []byte("hi")); err != nil {
		t.Fatal(err)
	}
	data, saved, ok := c.Get("pages/a/b.html")
	if !ok || string(data) != "hi" {
		t.Fatalf("got %q ok=%v", data, ok)
	}
	if d := time.Since(saved); d < -time.Second || d > time.Second {
		t.Fatalf("saved %v", saved)
	}
	if _, _, ok := c.Get("missing"); ok {
		t.Fatal("missing ok")
	}
}

func TestWriteFileAtomicNoPartial(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "c.json")
	if err := WriteFileAtomic(p, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("new")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "new" {
		t.Fatalf("content %q", b)
	}
	ents, _ := os.ReadDir(filepath.Dir(p))
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover %s", e.Name())
		}
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v", st.Mode())
	}
}

func TestSettingsDefaultsAndTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := filepath.Join(t.TempDir(), "cfg", "config.toml")
	s, err := LoadSettings(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.ExportDir != filepath.Join(home, "vedabase-exports") || !s.Animations || !s.Mouse {
		t.Fatalf("defaults: %+v", s)
	}
	b, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(b), "export_dir") || !strings.Contains(string(b), "animations") {
		t.Fatalf("default file: %q %v", b, err)
	}
	// The generated file must itself load as defaults.
	if s2, err := LoadSettings(p); err != nil || s2 != s {
		t.Fatalf("reload: %+v %v", s2, err)
	}
	os.WriteFile(p, []byte("export_dir = \"~/notes/vb\"\nanimations = false\nmouse = false\n"), 0o644)
	s, err = LoadSettings(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.ExportDir != filepath.Join(home, "notes/vb") || s.Animations || s.Mouse {
		t.Fatalf("custom: %+v", s)
	}
}

func TestStateRoundTripAndCorrupt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	want := State{LastRef: "bg/2/13", Scroll: 7, Parts: api.Parts{Translation: true}, TreeHidden: true}
	if err := SaveState(p, want); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(p); got != want {
		t.Fatalf("got %+v", got)
	}
	os.WriteFile(p, []byte("{not json"), 0o644)
	if got := LoadState(p); got.Parts != api.AllParts() || got.LastRef != "" {
		t.Fatalf("corrupt: %+v", got)
	}
	if got := LoadState(filepath.Join(t.TempDir(), "none")); got.Parts != api.AllParts() {
		t.Fatalf("missing: %+v", got)
	}
}
