package ocr

import (
	"image"
	"strings"
	"unicode"

	"github.com/mphpmaster/word-bomb-tool-go/internal/winocr"
	"github.com/mphpmaster/word-bomb-tool-go/internal/wordlist"
)

// windowsLayouts are the two line layouts for the English engine: anchor word,
// text height and gap (in text heights). Each read every synthetic prompt in
// the tests; the second only runs when the first finds nothing.
var windowsLayouts = []struct {
	anchor string
	height int
	gap    float64
}{
	{"WORD", 24, 1.2},
	{"THE", 32, 0.6},
}

// WindowsOCRAvailable reports whether the English Windows OCR engine exists;
// without it the letters are read with Tesseract.
func WindowsOCRAvailable() bool { return winocr.Available("en") }

// LettersFromWindowsOCR reads the prompt letters with the English Windows OCR
// engine. ok is false when the engine is unavailable or failed, so the caller
// can fall back to Tesseract.
func LettersFromWindowsOCR(img image.Image) (letters string, ok bool) {
	letters, _, ok = englishReading(img)
	return letters, ok
}

// englishReading is LettersFromWindowsOCR plus how many of the three copies
// gave the reading.
func englishReading(img image.Image) (letters string, votes int, ok bool) {
	if !winocr.Available("en") {
		return "", 0, false
	}
	for _, l := range windowsLayouts {
		pre := PreprocessForWindowsOCR(img, l.anchor, l.height, 3, l.gap)
		if pre.Bounds().Dx() <= 0 {
			return "", 0, true
		}
		text, err := winocr.Recognize(pre, "en")
		if err != nil {
			return "", 0, false
		}
		if letters, votes = majorityToken(text, l.anchor); letters != "" {
			return letters, votes, true
		}
	}
	return "", 0, true
}

// MajorityToken returns the most frequent letters-only token in the OCR text,
// ignoring the anchor word (the line holds several copies of the prompt). Ties
// go to the token seen first.
func MajorityToken(text, anchor string) string {
	t, _ := majorityToken(text, anchor)
	return t
}

func majorityToken(text, anchor string) (string, int) {
	skip := strings.ToLower(anchor)
	counts := map[string]int{}
	var order []string
	for _, f := range strings.Fields(text) {
		t := KeepLetters(f)
		if t == "" || t == skip {
			continue
		}
		if counts[t] == 0 {
			order = append(order, t)
		}
		counts[t]++
	}
	best, bestN := "", 0
	for _, t := range order {
		if counts[t] > bestN {
			best, bestN = t, counts[t]
		}
	}
	return best, bestN
}

// ArabicReading reads the prompt (three copies in a row) with the Windows
// Arabic engine. It returns the most common Arabic reading, how many copies gave
// it, and whether any copy came back with Latin letters instead.
func ArabicReading(img image.Image) (letters string, votes int, anyLatin bool) {
	if !winocr.Available("ar") {
		return "", 0, false
	}
	pre := PreprocessForWindowsOCR(img, "", 32, 3, 0.8)
	if pre.Bounds().Dx() <= 0 {
		return "", 0, false
	}
	text, err := winocr.Recognize(pre, "ar")
	if err != nil || text == "" {
		return "", 0, false
	}
	counts := map[string]int{}
	var order []string
	for _, raw := range strings.Fields(text) {
		var b strings.Builder
		for _, r := range raw {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				anyLatin = true
			}
			if wordlist.IsArabicLetter(r) {
				b.WriteRune(r)
			}
		}
		t := b.String()
		if t == "" {
			continue
		}
		if counts[t] == 0 {
			order = append(order, t)
		}
		counts[t]++
	}
	for _, t := range order {
		if counts[t] > votes {
			letters, votes = t, counts[t]
		}
	}
	return letters, votes, anyLatin
}

// ReadPrompt reads the prompt and decides between English and Arabic. The
// English engine turns Arabic glyphs into plausible Latin letters ("يز" ->
// "cz"), and the Arabic engine sometimes turns Latin ones into Arabic ("QU" ->
// "لاه"), so: an Arabic reading that all three copies agree on, with no Latin
// text, is Arabic; otherwise an English reading of 2+ letters that some word
// contains wins; otherwise a two-copy Arabic majority; otherwise nothing.
// One guard on top of that order: when the English engine also reads the same
// word-forming letters on all three copies, the prompt is Latin. The Arabic
// engine reads "QU" as "لاه" on all three copies for some fonts and sizes
// (which ones depends on single grey levels of the scaled glyphs), while the
// English engine reads nothing at all for real Arabic prompts.
// Returns lowercase a-z for English, Arabic letters for Arabic, or "".
func ReadPrompt(img image.Image) string {
	ar, votes, anyLatin := ArabicReading(img)
	en, enVotes, _ := englishReading(img)
	plausibleEn := len(en) >= 2 && wordlist.Any(en, "Contains")
	if votes >= 3 && !anyLatin && !(plausibleEn && enVotes >= 3) {
		return ar
	}
	if plausibleEn {
		return en
	}
	if votes >= 2 {
		return ar
	}
	return ""
}

// IsLatinPrompt reports whether the letters are a Latin a-z prompt.
func IsLatinPrompt(letters string) bool {
	if letters == "" {
		return false
	}
	for _, r := range letters {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// IsArabicPrompt reports whether the letters are an Arabic prompt.
func IsArabicPrompt(letters string) bool {
	if letters == "" {
		return false
	}
	for _, r := range letters {
		if !wordlist.IsArabicLetter(r) {
			return false
		}
	}
	return true
}

// KeepLetters returns lowercase a-z only. Accents are stripped first ("ä" ->
// "a"), and anything that isn't a Latin letter is dropped.
func KeepLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if base, ok := accentFold[r]; ok {
			b.WriteByte(base)
			continue
		}
		r = unicode.ToLower(r)
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
