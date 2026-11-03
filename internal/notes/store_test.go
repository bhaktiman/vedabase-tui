package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)

func tmpPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "notes.json")
}

func mustOpen(t *testing.T, p string) *Store {
	t.Helper()
	s, w, err := Open(p)
	if err != nil || w != "" {
		t.Fatalf("Open: warning=%q err=%v", w, err)
	}
	return s
}

func TestRoundTrip(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	if err := s.Set("bg/2/13", "the soul is eternal", t0); err != nil {
		t.Fatal(err)
	}
	n, ok := mustOpen(t, p).Get("bg/2/13")
	if !ok || n.Text != "the soul is eternal" || !n.Updated.Equal(t0) {
		t.Fatalf("reopened: %+v %v", n, ok)
	}
	if !s.Has("bg/2/13") || s.Has("bg/2/14") {
		t.Fatal("Has")
	}
	raw, _ := os.ReadFile(p)
	if !strings.Contains(string(raw), `"version": 1`) || !strings.Contains(string(raw), `"updated": "2026-11-03T15:00:00Z"`) {
		t.Fatalf("format: %s", raw)
	}
}

func TestEmptyOrWhitespaceDeletes(t *testing.T) {
	for _, empty := range []string{"", "  \n\t ", "\n\n"} {
		p := tmpPath(t)
		s := mustOpen(t, p)
		s.Set("a/1", "x", t0)
		s.Set("a/2", "y", t0)
		if err := s.Set("a/1", empty, t0); err != nil {
			t.Fatal(err)
		}
		if s.Has("a/1") || !s.Has("a/2") {
			t.Fatalf("memory after delete %q", empty)
		}
		r := mustOpen(t, p)
		if r.Has("a/1") || !r.Has("a/2") {
			t.Fatalf("disk after delete %q", empty)
		}
	}
}

func TestDeleteAbsentWritesNothing(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	if err := s.Set("a/1", "  ", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("file written: %v", err)
	}
}

func TestUnchangedNoWrite(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Set("a/1", "x", t0)
	fi1, _ := os.Stat(p)
	time.Sleep(20 * time.Millisecond)
	if err := s.Set("a/1", "x", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	fi2, _ := os.Stat(p)
	if !os.SameFile(fi1, fi2) || !fi1.ModTime().Equal(fi2.ModTime()) {
		t.Fatal("file rewritten for unchanged text")
	}
	if n, _ := s.Get("a/1"); !n.Updated.Equal(t0) {
		t.Fatalf("Updated moved: %v", n.Updated)
	}
	// changed text does write and bumps Updated.
	s.Set("a/1", "x2", t0.Add(time.Hour))
	if n, _ := mustOpen(t, p).Get("a/1"); n.Text != "x2" || !n.Updated.Equal(t0.Add(time.Hour)) {
		t.Fatalf("%+v", n)
	}
}

func TestMultilineAndTrailingWhitespacePreserved(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	text := "line one\n\n  indented line\nlast \n"
	s.Set("a/1", text, t0)
	if n, _ := mustOpen(t, p).Get("a/1"); n.Text != text {
		t.Fatalf("text = %q", n.Text)
	}
}

func TestAllIsCopy(t *testing.T) {
	s := mustOpen(t, tmpPath(t))
	if m := s.All(); m == nil || len(m) != 0 {
		t.Fatalf("empty All = %#v", m)
	}
	s.Set("a/1", "x", t0)
	m := s.All()
	m["a/1"] = Note{Text: "mutated"}
	m["z"] = Note{}
	if n, _ := s.Get("a/1"); n.Text != "x" || s.Has("z") {
		t.Fatal("All leaked internal map")
	}
}

func TestMissingFileWritesNothing(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("file exists after Open: %v", err)
	}
	if len(s.All()) != 0 {
		t.Fatal("not empty")
	}
}

func TestMissingDirCreated(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "dir", "notes.json")
	s := mustOpen(t, p)
	if err := s.Set("a/1", "x", t0); err != nil {
		t.Fatal(err)
	}
	if !mustOpen(t, p).Has("a/1") {
		t.Fatal("not persisted")
	}
}

