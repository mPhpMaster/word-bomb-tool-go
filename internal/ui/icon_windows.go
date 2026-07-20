//go:build windows

package ui

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
)

// makeTrayImage draws the simple "WB" tray icon (bordered box + letters),
// mirroring tray_manager._create_icon_image.
func makeTrayImage() image.Image {
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	bg := hexToNRGBA(config.Theme.Bg)
	accent := hexToNRGBA(config.Theme.Accent)
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	// Accent border.
	const border = 8
	drawRectOutline(img, image.Rect(border, border, size-border, size-border), accent, 2)

	// "WB" label using the built-in bitmap face.
	d := &font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{accent},
		Face: basicfont.Face7x13,
		Dot:  fixed.P(size/4, size/2+5),
	}
	d.DrawString("WB")
	return img
}

func drawRectOutline(img *image.RGBA, r image.Rectangle, c color.Color, width int) {
	for i := 0; i < width; i++ {
		rr := image.Rect(r.Min.X+i, r.Min.Y+i, r.Max.X-i, r.Max.Y-i)
		// top & bottom
		for x := rr.Min.X; x < rr.Max.X; x++ {
			img.Set(x, rr.Min.Y, c)
			img.Set(x, rr.Max.Y-1, c)
		}
		// left & right
		for y := rr.Min.Y; y < rr.Max.Y; y++ {
			img.Set(rr.Min.X, y, c)
			img.Set(rr.Max.X-1, y, c)
		}
	}
}

func hexToNRGBA(hex string) color.NRGBA {
	c := colorRef(hex) // 0x00BBGGRR
	return color.NRGBA{
		R: uint8(c & 0xFF),
		G: uint8((c >> 8) & 0xFF),
		B: uint8((c >> 16) & 0xFF),
		A: 255,
	}
}
