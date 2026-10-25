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
	if it.Document != nil {
		d := *it.Document // do not mutate the caller's copy
		d.Type, d.Date, d.Location = SanitizeText(d.Type), SanitizeText(d.Date), SanitizeText(d.Location)
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