func TestCorruptFileMovedAside(t *testing.T) {
	p := tmpPath(t)
	os.WriteFile(p, []byte("{not json"), 0o644)
	s, w, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w, ".bad-") {
		t.Fatalf("warning = %q", w)
	}
	bads, _ := filepath.Glob(p + ".bad-*")
	if len(bads) != 1 {
		t.Fatalf("bad files: %v", bads)
	}
	if b, _ := os.ReadFile(bads[0]); string(b) != "{not json" {
		t.Fatalf("bad bytes changed: %q", b)
	}
	if len(s.All()) != 0 {
		t.Fatal("store should be empty")
	}
	if err := s.Set("a/1", "x", t0); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bads[0]); string(b) != "{not json" {
		t.Fatal("bad file touched")
	}
	if !mustOpen(t, p).Has("a/1") {
		t.Fatal("fresh file not valid")
	}
}

func TestNewerVersionMovedAsideNotOverwritten(t *testing.T) {
	p := tmpPath(t)
	orig := `{"version":2,"notes":{"a/1":{"text":"future","updated":"2026-11-03T15:00:00Z"}}}`
	os.WriteFile(p, []byte(orig), 0o644)
	s, w, err := Open(p)
	if err != nil || !strings.Contains(w, ".bad-") {
		t.Fatalf("w=%q err=%v", w, err)
	}
	bads, _ := filepath.Glob(p + ".bad-*")
	if len(bads) != 1 {
		t.Fatalf("bad files: %v", bads)
	}
	if b, _ := os.ReadFile(bads[0]); string(b) != orig {
		t.Fatalf("bad bytes changed: %q", b)
	}
	s.Set("a/2", "x", t0)
	if b, _ := os.ReadFile(bads[0]); string(b) != orig {
		t.Fatal("bad file touched")
	}
}

func TestMutationReloadOfCorruptFileKeepsItAside(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Set("a/1", "x", t0)
	garbage := "<<<garbage, longer than before>>>"
	os.WriteFile(p, []byte(garbage), 0o644)
	err := s.Set("a/2", "y", t0)
	if err == nil || !strings.Contains(err.Error(), ".bad-") {
		t.Fatalf("err = %v", err)
	}
	bads, _ := filepath.Glob(p + ".bad-*")
	if len(bads) != 1 {
		t.Fatalf("bad files: %v", bads)
	}
	if b, _ := os.ReadFile(bads[0]); string(b) != garbage {
		t.Fatalf("garbage not preserved: %q", b)
	}
	if s.Has("a/2") || !s.Has("a/1") {
		t.Fatal("memory changed")
	}
	if r := mustOpen(t, p); !r.Has("a/1") {
		t.Fatal("restored file missing note")
	}
	if err := s.Set("a/2", "y", t0); err != nil {
		t.Fatal(err)
	}
	if r := mustOpen(t, p); !r.Has("a/1") || !r.Has("a/2") {
		t.Fatal("persisted")
	}
}

func TestVanishedFileIsWrittenBack(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Set("a/1", "x", t0)
	os.Remove(p)
	s.Set("a/2", "y", t0)
	r := mustOpen(t, p)
	if !r.Has("a/1") || !r.Has("a/2") {
		t.Fatalf("persisted: %v", r.All())
	}
}

func TestWriteFailureKeepsMemory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "notes.json")
	s := mustOpen(t, p)
	s.Set("a/1", "x", t0)
	before, _ := os.ReadFile(p)
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	if err := s.Set("a/1", "changed", t0); err == nil {
		t.Fatal("expected error")
	}
	if err := s.Set("a/1", "", t0); err == nil {
		t.Fatal("expected error")
	}
	if n, _ := s.Get("a/1"); n.Text != "x" {
		t.Fatalf("memory changed: %+v", n)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("file changed")
	}
}

