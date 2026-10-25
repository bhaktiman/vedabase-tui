package ref

import "testing"

func TestParseGoto(t *testing.T) {
	ok := map[string]string{"bg 2.13": "bg/2/13", "bg/2/13": "bg/2/13", "BG 2.13": "bg/2/13",
		"sb 1.2.6": "sb/1/2/6", "cc madhya 9.74": "cc/madhya/9/74", "cc m 9.74": "cc/madhya/9/74",
		"cc a 1.1": "cc/adi/1/1", "cc an 2.3": "cc/antya/2/3", "iso 3": "iso/3", " bg 1.16-18 ": "bg/1/16-18"}
	for in, want := range ok {
		got, err := ParseGoto(in)
		if err != nil || got != want {
			t.Errorf("ParseGoto(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "2.13", "bg 2..13"} {
		if _, err := ParseGoto(bad); err == nil {
			t.Errorf("ParseGoto(%q) want error", bad)
		}
	}
}
