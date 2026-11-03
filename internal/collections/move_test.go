package collections

import (
	"os"
	"strings"
	"testing"
	"time"
)

func refs(c Collection) string {
	var out []string
	for _, it := range c.Items {
		out = append(out, it.Ref)
	}
	return strings.Join(out, ",")
}

func TestMove(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	for _, r := range []string{"a", "b", "c"} {
		s.Add("X", item(r))
	}
	if err := s.Move("x", "c", -1); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Get("X"); refs(c) != "a,c,b" {
		t.Fatalf("up: %s", refs(c))
	}
	if err := s.Move("X", "a", 1); err != nil {
		t.Fatal(err)
	}
	if c, _ := mustOpen(t, p).Get("X"); refs(c) != "c,a,b" {
		t.Fatalf("down, persisted: %s", refs(c))
	}
	if err := s.Move("X", "nope", 1); err == nil {
		t.Fatal("unknown ref must error")
	}
	if err := s.Move("Y", "a", 1); err == nil {
		t.Fatal("unknown collection must error")
	}
}

func TestMoveClampsWithoutWriting(t *testing.T) {
	p := tmpPath(t)
	s := mustOpen(t, p)
	s.Add("X", item("a"))
	s.Add("X", item("b"))
	before, _ := os.ReadFile(p)
	old := time.Unix(1600000000, 0)
	os.Chtimes(p, old, old)
	for _, m := range []struct {
		ref   string
		delta int
	}{{"a", -1}, {"b", 1}, {"a", 0}} {
		if err := s.Move("X", m.ref, m.delta); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := os.ReadFile(p)
	fi, _ := os.Stat(p)
	if string(before) != string(after) || !fi.ModTime().Equal(old) {
		t.Fatal("clamped move wrote the file")
	}
	if c, _ := s.Get("X"); refs(c) != "a,b" {
		t.Fatalf("order changed: %s", refs(c))
	}
}

func TestMoveMergesExternalChange(t *testing.T) {
	p := tmpPath(t)
	a := mustOpen(t, p)
	b := mustOpen(t, p)
	a.Add("X", item("1"))
	a.Add("X", item("2"))
	b.Add("X", item("3")) // b merges a's items, appends 3
	if err := a.Move("X", "3", -2); err != nil {
		t.Fatal(err)
	}
	if c, _ := mustOpen(t, p).Get("X"); refs(c) != "3,1,2" {
		t.Fatalf("got %s", refs(c))
	}
}
