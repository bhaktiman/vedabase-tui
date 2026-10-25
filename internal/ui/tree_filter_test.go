package ui

import "testing"

// A kept filter must apply to the nodes of a library swapped in afterwards.
func TestTreeFilterSurvivesSetLibrary(t *testing.T) {
	tr, _ := newTestTree(t)
	tr.SetFilter("abhi")
	want := len(tr.rows)
	if want == 0 {
		t.Fatal("filter matched nothing")
	}
	tr.SetLibrary(sampleLibrary(t)) // fresh Node objects
	if len(tr.rows) != want || !has(labels(tr), "Abhirama") {
		t.Errorf("rows %d, want %d: %v", len(tr.rows), want, labels(tr))
	}
}
