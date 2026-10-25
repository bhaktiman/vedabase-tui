package collections

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tmpPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "collections.json")
}

func mustOpen(t *testing.T, p string) *Store {
	t.Helper()
	s, w, err := Open(p)
	if err != nil || w != "" {
		t.Fatalf("Open: warning=%q err=%v", w, err)
	}
	return s
}

func item(ref string) Item {
	return Item{Ref: ref, Book: "bg", Label: "label " + ref, Added: time.Unix(1700000000, 0).UTC()}
}

func TestAddCreatesAndPersists(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	added, err := s.Add("On the soul", item("bg/2/13"))
	if err != nil || !added {
		t.Fatalf("Add = %v, %v", added, err)
	}
	l := mustOpen(t, p).List()
	if len(l) != 1 || l[0].Name != "On the soul" || len(l[0].Items) != 1 {
		t.Fatalf("reopened: %+v", l)
	}
	if l[0].Items[0].Label != "label bg/2/13" || l[0].Created.IsZero() {
		t.Fatalf("item/created lost: %+v", l[0])
	}
}

func TestAddDuplicateIsNoop(t *testing.T) {
	s := mustOpen(t, tmpPath(t))
	s.Add("X", item("bg/1/16-18"))
	if added, err := s.Add("x", item("bg/1/16-18")); added || err != nil {
		t.Fatalf("dup Add = %v, %v", added, err)
	}
	if added, err := s.Add("X", item("bg/1/17")); !added || err != nil {
		t.Fatalf("Add 1/17 = %v, %v", added, err)
	}
	c, _ := s.Get("X")
	if len(c.Items) != 2 {
		t.Fatalf("items = %+v", c.Items)
	}
}

func TestContains(t *testing.T) {
	s := mustOpen(t, tmpPath(t))
	s.Add("A", item("bg/1/16-18"))
	s.Add("B", item("sb/1/1/1"))
	if !s.Contains("bg/1/16-18") || !s.Contains("sb/1/1/1") {
		t.Fatal("should contain")
	}
	if s.Contains("bg/1/17") || s.Contains("") {
		t.Fatal("exact ref only")
	}
}

func TestRenameDeleteRemove(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Add("On the soul", item("bg/2/13"))
	s.Add("On the soul", item("bg/2/14"))
	s.Add("Other", item("bg/3/1"))
	if err := s.Rename("on the soul", "other"); err == nil {
		t.Fatal("rename onto existing name must fail")
	}
	if err := s.Rename("on the soul", "  "); err == nil {
		t.Fatal("blank rename must fail")
	}
	if err := s.Rename("missing", "Z"); err == nil {
		t.Fatal("rename of missing must fail")
	}
	if err := s.Rename("on the soul", " Soul "); err != nil {
		t.Fatal(err)
	}
	if c, ok := s.Get("SOUL"); !ok || c.Name != "Soul" || len(c.Items) != 2 {
		t.Fatalf("Get after rename: %+v %v", c, ok)
	}
	if err := s.Rename("Soul", "SOUL"); err != nil { // case-only change of itself
		t.Fatal(err)
	}
	if err := s.Remove("soul", "bg/2/13"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("soul", "bg/2/13"); err == nil {
		t.Fatal("removing absent item should error")
	}
	if err := s.Delete("other"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("other"); err == nil {
		t.Fatal("deleting absent should error")
	}
	l := mustOpen(t, p).List()
	if len(l) != 1 || l[0].Name != "SOUL" || len(l[0].Items) != 1 || l[0].Items[0].Ref != "bg/2/14" {
		t.Fatalf("persisted: %+v", l)
	}
}

func TestListIsCopyInCreationOrder(t *testing.T) {
	s := mustOpen(t, tmpPath(t))
	s.Add("B", item("a/1"))
	s.Add("A", item("a/2"))
	l := s.List()
	if l[0].Name != "B" || l[1].Name != "A" {
		t.Fatalf("order: %+v", l)
	}
	l[0].Items[0].Ref = "mutated"
	l[0].Name = "mutated"
	if c, _ := s.Get("B"); c.Items[0].Ref != "a/1" {
		t.Fatal("List leaked internal slice")
	}
}

func TestBlankNameRejected(t *testing.T) {
	s := mustOpen(t, tmpPath(t))
	if _, err := s.Add("   ", item("a/1")); err == nil {
		t.Fatal("blank name must fail")
	}
}

func TestMissingFileWritesNothing(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("file exists after Open: %v", err)
	}
	if len(s.List()) != 0 {
		t.Fatal("not empty")
	}
	s.Add("X", item("a/1"))
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
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
	if len(s.List()) != 0 {
		t.Fatal("store should be empty")
	}
	if _, err := s.Add("X", item("a/1")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bads[0]); string(b) != "{not json" {
		t.Fatal("bad file touched")
	}
	if len(mustOpen(t, p).List()) != 1 {
		t.Fatal("fresh file not valid")
	}
}

func TestUnsupportedVersionMovedAside(t *testing.T) {
	p := tmpPath(t)
	orig := `{"version":2,"collections":[]}`
	os.WriteFile(p, []byte(orig), 0o644)
	_, w, err := Open(p)
	if err != nil || !strings.Contains(w, ".bad-") {
		t.Fatalf("w=%q err=%v", w, err)
	}
}

func TestMutationReloadOfCorruptFileKeepsItAside(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Add("X", item("a/1"))
	// Another process leaves garbage behind.
	garbage := "<<<garbage, longer than before>>>"
	os.WriteFile(p, []byte(garbage), 0o644)
	added, err := s.Add("X", item("a/2"))
	if err == nil || added {
		t.Fatalf("Add = %v, %v; want error", added, err)
	}
	if !strings.Contains(err.Error(), ".bad-") {
		t.Fatalf("error should name the aside file: %v", err)
	}
	bads, _ := filepath.Glob(p + ".bad-*")
	if len(bads) != 1 {
		t.Fatalf("bad files: %v", bads)
	}
	if b, _ := os.ReadFile(bads[0]); string(b) != garbage {
		t.Fatalf("garbage not preserved: %q", b)
	}
	if c, _ := s.Get("X"); len(c.Items) != 1 {
		t.Fatalf("memory changed: %+v", c)
	}
	// Garbage was not overwritten by a write.
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("original path should be vacated: %v", err)
	}
	// Memory is the only copy; the next mutation writes it back.
	if added, err := s.Add("X", item("a/2")); err != nil || !added {
		t.Fatalf("retry = %v, %v", added, err)
	}
	if c, _ := mustOpen(t, p).Get("X"); len(c.Items) != 2 {
		t.Fatalf("persisted: %+v", c)
	}
}

