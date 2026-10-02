//go:build windows

package winocr

import (
	"image"
	"image/draw"
	"strings"
	"sync"
	"testing"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// textImage renders black text on white, scaled up so the engine can read it.
func textImage(s string) *image.Gray {
	small := image.NewGray(image.Rect(0, 0, 10+7*len(s)+10, 30))
	draw.Draw(small, small.Bounds(), image.White, image.Point{}, draw.Src)
	d := &font.Drawer{Dst: small, Src: image.Black, Face: basicfont.Face7x13, Dot: fixed.P(10, 20)}
	d.DrawString(s)
	big := image.NewGray(image.Rect(0, 0, small.Bounds().Dx()*4, small.Bounds().Dy()*4))
	xdraw.NearestNeighbor.Scale(big, big.Bounds(), small, small.Bounds(), xdraw.Src, nil)
	return big
}

func TestRecognizesEnglishText(t *testing.T) {
	if !Available("en") {
		t.Skipf("no English OCR language installed (%v)", InitError())
	}
	t.Logf("English engine: %s, Arabic engine: %q", LanguageTag("en"), LanguageTag("ar"))
	text, err := Recognize(textImage("HELLO 42"), "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "HELLO") || !strings.Contains(text, "42") {
		t.Fatalf("read %q", text)
	}
}

func TestConcurrentCallsAreSerialized(t *testing.T) {
	if !Available("en") {
		t.Skip("no English OCR language installed")
	}
	img := textImage("HELLO 42")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Recognize(img, "en"); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestRejectsEmptyAndOversizedImages(t *testing.T) {
	if _, err := Recognize(image.NewGray(image.Rectangle{}), "en"); err == nil {
		t.Fatal("empty image accepted")
	}
	if !Available("en") {
		t.Skip("no English OCR language installed")
	}
	if _, err := Recognize(image.NewGray(image.Rect(0, 0, 20000, 10)), "en"); err == nil {
		t.Fatal("oversized image accepted")
	}
}

func TestUnknownLanguageIsUnavailable(t *testing.T) {
	if Available("xx") {
		t.Fatal(`"xx" reported available`)
	}
	if _, err := Recognize(textImage("HI"), "xx"); err == nil {
		t.Fatal("recognition with a missing language succeeded")
	}
}
