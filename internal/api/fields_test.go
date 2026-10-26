package api

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func load(t *testing.T, name string) Item {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var it Item
	if err := json.Unmarshal(b, &it); err != nil {
		t.Fatal(err)
	}
	return it
}

func TestDecodeVersePrevNext(t *testing.T) {
	it := load(t, "verse-bg-2-13-v1.json")
	if it.Prev != "bg/2/12" || it.Next != "bg/2/14" || len(it.References) != 0 {
		t.Errorf("prev=%q next=%q refs=%v", it.Prev, it.Next, it.References)
	}
}

func TestDecodeLecture(t *testing.T) {
	it := load(t, "verse-lecture-680317bgsf.json")
	want := []Reference{
		{Text: "[[bg/7/1|Bg. 7.1]]", Ref: "bg/7/1", Field: "purport"},
		{Text: "[[bg/9/4|*Bg.* 9.4]]", Ref: "bg/9/4", Field: "purport"},
		{Text: "[[iso/0a|*Īśo* Invocation]]", Ref: "iso/0a", Field: "purport"},
	}
	if !reflect.DeepEqual(it.References, want) {
		t.Errorf("refs = %+v", it.References)
	}
	d := it.Document
	if d == nil || d.IsoDate != "1968-03-17" || d.Year != 1968 || !strings.HasSuffix(d.AudioURL, "680317BG.SF.mp3") ||
		d.Recipients != nil || d.Address != "" {
		t.Errorf("document = %+v", d)
	}
}

func TestDecodeNewLetter(t *testing.T) {
	it := load(t, "verse-letter-lokanatha-2.json")
	d := it.Document
	if d == nil || !reflect.DeepEqual(d.Recipients, []string{"Lokanatha"}) || d.Address != "Vrindaban" ||
		d.IsoDate != "1976-08-23" || d.AudioURL != "" {
		t.Errorf("document = %+v", d)
	}
}

func TestDecodeNullRef(t *testing.T) {
	var it Item
	if err := json.Unmarshal([]byte(`{"prev":null,"next":null,"references":[{"text":"Cc","ref":null,"field":"purport"}]}`), &it); err != nil {
		t.Fatal(err)
	}
	if it.Prev != "" || it.Next != "" || len(it.References) != 1 || it.References[0].Ref != "" {
		t.Errorf("%+v", it)
	}
}

func TestCleanNewLetterUnchanged(t *testing.T) {
	it := load(t, "verse-letter-lokanatha-2.json")
	if got := Clean(it); got.Purport != it.Purport {
		t.Errorf("purport changed: %q", got.Purport[:80])
	}
}

func TestCleanLetterStampWithoutHeaderUnchanged(t *testing.T) {
	p := "My Dear Friend,\n\nI wrote to you on 68-10-13 about the plan.\n\nYour servant,\nACBS/hs"
	it := Item{Book: "letters", Purport: p}
	if got := Clean(it).Purport; got != p {
		t.Errorf("changed: %q", got)
	}
}

func TestCleanSanitizesNewFields(t *testing.T) {
	it := Item{Prev: "a\x1b[2J", Next: "b\x1b[2J",
		References: []Reference{{Text: "t\x1b[2J", Ref: "r\x1b[2J", Field: "f\x1b[2J"}},
		Document:   &Document{IsoDate: "1\x1b[2J", AudioURL: "u\x1b[2J", Address: "a\x1b[2J", Recipients: []string{"x\x1b[2J"}}}
	got := Clean(it)
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), `\u001b`) {
		t.Errorf("escape left: %s", b)
	}
	if len(it.References[0].Text) == len(got.References[0].Text) || it.Document.Recipients[0] != "x\x1b[2J" {
		t.Error("input mutated or not sanitized")
	}
}
