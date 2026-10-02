package ocr

import (
	"image"
	"image/draw"
	"math"
	"strconv"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// otsuLevel computes an optimal global threshold via Otsu's method so
// binarization adapts to the button/background brightness.
func otsuLevel(img *image.Gray) uint8 {
	var hist [256]int
	for _, v := range img.Pix {
		hist[v]++
	}
	total := len(img.Pix)
	if total == 0 {
		return 128
	}
	var sum float64
	for i := 0; i < 256; i++ {
		sum += float64(i) * float64(hist[i])
	}
	var sumB float64
	wB := 0
	maxVar := -1.0
	first, last := 128, 128
	for t := 0; t < 256; t++ {
		wB += hist[t]
		if wB == 0 {
			continue
		}
		wF := total - wB
		if wF == 0 {
			break
		}
		sumB += float64(t) * float64(hist[t])
		mB := sumB / float64(wB)
		mF := (sum - sumB) / float64(wF)
		between := float64(wB) * float64(wF) * (mB - mF) * (mB - mF)
		if between > maxVar {
			maxVar, first, last = between, t, t
		} else if between == maxVar {
			last = t
		}
	}
	// The midpoint of the optimal plateau places the cut cleanly between the two
	// pixel clusters (matters for flat, bimodal histograms).
	return uint8((first + last) / 2)
}

// majorityDark reports whether most pixels are dark (a dark background).
func majorityDark(img *image.Gray) bool {
	dark := 0
	for _, v := range img.Pix {
		if v < 128 {
			dark++
		}
	}
	return dark*2 > len(img.Pix)
}

// invert returns a photo-negative of the image.
func invert(img *image.Gray) *image.Gray {
	out := image.NewGray(img.Bounds())
	for i, v := range img.Pix {
		out.Pix[i] = 255 - v
	}
	return out
}

// pad adds a uniform border (a "quiet zone") around the image.
func pad(img *image.Gray, p int, fill uint8) *image.Gray {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	out := image.NewGray(image.Rect(0, 0, w+2*p, h+2*p))
	for i := range out.Pix {
		out.Pix[i] = fill
	}
	draw.Draw(out, image.Rect(p, p, p+w, p+h), img, img.Bounds().Min, draw.Src)
	return out
}

// resize scales to exactly nw x nh the way GDI+ DrawImage does with
// HighQualityBicubic (which the C# version uses): a separable cubic filter,
// taps outside the source dropped. The kernel parameters were fitted to GDI+
// output on glyph crops: a softer cubic when enlarging, close to Catmull-Rom
// when shrinking. Plain Catmull-Rom made the Arabic engine read a few Latin
// prompts ("QU") as Arabic.
func resize(img *image.Gray, nw, nh int) *image.Gray {
	nw, nh = max(1, nw), max(1, nh)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	out := image.NewGray(image.Rect(0, 0, nw, nh))
	if w <= 0 || h <= 0 {
		return out
	}
	img = toZeroGray(img)

	// Horizontal pass into a float buffer, then vertical.
	tmp := make([]float64, h*nw)
	row := make([]float64, w)
	hx := newResampler(w, nw)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			row[x] = float64(img.Pix[y*img.Stride+x])
		}
		hx.apply(row, tmp[y*nw:(y+1)*nw])
	}
	col := make([]float64, h)
	res := make([]float64, nh)
	vy := newResampler(h, nh)
	for x := 0; x < nw; x++ {
		for y := 0; y < h; y++ {
			col[y] = tmp[y*nw+x]
		}
		vy.apply(col, res)
		for y, v := range res {
			out.Pix[y*out.Stride+x] = uint8(math.Max(0, math.Min(255, math.Round(v))))
		}
	}
	return out
}

// cubic is the Mitchell-Netravali family of cubic filters with parameters B, C.
func cubic(b, c float64) func(float64) float64 {
	return func(x float64) float64 {
		x = math.Abs(x)
		switch {
		case x < 1:
			return ((12-9*b-6*c)*x*x*x + (-18+12*b+6*c)*x*x + (6 - 2*b)) / 6
		case x < 2:
			return ((-b-6*c)*x*x*x + (6*b+30*c)*x*x + (-12*b-48*c)*x + (8*b + 24*c)) / 6
		}
		return 0
	}
}

type tap struct {
	lo      int
	weights []float64
}

// resampler holds precomputed filter taps for scaling n samples to m.
type resampler struct {
	n    int
	taps []tap
}