func TestTwoInstancesMerge(t *testing.T) {
	p := tmpPath(t)
	a := mustOpen(t, p)
	b := mustOpen(t, p)
	a.Set("bg/2/13", "from a", t0)
	b.Set("bg/2/14", "from b", t0)
	r := mustOpen(t, p)
	if !r.Has("bg/2/13") || !r.Has("bg/2/14") {
		t.Fatalf("got %v", r.All())
	}
	a.Set("bg/2/15", "again a", t0)
	r = mustOpen(t, p)
	if len(r.All()) != 3 {
		t.Fatalf("got %v", r.All())
	}
}

func TestExternalChangeThenNoopAdopts(t *testing.T) {
	p := tmpPath(t)
	a := mustOpen(t, p)
	b := mustOpen(t, p)
	b.Set("a/2", "from b", t0)
	if err := a.Set("a/9", "", t0); err != nil { // no-op, but adopts b's file
		t.Fatal(err)
	}
	if !a.Has("a/2") {
		t.Fatal("no-op mutation did not adopt external change")
	}
}

func TestNoTempFilesLeft(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Set("a/1", "x", t0)
	s.Set("a/1", "y", t0)
	s.Set("a/1", "", t0)
	ents, _ := os.ReadDir(filepath.Dir(p))
	if len(ents) != 1 || ents[0].Name() != "notes.json" {
		t.Fatalf("dir: %v", ents)
	}
}

func TestConcurrentSet(t *testing.T) {
	s := mustOpen(t, tmpPath(t))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ref := fmt.Sprintf("a/%d/%d", i, j)
				s.Set(ref, "t"+ref, t0)
				s.Get(ref)
				s.Has("a/0/0")
				s.All()
			}
		}(i)
	}
	wg.Wait()
	if n := len(s.All()); n != 80 {
		t.Fatalf("notes = %d", n)
	}
}

func TestRefreshAdoptsExternalChange(t *testing.T) {
	p := tmpPath(t)
	a := mustOpen(t, p)
	b := mustOpen(t, p)
	b.Set("a/2", "from b", t0)
	if a.Has("a/2") {
		t.Fatal("precondition")
	}
	if err := a.Refresh(); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.Get("a/2"); n.Text != "from b" {
		t.Fatalf("got %q", n.Text)
	}
}

func TestRefreshMovesCorruptFileAside(t *testing.T) {
	p := tmpPath(t)
	a := mustOpen(t, p)
	a.Set("a/1", "mine", t0)
	if err := os.WriteFile(p, []byte("{garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.Refresh(); err == nil || !strings.Contains(err.Error(), "kept as") {
		t.Fatalf("err = %v", err)
	}
	bad, _ := filepath.Glob(p + ".bad-*")
	if len(bad) != 1 {
		t.Fatalf("bad files %v", bad)
	}
	if n, _ := a.Get("a/1"); n.Text != "mine" {
		t.Fatal("memory lost")
	}
	if r := mustOpen(t, p); !r.Has("a/1") {
		t.Fatal("our copy not restored")
	}
}

func TestSetIf(t *testing.T) {
	p := tmpPath(t)
	a := mustOpen(t, p)
	b := mustOpen(t, p)
	// absent counts as ""
	if err := a.SetIf("a/1", "", "one", t0); err != nil {
		t.Fatal(err)
	}
	// B changed it under A: conflict, nothing written
	b.Set("a/1", "two", t0)
	if err := a.SetIf("a/1", "one", "three", t0); err != ErrConflict {
		t.Fatalf("err = %v", err)
	}
	if r := mustOpen(t, p); func() string { n, _ := r.Get("a/1"); return n.Text }() != "two" {
		t.Fatal("conflict wrote")
	}
	// matching orig after the reload: ok, and deleting works
	if err := a.SetIf("a/1", "two", "", t0); err != nil {
		t.Fatal(err)
	}
	if r := mustOpen(t, p); r.Has("a/1") {
		t.Fatal("not deleted")
	}
	// B wrote a note where A saw none: A's delete-of-nothing conflicts
	b.Set("a/2", "b's", t0)
	if err := a.SetIf("a/2", "", "", t0); err != ErrConflict {
		t.Fatalf("err = %v", err)
	}
	if r := mustOpen(t, p); !r.Has("a/2") {
		t.Fatal("b's note deleted")
	}
}
