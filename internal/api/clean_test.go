package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCleanLetterAatie(t *testing.T) {
	b, err := os.ReadFile("testdata/verse-letter-aatie.json")
	if err != nil {
		t.Fatal(err)
	}
	var it Item
	if err := json.Unmarshal(b, &it); err != nil {
		t.Fatal(err)
	}
	got := Clean(it)
	if !strings.HasPrefix(got.Purport, "My Dear Aatie,\n\nPlease accept my blessings.") {
		t.Errorf("purport starts %q", got.Purport[:80])
	}
	if strings.Contains(got.Purport, "Dated:") {
		t.Error("header not removed")
	}
	if got.Translation != it.Translation || got.Ref != it.Ref {
		t.Error("other fields changed")
	}
}

func TestCleanLetterPlace(t *testing.T) {
	it := Item{Book: "letters", Purport: "Letter to: Abhirama70-07-14DetroitMy Dear Abhirama,Please accept my blessings.I am well."}
	got := Clean(it).Purport
	if !strings.HasPrefix(got, "Detroit\n\nMy Dear Abhirama,\n\nPlease") {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, "blessings.\n\nI am well.") {
		t.Errorf("sentence break missing: %q", got)
	}
}

func TestCleanLetterNoStampUnchanged(t *testing.T) {
	it := Item{Book: "letters", Purport: "Dear Friend,Hello.There"}
	if got := Clean(it); got != it {
		t.Errorf("changed: %q", got.Purport)
	}
}

func TestCleanTranscriptAnchors(t *testing.T) {
	it := Item{Book: "transcripts", Purport: "First line.\n\n[#bb573020]\nSecond line.\n[#a1]\n\n\nThird."}
	got := Clean(it).Purport
	if strings.Contains(got, "[#") {
		t.Errorf("anchor left: %q", got)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("3+ newlines left: %q", got)
	}
	if !strings.Contains(got, "Second line.") || !strings.Contains(got, "Third.") {
		t.Errorf("text lost: %q", got)
	}
}

func TestCleanLeavesOthersUnchanged(t *testing.T) {
	book := Item{Book: "bg", Purport: "Text.Another [#x] sentence.Here"}
	if got := Clean(book); got != book {
		t.Error("book changed")
	}
	letter := Item{Book: "letters", Purport: "Letter\n70-07-14DetroitMy Dear X,Hi.There"}
	if got := Clean(letter); got != letter {
		t.Error("letter with newlines changed")
	}
}
