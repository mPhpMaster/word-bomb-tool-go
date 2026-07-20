package suggest

import "testing"

func TestSortShortestLongest(t *testing.T) {
	in := []string{"bbb", "a", "cc"}
	if got := Sort(in, "Shortest"); got[0] != "a" || got[2] != "bbb" {
		t.Errorf("Shortest: %v", got)
	}
	if got := Sort(in, "Longest"); got[0] != "bbb" || got[2] != "a" {
		t.Errorf("Longest: %v", got)
	}
	// Input must not be mutated.
	if in[0] != "bbb" {
		t.Errorf("input mutated: %v", in)
	}
}

func TestNextUntypedWrapAndSkip(t *testing.T) {
	sugg := []string{"cat", "cat nap", "dog", "cow"}
	typed := map[string]struct{}{"cat": {}}

	// From index 0: "cat" is typed, "cat nap" has a space -> "dog".
	word, next := NextUntyped(sugg, 0, typed)
	if word != "dog" || next != 3 {
		t.Fatalf("got (%q,%d), want (dog,3)", word, next)
	}

	// All typed -> empty.
	all := map[string]struct{}{"cat": {}, "dog": {}, "cow": {}}
	if w, n := NextUntyped(sugg, 0, all); w != "" || n != 0 {
		t.Fatalf("got (%q,%d), want (\"\",0)", w, n)
	}
}
