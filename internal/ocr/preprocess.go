package ocr

import (
	"image"
	"image/color"

	xdraw "golang.org/x/image/draw"
)

// toGray converts any image to 8-bit grayscale using luminance, matching PIL's
// convert("L").
func toGray(src image.Image) *image.Gray {
	b := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			// ITU-R 601-2 luma transform, as used by PIL "L".
			lum := (299*(r>>8) + 587*(g>>8) + 114*(bl>>8)) / 1000
			dst.SetGray(x, y, color.Gray{Y: uint8(lum)})
		}
	}
	return dst
}

// autoContrast stretches the gray histogram so the darkest kept pixel maps to 0
// and the brightest to 255. cutoff is the percentage of pixels to trim from each
// end of the histogram before computing the range (PIL ImageOps.autocontrast).
func autoContrast(img *image.Gray, cutoff int) *image.Gray {
	var hist [256]int
	for _, v := range img.Pix {
		hist[v]++
	}
	total := len(img.Pix)
	if total == 0 {
		return img
	}

	lo, hi := 0, 255
	if cutoff > 0 {
		trim := total * cutoff / 100
		// Advance lo until we've trimmed `trim` pixels from the dark end.
		n := 0
		for lo = 0; lo < 256; lo++ {
			n += hist[lo]
			if n > trim {
				break
			}
		}
		// Retreat hi until we've trimmed `trim` pixels from the bright end.
		n = 0
		for hi = 255; hi >= 0; hi-- {
			n += hist[hi]
			if n > trim {
				break
			}
		}
	} else {
		for lo = 0; lo < 256 && hist[lo] == 0; lo++ {
		}
		for hi = 255; hi >= 0 && hist[hi] == 0; hi-- {
		}
	}
	if hi <= lo {
		return img
	}

	var lut [256]uint8
	scale := 255.0 / float64(hi-lo)
	for i := 0; i < 256; i++ {
		v := float64(i-lo) * scale
		if v < 0 {
			v = 0
		} else if v > 255 {
			v = 255
		}
		lut[i] = uint8(v + 0.5)
	}

	out := image.NewGray(img.Bounds())
	for i, v := range img.Pix {
		out.Pix[i] = lut[v]
	}
	return out
}

// threshold binarizes: pixels below t become 0, others 255 (PIL point()).
func threshold(img *image.Gray, t uint8) *image.Gray {
	out := image.NewGray(img.Bounds())
	for i, v := range img.Pix {
		if v < t {
			out.Pix[i] = 0
		} else {
			out.Pix[i] = 255
		}
	}
	return out
}

// upscaleIfSmall enlarges tiny crops (Tesseract struggles on small UI text)
// while preserving aspect ratio, capped at 4x. Mirrors _upscale_if_small.
func upscaleIfSmall(img *image.Gray, minW, minH int) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return img
	}
	sx := float64(minW) / float64(w)
	if sx < 1.0 {
		sx = 1.0
	}
	sy := float64(minH) / float64(h)
	if sy < 1.0 {
		sy = 1.0
	}
	scale := sx
	if sy > scale {
		scale = sy
	}
	if scale > 4.0 {
		scale = 4.0
	}
	if scale <= 1.01 {
		return img
	}
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	dst := image.NewGray(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}

// preprocessLetters is the harsh pipeline for the letter region: grayscale,
// autocontrast, hard threshold at 140.
func preprocessLetters(src image.Image) *image.Gray {
	g := toGray(src)
	g = autoContrast(g, 0)
	return threshold(g, 140)
}

// preprocessTurnGate is the softer pipeline for colored "YOUR TURN" UI:
// grayscale, autocontrast (cutoff 1), upscale small crops.
func preprocessTurnGate(src image.Image) *image.Gray {
	g := toGray(src)
	g = autoContrast(g, 1)
	return upscaleIfSmall(g, 140, 48)
}
