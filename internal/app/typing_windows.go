//go:build windows

package app

import (
	"math"
	"math/rand"
	"time"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
	"github.com/mphpmaster/word-bomb-tool-go/internal/input"
)

// typeWordHumanLike types a word with variable gaps between keys so the rhythm
// is not perfectly metronomic. baseDelay is the typical seconds between
// keystrokes; interKeyScale nudges speed without changing the saved setting.
// Port of _type_word_human_like from main.py.
func typeWordHumanLike(word string, baseDelay, interKeyScale float64) {
	if word == "" {
		return
	}
	if baseDelay <= 0 {
		input.TypeString(word)
		return
	}

	base := config.TypingDelayMin
	if v := baseDelay * interKeyScale; v > base {
		base = v
	}
	low := 0.03
	if v := base * 0.52; v > low {
		low = v
	}
	high := config.TypingDelayMax
	if v := base * 2.45; v < high {
		high = v
	}
	mode := base
	if mode < low {
		mode = low
	}
	if mode > high {
		mode = high
	}

	runes := []rune(word)
	for i, ch := range runes {
		input.TypeRune(ch)
		if i >= len(runes)-1 {
			break
		}
		// Hesitation / micro-pauses.
		r := rand.Float64()
		if r < 0.34 {
			sleepSeconds(uniform(0.1, 0.34))
		} else if r < 0.42 {
			sleepSeconds(uniform(0.16, 0.45))
		}
		sleepSeconds(triangular(low, high, mode))
	}
}

// uniform returns a random float in [a,b).
func uniform(a, b float64) float64 { return a + rand.Float64()*(b-a) }

// triangular samples the triangular distribution on [low,high] with the given
// mode, matching Python's random.triangular.
func triangular(low, high, mode float64) float64 {
	if high <= low {
		return low
	}
	u := rand.Float64()
	c := (mode - low) / (high - low)
	if u > c {
		u = 1 - u
		c = 1 - c
		low, high = high, low
	}
	return low + (high-low)*math.Sqrt(u*c)
}

func sleepSeconds(s float64) {
	if s <= 0 {
		return
	}
	time.Sleep(time.Duration(s * float64(time.Second)))
}
