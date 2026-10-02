package ui

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed assets/author.jpg
var authorJPEG []byte

// avatarImage returns the author photo cropped to a circle of the given size
// over bg, with a faint light outline. Without a usable photo it draws the
// initials on a dark circle instead.
func avatarImage(size int, bg, accent color.NRGBA, initials string) *image.NRGBA {
	// The circle's content, square.
	content := image.NewNRGBA(image.Rect(0, 0, size, size))
	if photo, err := jpeg.Decode(bytes.NewReader(authorJPEG)); err == nil {
		// Scale to cover the square, centred.
		b := photo.Bounds()
		scale := math.Max(float64(size)/float64(b.Dx()), float64(size)/float64(b.Dy()))
		w, h := int(math.Ceil(float64(b.Dx())*scale)), int(math.Ceil(float64(b.Dy())*scale))
		dst := image.Rect((size-w)/2, (size-h)/2, (size-w)/2+w, (size-h)/2+h)
		xdraw.CatmullRom.Scale(content, dst, photo, b, draw.Src, nil)
	} else {
		draw.Draw(content, content.Bounds(), image.NewUniform(color.NRGBA{0x1F, 0x29, 0x37, 0xFF}), image.Point{}, draw.Src)
		drawInitials(content, initials, accent)
	}

	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	r := float64(size) / 2
	outline := color.NRGBA{255, 255, 255, 60}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Coverage of this pixel by the circle, 4x4 supersampled for a smooth edge,
			// and of the 1px outline ring just inside it.
			var in, ring float64
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					dx := float64(x) + (float64(sx)+0.5)/4 - r
					dy := float64(y) + (float64(sy)+0.5)/4 - r
					d := math.Sqrt(dx*dx + dy*dy)
					if d <= r {
						in++
						if d >= r-1 {
							ring++
						}
					}
				}
			}
			in /= 16
			ring /= 16
			c := content.NRGBAAt(x, y)
			c = blend(c, outline, ring/math.Max(in, 1e-9))
			out.SetNRGBA(x, y, blend(bg, c, in))
		}
	}
	return out
}

// blend mixes top over base with the given coverage (0..1), also scaled by
// top's own alpha.
func blend(base, top color.NRGBA, coverage float64) color.NRGBA {
	a := coverage * float64(top.A) / 255
	mix := func(b, t uint8) uint8 { return uint8(math.Round(float64(b)*(1-a) + float64(t)*a)) }
	return color.NRGBA{mix(base.R, top.R), mix(base.G, top.G), mix(base.B, top.B), 255}
}

func drawInitials(img *image.NRGBA, initials string, c color.NRGBA) {
	f, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return
	}
	size := img.Bounds().Dx()
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: float64(size) * 0.32 * 96 / 72, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer face.Close()
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face}
	m := face.Metrics()
	adv := d.MeasureString(initials)
	d.Dot = fixed.Point26_6{
		X: (fixed.I(size) - adv) / 2,
		Y: (fixed.I(size)-(m.Ascent+m.Descent))/2 + m.Ascent,
	}
	d.DrawString(initials)
}
