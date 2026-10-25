package ref

import "testing"

func TestFromPath(t *testing.T) {
	cases := map[string]string{
		"https://vedabase.cc/bg/2/13/": "bg/2/13",
		"/cc/adi/0a/":                  "cc/adi/0a",
		"https://vedabase.cc/en/library/letters/letter-to-aatie/": "letters/letter-to-aatie",
		"/en/library/transcripts/660219bg-new-york/":              "transcripts/660219bg-new-york",
	}
	for in, want := range cases {
		got, ok := FromPath(in)
		if !ok || got != want {
			t.Errorf("FromPath(%q)=%q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := FromPath("https://vedabase.cc/"); ok {
		t.Error("root must not be a ref")
	}
}

func TestLess(t *testing.T) {
	sorted := []string{"bg/0a", "bg/0b", "bg/1", "bg/1/1", "bg/1/16-18", "bg/1/19", "bg/2", "bg/10",
		"cc/adi", "cc/madhya", "cc/antya", "sb/4/29/1a-2a", "sb/4/29/1b", "sb/4/29/2b", "sb/10/1/notes", "sb/10/1/summary"}
	for i := 0; i+1 < len(sorted); i++ {
		if !Less(sorted[i], sorted[i+1]) || Less(sorted[i+1], sorted[i]) {
			t.Errorf("want %q < %q", sorted[i], sorted[i+1])
		}
	}
}

func TestDisplay(t *testing.T) {
	cases := map[string]string{"bg/2/13": "Bg 2.13", "sb/1/2/6": "SB 1.2.6", "cc/madhya/9/74": "CC Madhya 9.74",
		"bg/1/16-18": "Bg 1.16-18", "iso/3": "Iso 3", "lcfl/1": "Lcfl 1"}
	for in, want := range cases {
		if got := Display(in); got != want {
			t.Errorf("Display(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRecipientAndDate(t *testing.T) {
	if got := Recipient("letter-to-abhirama-12"); got != "abhirama" {
		t.Error(got)
	}
	if got := Recipient("letter-to-damodara-abhirama"); got != "damodara-abhirama" {
		t.Error(got)
	}
	d, ok := TranscriptDate("660219bg-new-york")
	if !ok || d.Format("2006-01-02") != "1966-02-19" {
		t.Error(d, ok)
	}
	if _, ok := TranscriptDate("bg-new-york"); ok {
		t.Error("no date")
	}
}

func TestFilenameSlug(t *testing.T) {
	if Filename("bg/2/13") != "bg-2-13" {
		t.Fail()
	}
	if Slug("On the Soul — Kṛṣṇa's") != "on-the-soul-krsna-s" {
		t.Error(Slug("On the Soul — Kṛṣṇa's"))
	}
}
