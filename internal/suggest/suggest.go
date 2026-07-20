// Package suggest provides suggestion list operations (sorting and picking the
// next untyped word). It is the Go port of suggestion_manager.py.
package suggest

import (
	"math/rand"
	"sort"
	"strings"
	"unicode"
)

// Sort returns a new slice of suggestions ordered according to sortMode. The
// input slice is never mutated. Sorting is stable, matching Python's sorted().
func Sort(suggestions []string, sortMode string) []string {
	if len(suggestions) == 0 {
		return nil
	}
	out := make([]string, len(suggestions))
	copy(out, suggestions)

	switch sortMode {
	case "Shortest":
		sort.SliceStable(out, func(i, j int) bool { return len(out[i]) < len(out[j]) })
	case "Longest":
		sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	case "Frequency":
		// Mirrors the original: sort by descending count of uppercase letters.
		// For typical lowercase results this leaves order effectively unchanged.
		sort.SliceStable(out, func(i, j int) bool {
			return upperCount(out[i]) > upperCount(out[j])
		})
	case "Random":
		rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	}
	return out
}

func upperCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsUpper(r) {
			n++
		}
	}
	return n
}

// NextUntyped finds the next untyped word starting at startIndex, wrapping
// around the list. Words containing a space are skipped. It returns the chosen
// word and the index to resume from next time. When every candidate has already
// been typed it returns ("", startIndex).
func NextUntyped(suggestions []string, startIndex int, typed map[string]struct{}) (string, int) {
	n := len(suggestions)
	if n == 0 {
		return "", startIndex
	}
	// Guard against an out-of-range start index.
	start := ((startIndex % n) + n) % n
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		word := suggestions[idx]
		if strings.Contains(word, " ") {
			continue
		}
		if _, ok := typed[word]; !ok {
			return word, idx + 1
		}
	}
	return "", startIndex
}
