package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteNewIsPrivateAndFresh(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "made")
	target := filepath.Join(t.TempDir(), "victim")
	os.WriteFile(target, []byte("keep"), 0o644)
	// A planted symlink under the old guessable name is never written through.
	os.MkdirAll(dir, 0o755)
	os.Symlink(target, filepath.Join(dir, "notes.json.unsaved-x-"))

	a, err := writeNew(dir, "notes.json.unsaved-x-*", "my note")
	if err != nil {
		t.Fatal(err)
	}
	b, err := writeNew(dir, "notes.json.unsaved-x-*", "my note")
	if err != nil || a == b {
		t.Fatalf("second: %q %q %v", a, b, err)
	}
	if got, _ := os.ReadFile(a); string(got) != "my note" {
		t.Fatalf("content %q", got)
	}
	if got, _ := os.ReadFile(target); string(got) != "keep" {
		t.Fatalf("wrote through symlink: %q", got)
	}
	if fi, _ := os.Lstat(a); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
}
