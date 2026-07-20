package ocr

import (
	"image"
	"image/color"
	"testing"
)

func TestPreprocessLettersThreshold(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 1))
	// A near-black and a near-white pixel plus two mid values.
	src.Set(0, 0, color.RGBA{10, 10, 10, 255})
	src.Set(1, 0, color.RGBA{100, 100, 100, 255})
	src.Set(2, 0, color.RGBA{200, 200, 200, 255})
	src.Set(3, 0, color.RGBA{250, 250, 250, 255})

	out := preprocessLetters(src)
	// After autocontrast+threshold the result must be strictly binary.
	for _, v := range out.Pix {
		if v != 0 && v != 255 {
			t.Fatalf("non-binary pixel after threshold: %d", v)
		}
	}
	// Darkest pixel should end at 0, brightest at 255.
	if out.Pix[0] != 0 || out.Pix[3] != 255 {
		t.Fatalf("threshold endpoints wrong: %v", out.Pix)
	}
}

func TestUpscaleSmallGrowsToMinimum(t *testing.T) {
	g := image.NewGray(image.Rect(0, 0, 20, 10))
	out := upscaleIfSmall(g, 140, 48)
	if out.Bounds().Dx() <= 20 {
		t.Fatalf("expected upscaled width, got %d", out.Bounds().Dx())
	}
	// Capped at 4x.
	if out.Bounds().Dx() > 80 {
		t.Fatalf("width exceeded 4x cap: %d", out.Bounds().Dx())
	}
}
