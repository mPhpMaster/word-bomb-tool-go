package wordlist

import (
	"strings"
	"testing"
	"time"
)

func contains(list []string, w string) bool {
	for _, x := range list {
		if x == w {
			return true
		}
	}
	return false
}

func TestLoadsTheWholeList(t *testing.T) {
	if n := Count(); n <= 170_000 {
		t.Fatalf("only %d English words", n)
	}
}

func TestContainsSearchFindsWordsShortestFirstWithoutThePrompt(t *testing.T) {
	words := Search("HEA", "Contains")
	if !contains(words, "head") || !contains(words, "ahead") {
		t.Fatalf("missing head/ahead in %d words", len(words))
	}
	if contains(words, "hea") {
		t.Fatal("the prompt itself was suggested")
	}
	for i, w := range words {
		if !strings.Contains(w, "hea") {
			t.Fatalf("%q does not contain hea", w)
		}
		if i > 0 && len(words[i-1]) > len(w) {
			t.Fatalf("not shortest first: %q before %q", words[i-1], w)
		}
	}
}

func TestStartsAndEndsWith(t *testing.T) {
	for _, w := range Search("qu", "Starts With") {
		if !strings.HasPrefix(w, "qu") {
			t.Fatalf("%q does not start with qu", w)
		}
	}
	ing := Search("ing", "Ends With")
	if len(ing) == 0 {
		t.Fatal("no words end with ing")
	}
	for _, w := range ing {
		if !strings.HasSuffix(w, "ing") {
			t.Fatalf("%q does not end with ing", w)
		}
	}
}

func TestRhymesAreNotAnsweredLocally(t *testing.T) {
	if Supports("Rhymes") || Supports("Related Words") {
		t.Fatal("Rhymes/Related Words must use Datamuse")
	}
	if len(Search("cat", "Rhymes")) != 0 {
		t.Fatal("Rhymes answered locally")
	}
}

func TestFixesCapitalIReadAsL(t *testing.T) {
	cases := map[string]string{
		// A capital I read as l: "lzz" matches nothing, "lgh" only a few rare words.
		"lzz": "izz",
		"lgh": "igh",
		// Already-valid letters are left alone.
		"ill": "ill",
		"lli": "lli",
		"li":  "li",
	}
	for in, want := range cases {
		if got := FixILConfusion(in, "Contains"); got != want {
			t.Errorf("FixILConfusion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestArabicPromptsSearchTheArabicListMostCommonFirst(t *testing.T) {
	if n := ArabicCount(); n <= 50_000 {
		t.Fatalf("only %d Arabic words", n)
	}
	words := Search("يز", "Contains")
	if len(words) == 0 {
		t.Fatal("no Arabic words contain يز")
	}
	if contains(words, "يز") {
		t.Fatal("the prompt itself was suggested")
	}
	for _, w := range words {
		if !strings.Contains(w, "يز") || !IsArabic(w) {
			t.Fatalf("bad Arabic match %q", w)
		}
	}
	// Most common first: the list order is kept, not re-sorted by length.
	all := arabicWords()
	idx := map[string]int{}
	for i, w := range all {
		idx[w] = i
	}
	for i := 1; i < len(words); i++ {
		if idx[words[i-1]] > idx[words[i]] {
			t.Fatalf("Arabic results not in list (frequency) order: %q before %q", words[i-1], words[i])
		}
	}
	if len(Search("ال", "Starts With")) == 0 {
		t.Fatal("no Arabic words start with ال")
	}
	// No I/L swapping on Arabic letters.
	if got := FixILConfusion("يز", "Contains"); got != "يز" {
		t.Fatalf("FixILConfusion changed Arabic letters to %q", got)
	}
}

func TestAnyMatchesSearch(t *testing.T) {
	for _, p := range []string{"hea", "zzzq", "يز", "ing"} {
		if Any(p, "Contains") != (len(Search(p, "Contains")) > 0) {
			t.Fatalf("Any(%q) disagrees with Search", p)
		}
	}
}

func TestSearchIsFast(t *testing.T) {
	Count()
	start := time.Now()
	for i := 0; i < 20; i++ {
		Search("in", "Contains")
	}
	if per := time.Since(start) / 20; per > 200*time.Millisecond {
		t.Fatalf("%v per search", per)
	}
}
