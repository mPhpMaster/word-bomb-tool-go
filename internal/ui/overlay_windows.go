//go:build windows

package ui

import (
	"github.com/lxn/walk"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
)

// colorKeyHex is punched out of the overlay windows so only their borders show.
const colorKeyHex = "#ff00ff"

// borderWin is a single frameless, click-through, topmost overlay that draws a
// colored rectangle border with a fully transparent interior.
type borderWin struct {
	mw       *walk.MainWindow
	border   walk.Color
	keyBrush *walk.SolidColorBrush
	borderBr *walk.SolidColorBrush
}

func newBorderWin(borderHex string) (*borderWin, error) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return nil, err
	}
	bw := &borderWin{mw: mw, border: parseHexColor(borderHex)}

	// Frameless popup.
	setPopupStyle(mw.Handle())
	makeToolOverlay(mw.Handle())
	setWindowColorKey(mw.Handle(), colorRef(colorKeyHex))

	bw.keyBrush, _ = walk.NewSolidColorBrush(parseHexColor(colorKeyHex))
	bw.borderBr, _ = walk.NewSolidColorBrush(bw.border)
	if bw.keyBrush != nil {
		mw.SetBackground(bw.keyBrush)
	}

	cw, err := walk.NewCustomWidgetPixels(mw, 0, bw.paint)
	if err != nil {
		return nil, err
	}
	cw.SetBounds(walk.Rectangle{X: 0, Y: 0, Width: 10, Height: 10})
	mw.SetVisible(false)
	return bw, nil
}

func (bw *borderWin) paint(canvas *walk.Canvas, _ walk.Rectangle) error {
	b := bw.mw.ClientBoundsPixels()
	// Fill with the key color (transparent) then draw a 2px border.
	if bw.keyBrush != nil {
		_ = canvas.FillRectanglePixels(bw.keyBrush, b)
	}
	if bw.borderBr == nil {
		return nil
	}
	const t = 2
	rects := []walk.Rectangle{
		{X: 0, Y: 0, Width: b.Width, Height: t},                       // top
		{X: 0, Y: b.Height - t, Width: b.Width, Height: t},            // bottom
		{X: 0, Y: 0, Width: t, Height: b.Height},                      // left
		{X: b.Width - t, Y: 0, Width: t, Height: b.Height},            // right
	}
	for _, r := range rects {
		_ = canvas.FillRectanglePixels(bw.borderBr, r)
	}
	return nil
}

func (bw *borderWin) show(r config.Region) {
	bw.mw.SetBoundsPixels(walk.Rectangle{X: r.Left, Y: r.Top, Width: r.Width, Height: r.Height})
	bw.mw.SetVisible(true)
	setTopmost(bw.mw.Handle())
	bw.mw.Invalidate()
}

func (bw *borderWin) hide() { bw.mw.SetVisible(false) }

// RegionOverlay shows the selected letter region (accent border) and the
// optional turn-gate region (success/green border). Port of RegionOverlay.
type RegionOverlay struct {
	letter        *borderWin
	turn          *borderWin
	region        *config.Region
	turnRegion    *config.Region
	bundleVisible bool
}

// NewRegionOverlay creates both overlay windows (initially hidden). Must be
// called on the GUI thread.
func NewRegionOverlay() (*RegionOverlay, error) {
	l, err := newBorderWin(config.Theme.Accent)
	if err != nil {
		return nil, err
	}
	t, err := newBorderWin(config.Theme.Success)
	if err != nil {
		return nil, err
	}
	return &RegionOverlay{letter: l, turn: t, bundleVisible: true}, nil
}

// ShowRegion displays the letter region and, if set, the turn region. Must run
// on the GUI thread.
func (o *RegionOverlay) ShowRegion(region, turnRegion *config.Region) {
	o.region = region
	o.turnRegion = turnRegion
	o.apply()
}

// SetBundleVisible hides/shows both overlays with the log window.
func (o *RegionOverlay) SetBundleVisible(visible bool) {
	o.bundleVisible = visible
	o.apply()
}

func (o *RegionOverlay) apply() {
	if o.region == nil || !o.bundleVisible {
		o.letter.hide()
	} else {
		o.letter.show(*o.region)
	}
	if o.turnRegion == nil || !o.bundleVisible {
		o.turn.hide()
	} else {
		o.turn.show(*o.turnRegion)
	}
}
