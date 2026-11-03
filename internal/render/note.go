package render

import "strings"

// WithNote puts the user's note at the top of md as a block quote headed
// "Your note". The text is escaped so Markdown and [[ref|label]] in it show
// as typed and never become links; a blank note leaves md alone.
func WithNote(md, note string) string {
	note = strings.TrimSpace(sentinelStripper.Replace(note))
	if note == "" {
		return md
	}
	note = strings.ReplaceAll(strings.ReplaceAll(note, "\r\n", "\n"), "\r", "\n")
	var b strings.Builder
	b.WriteString("> **Your note**\n>\n")
	for _, l := range strings.Split(note, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			b.WriteString(">\n")
			continue
		}
		b.WriteString("> " + escapeMarkdown(l) + "\n")
	}
	b.WriteString("\n---\n\n")
	return b.String() + md
}

// escapeMarkdown backslash-escapes what Markdown would act on: emphasis,
// code, brackets, tags, entities, and a line's leading list or heading mark.
// (Glamour shows an escaped "/" or ":" with its backslash, so those stay.)
func escapeMarkdown(s string) string {
	var b strings.Builder
	digits := 0
	for i, r := range s {
		switch {
		case strings.ContainsRune("\\`*_[]<>~&|", r):
			b.WriteByte('\\')
		case i == 0 && strings.ContainsRune("#-+=", r):
			b.WriteByte('\\')
		case (r == '.' || r == ')') && digits > 0 && digits == i:
			b.WriteByte('\\')
		}
		if r >= '0' && r <= '9' && digits == i {
			digits++
		}
		b.WriteRune(r)
	}
	return b.String()
}
