package ocr

import (
	"image"
	"testing"
)

func TestKeepLettersFoldsAccentsAndKeepsAToZ(t *testing.T) {
	cases := map[string]string{
		"HEA":      "hea",
		"ÄB":       "ab",
		"çàé":      "cae",
		"Ŭṅķ":      "unk",
		"a-b 1c!":  "abc",
		"يز":       "",
		"ß":        "",
		"QU\n":     "qu",
		"İstanbul": "istanbul",
	}
	for in, want := range cases {
		if got := KeepLetters(in); got != want {
			t.Errorf("KeepLetters(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMajorityTokenDropsAnchorAndVotes(t *testing.T) {
	cases := []struct{ text, anchor, want string }{
		{"WORD AB AB A8", "WORD", "ab"},
		{"word TR TR", "WORD", "tr"},
		{"WORD", "WORD", ""},
		// Accents are stripped and non-Latin letters dropped.
		{"WORD ÄB ÄB", "WORD", "ab"},
		// Ties go to the first token.
		{"THE XE QU", "THE", "xe"},
	}
	for _, c := range cases {
		if got := MajorityToken(c.text, c.anchor); got != c.want {
			t.Errorf("MajorityToken(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestPromptScripts(t *testing.T) {
	if !IsLatinPrompt("hea") || IsLatinPrompt("hé") || IsLatinPrompt("") || IsLatinPrompt("يز") {
		t.Fatal("IsLatinPrompt")
	}
	if !IsArabicPrompt("يز") || IsArabicPrompt("يzز") || IsArabicPrompt("") || IsArabicPrompt("ab") {
		t.Fatal("IsArabicPrompt")
	}
}

// fillRect paints a black rectangle on a white image.
func fillRect(img *image.Gray, r image.Rectangle) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Pix[y*img.Stride+x] = 0
		}
	}
}

func whiteGray(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	return img
}

func TestKeepMainTextDropsFrameCornerTextAndSpecks(t *testing.T) {
	img := whiteGray(200, 80)
	// Two big glyph-like blobs in the middle.
	fillRect(img, image.Rect(60, 20, 80, 60))
	fillRect(img, image.Rect(100, 20, 120, 60))
	// A dot above the second glyph (like the dot of an i or an Arabic letter).
	fillRect(img, image.Rect(106, 10, 112, 15))
	// Small corner text ("1K" counter) and a speck far from the glyphs.
	fillRect(img, image.Rect(180, 66, 184, 76))
	fillRect(img, image.Rect(186, 66, 190, 76))
	fillRect(img, image.Rect(10, 70, 12, 72))
	// A 1px frame around the whole region (the old overlay border).
	fillRect(img, image.Rect(0, 0, 200, 1))
	fillRect(img, image.Rect(0, 79, 200, 80))
	fillRect(img, image.Rect(0, 0, 1, 80))
	fillRect(img, image.Rect(199, 0, 200, 80))

	out := KeepMainText(img)
	at := func(x, y int) uint8 { return out.Pix[y*out.Stride+x] }
	for _, p := range [][2]int{{70, 40}, {110, 40}, {108, 12}} {
		if at(p[0], p[1]) != 0 {
			t.Errorf("glyph pixel %v was removed", p)
		}
	}
	for _, p := range [][2]int{{182, 70}, {188, 70}, {11, 71}, {100, 0}, {0, 40}, {199, 40}, {100, 79}} {
		if at(p[0], p[1]) != 255 {
			t.Errorf("noise pixel %v was kept", p)
		}
	}
	box, ok := inkBounds(out)
	if !ok || box != image.Rect(60, 10, 120, 60) {
		t.Errorf("ink bounds after cleanup = %v, want (60,10)-(120,60)", box)
	}
}

func TestPreprocessForWindowsOCRLaysOutAnchorAndCopies(t *testing.T) {
	src := whiteGray(120, 60)
	fillRect(src, image.Rect(30, 15, 50, 45))
	fillRect(src, image.Rect(60, 15, 80, 45))
	pre := PreprocessForWindowsOCR(src, "", 32, 3, 0.8)
	// Padding of one text height on every side around a 32px line.
	if pre.Bounds().Dy() != 32*3 {
		t.Fatalf("height = %d, want 96", pre.Bounds().Dy())
	}
	if pre.Bounds().Dx() <= 3*32 {
		t.Fatalf("width %d too small for three copies", pre.Bounds().Dx())
	}
	withAnchor := PreprocessForWindowsOCR(src, "WORD", 24, 3, 1.2)
	if withAnchor.Bounds().Dy() != 24*3 {
		t.Fatalf("anchored height = %d, want 72", withAnchor.Bounds().Dy())
	}
	if empty := PreprocessForWindowsOCR(whiteGray(50, 20), "", 32, 3, 0.8); empty.Bounds().Dx() != 0 {
		t.Fatal("blank region must give an empty image")
	}
}

func TestResizeKeepsFlatRegionsFlat(t *testing.T) {
	img := whiteGray(10, 10)
	for _, size := range [][2]int{{40, 40}, {5, 5}, {17, 3}} {
		out := resize(img, size[0], size[1])
		for i, v := range out.Pix {
			if v != 255 {
				t.Fatalf("resize to %v: pixel %d = %d, want 255 (edges must not darken)", size, i, v)
			}
		}
	}
}
