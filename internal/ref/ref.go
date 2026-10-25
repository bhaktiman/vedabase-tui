// Package ref handles vedabase references such as "bg/2/13": extracting them
// from URLs, ordering them naturally, and turning them into labels and slugs.
package ref

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Lilas are the three divisions of Caitanya-caritamrta, in reading order.
var Lilas = []string{"adi", "madhya", "antya"}

// FromPath turns a vedabase URL or path into a ref. The site's "/en/library/"
// prefix is dropped; the root is not a ref.
func FromPath(p string) (string, bool) {
	if u, err := url.Parse(p); err == nil {
		p = u.Path
	}
	p = strings.Trim(p, "/")
	p = strings.TrimPrefix(p, "en/")
	p = strings.TrimPrefix(p, "library/")
	if p == "" {
		return "", false
	}
	return p, true
}

// Less reports whether ref a sorts before ref b, comparing segment by
// segment; a ref sorts before any ref it is a prefix of.
func Less(a, b string) bool {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := CompareSegment(as[i], bs[i]); c != 0 {
			return c < 0
		}
	}
	return len(as) < len(bs)
}

func lilaIndex(s string) int {
	for i, l := range Lilas {
		if l == s {
			return i
		}
	}
	return -1
}

// splitNumber splits a segment into its leading digits and the remaining suffix.
func splitNumber(s string) (num, rest string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i], s[i:]
}

// CompareSegment orders segments: lila names first (in Lilas order), then
// segments with a leading number (by number, then suffix), then other words.
func CompareSegment(a, b string) int {
	rank := func(s string) int {
		if lilaIndex(s) >= 0 {
			return 0
		}
		if n, _ := splitNumber(s); n != "" {
			return 1
		}
		return 2
	}
	ra, rb := rank(a), rank(b)
	if ra != rb {
		return ra - rb
	}
	switch ra {
	case 0:
		return lilaIndex(a) - lilaIndex(b)
	case 1:
		na, sa := splitNumber(a)
		nb, sb := splitNumber(b)
		// Compare as digit strings so huge numbers cannot overflow.
		na, nb = strings.TrimLeft(na, "0"), strings.TrimLeft(nb, "0")
		if len(na) != len(nb) {
			return len(na) - len(nb)
		}
		if c := strings.Compare(na, nb); c != 0 {
			return c
		}
		return strings.Compare(sa, sb)
	}
	return strings.Compare(a, b)
}

// Display renders a ref as a short label: "bg/2/13" -> "Bg 2.13".
func Display(r string) string {
	parts := strings.Split(r, "/")
	book := parts[0]
	if book == "sb" || book == "cc" {
		book = strings.ToUpper(book)
	} else {
		book = capitalize(book)
	}
	rest := parts[1:]
	if len(rest) > 0 && lilaIndex(rest[0]) >= 0 {
		book += " " + capitalize(rest[0])
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return book
	}
	return book + " " + strings.Join(rest, ".")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Recipient extracts the addressee from a letter slug such as
// "letter-to-abhirama-12" (the trailing disambiguating number is dropped).
// It returns "" for slugs that are not letters.
func Recipient(slug string) string {
	name, ok := strings.CutPrefix(slug, "letter-to-")
	if !ok {
		return ""
	}
	if i := strings.LastIndexByte(name, '-'); i >= 0 {
		if n, rest := splitNumber(name[i+1:]); n != "" && rest == "" {
			name = name[:i]
		}
	}
	return name
}

// TranscriptDate reads the YYMMDD prefix of a transcript slug. All transcripts
// are from the twentieth century, so the year is always 19YY.
func TranscriptDate(slug string) (time.Time, bool) {
	if len(slug) < 6 {
		return time.Time{}, false
	}
	for i := 0; i < 6; i++ {
		if slug[i] < '0' || slug[i] > '9' {
			return time.Time{}, false
		}
	}
	yy, _ := strconv.Atoi(slug[0:2])
	mm, _ := strconv.Atoi(slug[2:4])
	dd, _ := strconv.Atoi(slug[4:6])
	d := time.Date(1900+yy, time.Month(mm), dd, 0, 0, 0, 0, time.UTC)
	// time.Date normalises out-of-range values; reject those.
	if d.Month() != time.Month(mm) || d.Day() != dd {
		return time.Time{}, false
	}
	return d, true
}

// Filename makes a ref safe to use as a single file name.
func Filename(r string) string { return strings.ReplaceAll(r, "/", "-") }

// Slug lowercases name to ASCII, folding diacritics, with runs of other
// characters collapsed to a single "-".
func Slug(name string) string {
	var b strings.Builder
	pendingDash := false
	for _, r := range norm.NFD.String(name) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			if pendingDash && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingDash = false
			b.WriteRune(unicode.ToLower(r))
		} else {
			pendingDash = true
		}
	}
	return b.String()
}
