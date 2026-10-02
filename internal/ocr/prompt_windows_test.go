//go:build windows

package ocr

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
	"github.com/mphpmaster/word-bomb-tool-go/internal/winocr"
)

// Bold system fonts used to draw synthetic prompts (file name in C:\Windows\Fonts).
var promptFonts = map[string]string{
	"Arial":        "arialbd.ttf",
	"Segoe UI":     "segoeuib.ttf",
	"Verdana":      "verdanab.ttf",
	"Tahoma":       "tahomabd.ttf",
	"Trebuchet MS": "trebucbd.ttf",
}

var fontCache = map[string]*opentype.Font{}

func loadFont(t *testing.T, name string) *opentype.Font {
	t.Helper()
	if f, ok := fontCache[name]; ok {
		return f
	}
	path := filepath.Join(os.Getenv("WINDIR"), "Fonts", promptFonts[name])
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("font %s not installed: %v", path, err)
	}
	f, err := opentype.Parse(data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	fontCache[name] = f
	return f
}

// drawPrompt draws a prompt the way the game shows it: a short letter cluster
// in a bold font on a flat background, cropped like a user-picked region.
func drawPrompt(t *testing.T, text, fontName string, fg, bg color.Color, size int) *image.RGBA {
	t.Helper()
	face, err := opentype.NewFace(loadFont(t, fontName), &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	img := image.NewRGBA(image.Rect(0, 0, size*len([]rune(text))+40, size*2))
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	d := &font.Drawer{Dst: img, Src: image.NewUniform(fg), Face: face}
	m := face.Metrics()
	adv := d.MeasureString(text)
	x := (fixed.I(img.Bounds().Dx()) - adv) / 2
	y := (fixed.I(img.Bounds().Dy())-(m.Ascent+m.Descent))/2 + m.Ascent
	d.Dot = fixed.Point26_6{X: x, Y: y}
	d.DrawString(text)
	return img
}

func requireEngine(t *testing.T, lang string) {
	t.Helper()
	if !winocr.Available(lang) {
		t.Skipf("no Windows OCR language %q installed (%v)", lang, winocr.InitError())
	}
}

func loadSample(t *testing.T, name string) image.Image {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestReadsShortPromptsInCommonFontsAndNeverMistakesThemForArabic(t *testing.T) {
	requireEngine(t, "en")
	texts := []string{"HEA", "ING", "TR", "AB", "QU", "OMP", "ZY", "XE", "RIC", "WO", "ST", "ECT", "NN", "GH", "PH", "UNK", "CZ"}
	styles := []struct{ fg, bg color.RGBA }{
		{color.RGBA{255, 255, 255, 255}, color.RGBA{40, 44, 52, 255}},
		{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}},
		{color.RGBA{255, 255, 255, 255}, color.RGBA{120, 70, 200, 255}},
		{color.RGBA{180, 230, 60, 255}, color.RGBA{28, 28, 28, 255}},
	}
	ok, n := 0, 0
	var misses, arabic []string
	var total time.Duration
	for _, fontName := range []string{"Arial", "Segoe UI", "Verdana", "Tahoma", "Trebuchet MS"} {
		for _, text := range texts {
			for _, st := range styles {
				for _, size := range []int{18, 32, 56} {
					img := drawPrompt(t, text, fontName, st.fg, st.bg, size)
					start := time.Now()
					got := ReadPrompt(img)
					total += time.Since(start)
					n++
					tag := fmt.Sprintf("%s/%d:%s->%s", fontName, size, text, got)
					if got == strings.ToLower(text) {
						ok++
					} else {
						misses = append(misses, tag)
					}
					if got != "" && !IsLatinPrompt(got) {
						arabic = append(arabic, tag)
					}
				}
			}
		}
	}
	t.Logf("%d/%d read correctly (%.1f%%), %.1fms per read", ok, n, 100*float64(ok)/float64(n),
		float64(total.Microseconds())/1000/float64(n))
	if float64(ok) < float64(n)*0.97 {
		t.Errorf("%d/%d read correctly; misses: %v", ok, n, first(misses, 20))
	} else if len(misses) > 0 {
		t.Logf("misses: %v", first(misses, 20))
	}
	if len(arabic) > 0 {
		t.Errorf("Latin prompts read as Arabic: %v", first(arabic, 20))
	}
}

func TestRealArabicPromptIsDetectedAndNotReadAsLatin(t *testing.T) {
	requireEngine(t, "en")
	requireEngine(t, "ar")
	// Captured from the game (with the old on-region overlay border and the "1K"
	// counter in the corner). The English engine used to read it as "cz".
	img := loadSample(t, "arabic-prompt-yz.png")
	if got := ReadPrompt(img); got != "يز" {
		t.Fatalf("ReadPrompt = %q, want يز", got)
	}
	if IsLatinPrompt("يز") || !IsArabicPrompt("يز") {
		t.Fatal("يز must be an Arabic prompt")
	}
}

func TestIgnoresRegionFrameAndSmallCornerText(t *testing.T) {
	requireEngine(t, "en")
	frame := color.RGBA{97, 175, 239, 255}
	for _, text := range []string{"HEA", "TR", "ING", "ST"} {
		img := drawPrompt(t, text, "Arial", color.RGBA{180, 230, 60, 255}, color.RGBA{28, 28, 28, 255}, 32)
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		// A 2px frame like the old overlay border, plus a small "1K" counter.
		for i := 1; i <= 2; i++ {
			for x := i; x < w-i; x++ {
				img.Set(x, i, frame)
				img.Set(x, h-1-i, frame)
			}
			for y := i; y < h-i; y++ {
				img.Set(i, y, frame)
				img.Set(w-1-i, y, frame)
			}
		}
		face, err := opentype.NewFace(loadFont(t, "Arial"), &opentype.FaceOptions{Size: 9, DPI: 72})
		if err != nil {
			t.Fatal(err)
		}
		d := &font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{150, 200, 60, 255}), Face: face, Dot: fixed.P(w-18, h-5)}
		d.DrawString("1K")
		face.Close()
		if got := ReadPrompt(img); got != strings.ToLower(text) {
			t.Errorf("ReadPrompt(%s with frame) = %q", text, got)
		}
	}
}

