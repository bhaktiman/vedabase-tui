package ref

import (
	"errors"
	"fmt"
	"strings"
)

// ParseGoto turns what a user typed in the go-to prompt ("bg 2.13",
// "cc m 9.74", "bg/2/13") into a ref.
func ParseGoto(in string) (string, error) {
	in = strings.ToLower(strings.TrimSpace(in))
	if in == "" {
		return "", errors.New("empty reference")
	}
	if strings.Contains(in, "/") {
		segs := strings.Split(in, "/")
		if len(segs) < 2 || len(segs) > 5 {
			return "", fmt.Errorf("bad reference %q", in)
		}
		for _, s := range segs {
			if s == "" {
				return "", fmt.Errorf("bad reference %q", in)
			}
		}
		return in, nil
	}

	fields := strings.Fields(in)
	book := fields[0]
	if !isLetters(book) {
		return "", fmt.Errorf("%q is not a book code", book)
	}
	out := []string{book}
	rest := fields[1:]
	if book == "cc" {
		if len(rest) == 0 {
			return "", errors.New("cc needs a lila and numbers")
		}
		lila, err := matchLila(rest[0])
		if err != nil {
			return "", err
		}
		out = append(out, lila)
		rest = rest[1:]
	}
	if len(rest) != 1 {
		return "", errors.New("expected numbers like 2.13")
	}
	parts := strings.Split(rest[0], ".")
	for i, p := range parts {
		if !validNumber(p, i == len(parts)-1) {
			return "", fmt.Errorf("bad number %q", p)
		}
	}
	return strings.Join(append(out, parts...), "/"), nil
}

func isLetters(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// matchLila accepts a unique prefix of a lila name; "a" is taken as adi.
func matchLila(s string) (string, error) {
	if s == "a" {
		return "adi", nil
	}
	var hit string
	for _, l := range Lilas {
		if s != "" && strings.HasPrefix(l, s) {
			if hit != "" {
				return "", fmt.Errorf("ambiguous lila %q", s)
			}
			hit = l
		}
	}
	if hit == "" {
		return "", fmt.Errorf("unknown lila %q", s)
	}
	return hit, nil
}

// validNumber accepts digits with an optional letter suffix ("0a", "1b");
// the last part may also be a range such as "16-18".
func validNumber(p string, last bool) bool {
	if last {
		if a, b, ok := strings.Cut(p, "-"); ok {
			return validNumber(a, false) && validNumber(b, false)
		}
	}
	n, rest := splitNumber(p)
	return n != "" && (rest == "" || isLetters(rest))
}