func newResampler(n, m int) *resampler {
	scale := float64(n) / float64(m)
	k := cubic(0.4, 0.4) // enlarging
	if scale > 1 {
		k = cubic(0, 0.5) // shrinking (Catmull-Rom), stretched over the source
	}
	fs := math.Max(1, scale)
	r := &resampler{n: n, taps: make([]tap, m)}
	for x := 0; x < m; x++ {
		c := (float64(x)+0.5)*scale - 0.5
		lo, hi := int(math.Floor(c-2*fs)), int(math.Ceil(c+2*fs))
		ws := make([]float64, hi-lo+1)
		var sum float64
		for j := lo; j <= hi; j++ {
			ws[j-lo] = k((float64(j) - c) / fs)
			if j >= 0 && j < n {
				sum += ws[j-lo] // taps outside the source are dropped
			}
		}
		if sum != 0 {
			for i := range ws {
				ws[i] /= sum
			}
		}
		r.taps[x] = tap{lo: lo, weights: ws}
	}
	return r
}

func (r *resampler) apply(src, dst []float64) {
	for x, t := range r.taps {
		var v float64
		for i, wt := range t.weights {
			if j := t.lo + i; j >= 0 && j < r.n {
				v += wt * src[j]
			}
		}
		dst[x] = v
	}
}

// inkBounds returns the bounding box of the dark (text) pixels; ok is false
// when there are none.
func inkBounds(img *image.Gray) (image.Rectangle, bool) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+w]
		for x, v := range row {
			if v >= 128 {
				continue
			}
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	if maxX < 0 {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

// crop copies r out of img into a new image with a zero origin.
func crop(img *image.Gray, r image.Rectangle) *image.Gray {
	out := image.NewGray(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out
}

// blit copies src into dst with its left edge at x.
func blit(src, dst *image.Gray, x int) {
	draw.Draw(dst, image.Rect(x, 0, x+src.Bounds().Dx(), src.Bounds().Dy()), src, image.Point{}, draw.Src)
}

// toZeroGray returns img as an *image.Gray whose bounds start at (0,0).
func toZeroGray(img *image.Gray) *image.Gray {
	if img.Bounds().Min == (image.Point{}) {
		return img
	}
	return crop(img, img.Bounds())
}

// PreprocessForWindowsOCR is the pipeline for the Windows OCR engine. The
// engine skips short, word-less clusters such as "AB" or "TR" when they stand
// alone, so the prompt is laid out as a normal line of text: a known anchor
// word followed by copies of the prompt, all black on white at textHeight
// pixels with a wide margin. The caller drops the anchor and takes the
// majority reading of the copies. An empty anchor lays out the copies only.
// Returns an empty image when the region holds no ink.
func PreprocessForWindowsOCR(src image.Image, anchor string, textHeight, copies int, gap float64) *image.Gray {
	g := toGray(src)
	g = autoContrast(g, 2)
	g = threshold(g, otsuLevel(g))
	if majorityDark(g) {
		g = invert(g)
	}
	g = KeepMainText(g)
	box, ok := inkBounds(g)
	if !ok {
		return image.NewGray(image.Rectangle{})
	}
	g = crop(g, box)
	g = resize(g, max(1, int(math.Round(float64(g.Bounds().Dx())*float64(textHeight)/float64(g.Bounds().Dy())))), textHeight)

	a := image.NewGray(image.Rect(0, 0, 0, textHeight))
	if anchor != "" {
		a = renderAnchor(anchor, textHeight)
	}
	gp := int(math.Round(float64(textHeight) * gap))
	aw, gw := a.Bounds().Dx(), g.Bounds().Dx()
	lead := 0
	if aw > 0 {
		lead = aw + gp
	}
	line := image.NewGray(image.Rect(0, 0, lead+copies*gw+(copies-1)*gp, textHeight))
	for i := range line.Pix {
		line.Pix[i] = 255
	}
	if aw > 0 {
		blit(a, line, 0)
	}
	for k := 0; k < copies; k++ {
		blit(g, line, lead+k*(gw+gp))
	}
	return pad(line, textHeight, 255)
}

type blob struct {
	x0, y0, x1, y1, n int
}

// KeepMainText keeps only the prompt's own glyphs in a black-on-white image:
// it drops thin lines spanning the region (box edges), then every blob less
// than half as tall as the tallest one unless it sits over or under a big glyph
// (dots of i/j and Arabic letters). That removes small corner text such as a
// "1K" counter and specks from animated backgrounds.
func KeepMainText(img *image.Gray) *image.Gray {
	img = toZeroGray(img)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	dark := func(i int) bool { return img.Pix[(i/w)*img.Stride+i%w] < 128 }

	label := make([]int, w*h)
	var blobs []blob
	var stack []int
	for start := range label {
		if label[start] != 0 || !dark(start) {
			continue
		}
		id := len(blobs) + 1
		b := blob{x0: w, y0: h, x1: -1, y1: -1}
		label[start] = id
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			b.n++
			px, py := p%w, p/w
			b.x0, b.x1 = min(b.x0, px), max(b.x1, px)
			b.y0, b.y1 = min(b.y0, py), max(b.y1, py)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := px+dx, py+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					q := ny*w + nx
					if label[q] == 0 && dark(q) {
						label[q] = id
						stack = append(stack, q)
					}
				}
			}
		}
		blobs = append(blobs, b)
	}
	if len(blobs) == 0 {
		return img
	}

	isLine := func(b blob) bool {
		bw, bh := float64(b.x1-b.x0+1), float64(b.y1-b.y0+1)
		fw, fh := float64(w), float64(h)
		return (bh >= fh*0.9 && bw <= math.Max(3, bh*0.12)) ||
			(bw >= fw*0.9 && bh <= math.Max(3, bw*0.12)) ||
			// A frame around the whole region: huge box, very little ink.
			(bw >= fw*0.9 && bh >= fh*0.9 && float64(b.n) < bw*bh*0.25)
	}

	maxH := 0
	for _, b := range blobs {
		if !isLine(b) {
			maxH = max(maxH, b.y1-b.y0+1)
		}
	}
	if maxH == 0 {
		return img
	}

	keep := make([]bool, len(blobs)+1)
	var big []int
	for i, b := range blobs {
		if !isLine(b) && float64(b.y1-b.y0+1) >= float64(maxH)*0.5 {
			big = append(big, i)
			keep[i+1] = true
		}
	}
	reach := int(float64(maxH) * 0.7)
	for i, b := range blobs {
		if keep[i+1] || isLine(b) {
			continue
		}
		for _, j := range big {
			g := blobs[j]
			if b.x1 >= g.x0 && b.x0 <= g.x1 && b.y1 >= g.y0-reach && b.y0 <= g.y1+reach {
				keep[i+1] = true
				break
			}
		}
	}

	out := image.NewGray(image.Rect(0, 0, w, h))
	for i, l := range label {
		if l != 0 && keep[l] {
			out.Pix[i] = 0
		} else {
			out.Pix[i] = 255
		}
	}
	return out
}

