// Package wordlist is the offline word search, embedded in the binary: English
// from the ENABLE word list (public domain, ~173k words) and Arabic from a
// frequency-ordered list (see data/ARABIC-WORDS-NOTICE.md). Looking words up
// here takes a few milliseconds, where a Datamuse request took 0.5-2.5s per
// prompt.
package wordlist

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"sort"
	"strings"
	"sync"
)

var (
	//go:embed data/enable1.txt.gz
	englishGz []byte
	//go:embed data/arabic-words.txt.gz
	arabicGz []byte

	englishOnce, arabicOnce sync.Once
	english, arabic         []string
)

func englishWords() []string {
	englishOnce.Do(func() { english = load(englishGz) })
	return english
}

func arabicWords() []string {
	arabicOnce.Do(func() { arabic = load(arabicGz) })
	return arabic
}

func load(gz []byte) []string {
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil
	}
	defer r.Close()
	words := make([]string, 0, 200_000)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if line := strings.TrimRight(sc.Text(), "\r"); line != "" {
			words = append(words, line)
		}
	}
	return words
}

// Count is the number of English words loaded (forces the load).
func Count() int { return len(englishWords()) }

// ArabicCount is the number of Arabic words loaded (forces the load).
func ArabicCount() int { return len(arabicWords()) }

// Preload loads both lists in the background so the first prompt doesn't pay
// for it.
func Preload() {
	go func() {
		englishWords()
		arabicWords()
	}()
}

// IsArabicLetter reports whether r is an Arabic letter (hamza through yeh).
func IsArabicLetter(r rune) bool { return r >= 'ء' && r <= 'ي' }

// IsArabic reports whether the text contains Arabic letters.
func IsArabic(s string) bool {
	for _, r := range s {
		if IsArabicLetter(r) {
			return true
		}
	}
	return false
}

// Supports reports whether the search mode is answered from the local lists.
// Rhymes and Related Words need Datamuse.
func Supports(mode string) bool {
	return mode == "Starts With" || mode == "Ends With" || mode == "Contains"
}

func prepare(letters, mode string) (p string, ar bool, ok bool) {
	ar = IsArabic(letters)
	p = strings.TrimSpace(letters)
	if !ar {
		p = strings.ToLower(p)
	}
	return p, ar, p != "" && Supports(mode)
}

func matches(w, p, mode string) bool {
	// The prompt itself is never a suggestion.
	if len(w) <= len(p) {
		return false
	}
	switch mode {
	case "Starts With":
		return strings.HasPrefix(w, p)
	case "Ends With":
		return strings.HasSuffix(w, p)
	default:
		return strings.Contains(w, p)
	}
}

// Search returns all words matching the letters for the mode, never including
// the letters themselves. Arabic letters search the Arabic list (most common
// words first); anything else searches the English list (shortest first, then
// alphabetical). Unsupported modes return nil.
func Search(letters, mode string) []string {
	p, ar, ok := prepare(letters, mode)
	if !ok {
		return nil
	}
	list := englishWords()
	if ar {
		list = arabicWords()
	}
	var out []string
	for _, w := range list {
		if matches(w, p, mode) {
			out = append(out, w)
		}
	}
	// The Arabic list is already ordered by how common each word is. The English
	// list is alphabetical, so a stable sort by length keeps ties alphabetical.
	if !ar {
		sort.SliceStable(out, func(i, j int) bool { return len(out[i]) < len(out[j]) })
	}
	return out
}

// countMatches counts the words Search would return, without allocating them.
func countMatches(letters, mode string) int {
	p, ar, ok := prepare(letters, mode)
	if !ok {
		return 0
	}
	list := englishWords()
	if ar {
		list = arabicWords()
	}
	n := 0
	for _, w := range list {
		if matches(w, p, mode) {
			n++
		}
	}
	return n
}

// Any reports whether Search would return at least one word.
func Any(letters, mode string) bool {
	p, ar, ok := prepare(letters, mode)
	if !ok {
		return false
	}
	list := englishWords()
	if ar {
		list = arabicWords()
	}
	for _, w := range list {
		if matches(w, p, mode) {
			return true
		}
	}
	return false
}

// FixILConfusion handles OCR confusing a capital I with a lowercase l. Game
// prompts are common letter clusters, so when an I/L swap of the letters
// matches at least ten times as many words as the letters as read, the swap is
// returned; otherwise the letters are returned unchanged (lowercased).
func FixILConfusion(letters, mode string) string {
	p := strings.ToLower(letters)
	if !Supports(mode) || IsArabic(p) {
		return p
	}
	var positions []int
	for i := 0; i < len(p); i++ {
		if p[i] == 'i' || p[i] == 'l' {
			positions = append(positions, i)
		}
	}
	if len(positions) == 0 || len(positions) > 4 {
		return p
	}

	original := countMatches(p, mode)
	best, bestCount := p, original
	for mask := 1; mask < 1<<len(positions); mask++ {
		b := []byte(p)
		for k, pos := range positions {
			if mask&(1<<k) != 0 {
				if b[pos] == 'i' {
					b[pos] = 'l'
				} else {
					b[pos] = 'i'
				}
			}
		}
		variant := string(b)
		if n := countMatches(variant, mode); n > bestCount {
			best, bestCount = variant, n
		}
	}
	if bestCount >= max(1, original)*10 {
		return best
	}
	return p
}
