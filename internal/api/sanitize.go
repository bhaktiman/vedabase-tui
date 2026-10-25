package api

import (
	"regexp"
	"strconv"
	"strings"
)

// numericRef matches an HTML numeric character reference; HTML allows the
// closing semicolon to be missing.
var numericRef = regexp.MustCompile(`&#(?:[xX]([0-9a-fA-F]+)|([0-9]+));?`)

// SanitizeText drops terminal control characters from remote text: C0 controls
// other than newline and tab, DEL, and the C1 range (which includes the 8-bit
// CSI). It also drops numeric character references that would decode to one of
// them (&#27; and friends), because the Markdown renderer decodes those later.
// Everything else, including combining marks and emoji, is kept.
func SanitizeText(s string) string {
	s = stripControls(s)
	// Removing a reference can join the text around it into a new one
	// ("&#&#27;27;"), so repeat until nothing changes.
	for strings.Contains(s, "&#") {
		next := numericRef.ReplaceAllStringFunc(s, func(m string) string {
			if refIsControl(numericRef.FindStringSubmatch(m)) {
				return ""
			}
			return m
		})
		if next == s {
			break
		}
		s = next
	}
	return s
}

func refIsControl(sub []string) bool {
	digits, base := sub[2], 10
	if sub[1] != "" {
		digits, base = sub[1], 16
	}
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return true // &#0; is a NUL
	}
	if len(digits) > 8 {
		return false // far beyond Unicode; decoders substitute U+FFFD
	}
	n, err := strconv.ParseInt(digits, base, 64)
	return err == nil && isControl(rune(n))
}

func stripControls(s string) string {
	clean := true
	for _, r := range s {
		if isControl(r) {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if !isControl(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isControl(r rune) bool {
	switch {
	case r == '\n' || r == '\t':
		return false
	case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
		return true
	}
	return false
}