var (
	anchorFontOnce sync.Once
	anchorFont     *opentype.Font
	anchorMu       sync.Mutex
	anchorCache    = map[string]*image.Gray{}
)

// renderAnchor renders a known word in a bold sans font (Go Bold), cropped to
// its ink and scaled to textHeight. Results are cached.
func renderAnchor(word string, textHeight int) *image.Gray {
	key := word + "/" + strconv.Itoa(textHeight)
	anchorMu.Lock()
	defer anchorMu.Unlock()
	if a, ok := anchorCache[key]; ok {
		return a
	}
	anchorFontOnce.Do(func() { anchorFont, _ = opentype.Parse(gobold.TTF) })
	empty := image.NewGray(image.Rect(0, 0, 0, textHeight))
	if anchorFont == nil {
		return empty
	}
	size := float64(textHeight) * 1.4
	face, err := opentype.NewFace(anchorFont, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return empty
	}
	defer face.Close()

	canvas := image.NewGray(image.Rect(0, 0, textHeight*(len(word)+2)*2, textHeight*3))
	for i := range canvas.Pix {
		canvas.Pix[i] = 255
	}
	d := &font.Drawer{Dst: canvas, Src: image.Black, Face: face,
		Dot: fixed.P(textHeight/2, textHeight/4+face.Metrics().Ascent.Ceil())}
	d.DrawString(word)

	box, ok := inkBounds(threshold(canvas, 128))
	if !ok {
		return empty
	}
	g := crop(canvas, box)
	a := resize(g, max(1, int(math.Round(float64(g.Bounds().Dx())*float64(textHeight)/float64(g.Bounds().Dy())))), textHeight)
	anchorCache[key] = a
	return a
}