func TestVanishedFileIsWrittenBack(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Add("X", item("a/1"))
	os.Remove(p)
	s.Add("X", item("a/2"))
	if c, _ := mustOpen(t, p).Get("X"); len(c.Items) != 2 {
		t.Fatalf("persisted: %+v", c)
	}
}

func TestWriteFailureKeepsMemory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "collections.json")
	s := mustOpen(t, p)
	s.Add("X", item("a/1"))
	before, _ := os.ReadFile(p)
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	if _, err := s.Add("X", item("a/2")); err == nil {
		t.Fatal("expected error")
	}
	if err := s.Rename("X", "Y"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.Delete("X"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.Remove("X", "a/1"); err == nil {
		t.Fatal("expected error")
	}
	l := s.List()
	if len(l) != 1 || l[0].Name != "X" || len(l[0].Items) != 1 {
		t.Fatalf("memory changed: %+v", l)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("file changed")
	}
}

func TestTwoInstances(t *testing.T) {
	p := tmpPath(t)
	a := mustOpen(t, p)
	b := mustOpen(t, p)
	a.Add("X", item("bg/2/13"))
	b.Add("X", item("bg/2/14"))
	c, ok := mustOpen(t, p).Get("X")
	if !ok || len(c.Items) != 2 {
		t.Fatalf("got %+v", c)
	}
	// a must also see b's item once it mutates again.
	a.Add("X", item("bg/2/15"))
	if c, _ := mustOpen(t, p).Get("X"); len(c.Items) != 3 {
		t.Fatalf("got %+v", c)
	}
}

func TestNoTempFilesLeft(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Add("X", item("a/1"))
	s.Add("X", item("a/2"))
	s.Rename("X", "Y")
	s.Remove("Y", "a/1")
	s.Delete("Y")
	ents, _ := os.ReadDir(filepath.Dir(p))
	if len(ents) != 1 || ents[0].Name() != "collections.json" {
		t.Fatalf("dir: %v", ents)
	}
}

func TestConcurrentUse(t *testing.T) {
	s := mustOpen(t, tmpPath(t))
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 10; j++ {
				s.Add("X", item("a/"+string(rune('a'+i))+string(rune('a'+j))))
				s.List()
				s.Contains("a/aa")
			}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if c, _ := s.Get("X"); len(c.Items) != 80 {
		t.Fatalf("items = %d", len(c.Items))
	}
}
