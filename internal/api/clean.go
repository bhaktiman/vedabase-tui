package api

import (
	"regexp"
	"strings"
)

var (
	anchorLine   = regexp.MustCompile(`(?m)^[ \t]*\[#[A-Za-z0-9]+\][ \t]*\n?`)
	manyNewlines = regexp.MustCompile(`\n{3,}`)
	dateStamp    = regexp.MustCompile(`\d\d-\d\d-\d\d`)
	salutation   = regexp.MustCompile(`^(?i:(?:my )?dear) `)
	placeBefore  = regexp.MustCompile(`^([A-Z][A-Za-z .,'-]{0,40}?)((?i:(?:my )?dear) )`)
	greetComma   = regexp.MustCompile(`((?i:dear) [^,\n]{1,80},)(\S)`)
	lostBreak    = regexp.MustCompile(`,([A-Z])`)
	sentenceGlue = regexp.MustCompile(`([.!?]["'\x{201D}\x{2019})\]]?)([A-Z])`)
)

const headerWindow = 400

// Clean repairs known quirks in API text. The cache keeps the raw item, so
// callers apply Clean on every read and the rules can change later.
func Clean(it Item) Item {
	for _, f := range []*string{&it.Ref, &it.Lang, &it.Book, &it.URL, &it.Devanagari, &it.VerseText,
		&it.Synonyms, &it.Translation, &it.Purport, &it.Format} {
		*f = SanitizeText(*f)
	}
	it.Prev, it.Next = SanitizeText(it.Prev), SanitizeText(it.Next)
	if it.References != nil {
		refs := make([]Reference, len(it.References)) // do not mutate the caller's copy
		for i, r := range it.References {
			refs[i] = Reference{Text: SanitizeText(r.Text), Ref: SanitizeText(r.Ref), Field: SanitizeText(r.Field)}
		}
		it.References = refs
	}
	if it.Document != nil {
		d := *it.Document
		d.Type, d.Date, d.Location = SanitizeText(d.Type), SanitizeText(d.Date), SanitizeText(d.Location)
		d.IsoDate, d.AudioURL, d.Address = SanitizeText(d.IsoDate), SanitizeText(d.AudioURL), SanitizeText(d.Address)
		if d.Recipients != nil {
			d.Recipients = make([]string, len(it.Document.Recipients))
			for i, r := range it.Document.Recipients {
				d.Recipients[i] = SanitizeText(r)
			}
		}
		it.Document = &d
	}
	switch it.Book {
	case "transcripts":
		it.Purport = cleanTranscript(it.Purport)
	case "letters":
		it.Purport = cleanLetter(it.Purport)
	}
	return it
}

func cleanTranscript(s string) string {
	if !strings.Contains(s, "[#") {
		return s
	}
	s = anchorLine.ReplaceAllString(s, "")
	return manyNewlines.ReplaceAllString(s, "\n\n")
}

func cleanLetter(s string) string {
	window := s
	if len(window) > headerWindow {
		window = window[:headerWindow]
	}
	if !oldHeader(s, window) {
		return s
	}
	loc := dateStamp.FindStringIndex(window)
	if loc == nil {
		return s
	}
	body := s[loc[1]:]
	if strings.Contains(s, "\n") {
		// Header fields sit on their own lines; the text is already broken
		// into paragraphs, so only the header goes.
		return strings.TrimPrefix(body, "\n")
	}
	if !salutation.MatchString(body) {
		if m := placeBefore.FindStringSubmatch(body); m != nil && strings.TrimSpace(m[1]) != "" {
			body = strings.TrimSpace(m[1]) + "\n\n" + body[len(m[1]):]
		}
	}
	body = greetComma.ReplaceAllString(body, "$1\n\n$2")
	body = lostBreak.ReplaceAllString(body, ",\n$1")
	return sentenceGlue.ReplaceAllString(body, "$1\n\n$2")
}

// oldHeader reports whether a letter still has the pre-2026 header: the text
// starts with "Letter to:", or the dd-dd-dd stamp follows a "Letter to:" or
// "Dated:" label inside the header window. Newer letters begin at the
// salutation and may mention such a date in the body.
func oldHeader(s, window string) bool {
	if strings.HasPrefix(s, "Letter to:") {
		return true
	}
	loc := dateStamp.FindStringIndex(window)
	if loc == nil {
		return false
	}
	before := window[:loc[0]]
	return strings.Contains(before, "Letter to:") || strings.Contains(before, "Dated:")
}