// arabicVisual holds Arabic prompts as pre-shaped presentation forms in visual
// (left-to-right) order, since the Go rasterizer does no Arabic shaping.
var arabicVisual = map[string]string{
	"ست":  "ﺖﺳ",
	"قل":  "ﻞﻗ",
	"عين": "ﻦﻴﻋ",
	"ما":  "ﺎﻣ",
	"لا":  "ﻻ",
	"من":  "ﻦﻣ",
}

func TestArabicPromptsAreRecognisedAsArabic(t *testing.T) {
	requireEngine(t, "en")
	requireEngine(t, "ar")
	// Whatever the engines read, an Arabic prompt must never come back as Latin
	// letters to type.
	var latin []string
	arabic, exact, n := 0, 0, 0
	for _, fontName := range []string{"Arial", "Segoe UI"} {
		for text, visual := range arabicVisual {
			for _, size := range []int{32, 56} {
				img := drawPrompt(t, visual, fontName, color.RGBA{180, 230, 60, 255}, color.RGBA{28, 28, 28, 255}, size)
				got := ReadPrompt(img)
				n++
				switch {
				case IsLatinPrompt(got):
					latin = append(latin, fmt.Sprintf("%s/%d:%s->%s", fontName, size, text, got))
				case got != "":
					arabic++
					if got == text {
						exact++
					}
				}
			}
		}
	}
	t.Logf("%d/%d recognised as Arabic (%d exact)", arabic, n, exact)
	if len(latin) > 0 {
		t.Errorf("Arabic prompts read as Latin: %v", latin)
	}
	if float64(arabic) < float64(n)*0.8 {
		t.Errorf("only %d/%d recognised as Arabic", arabic, n)
	}
}

func TestReadsRealYourTurnBox(t *testing.T) {
	requireEngine(t, "en")
	// Captured from the game: green "YOUR TURN" on a dark purple box.
	img := loadSample(t, "your-turn.png")
	text, err := winocr.Recognize(preprocessTurnGate(img), "en")
	if err != nil {
		t.Fatal(err)
	}
	if !config.TurnGateAccepts(keepAlnum(text)) {
		t.Fatalf("turn gate read %q", text)
	}
}

func first(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
