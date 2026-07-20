//go:build windows

package ui

import (
	"errors"

	"github.com/lxn/walk"
	"github.com/lxn/win"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
)

// ErrSelectionCancelled is returned when the user aborts region selection.
var ErrSelectionCancelled = errors.New("region selection cancelled")

// SelectRegion opens a fullscreen, semi-transparent picker and returns the
// dragged rectangle in virtual-screen coordinates. Must be called on the GUI
// thread. Pressing Escape (or selecting nothing) cancels.
func SelectRegion(owner walk.Form) (*config.Region, error) {
	dlg, err := walk.NewDialog(owner)
	if err != nil {
		return nil, err
	}
	defer dlg.Dispose()

	vx, vy, vw, vh := virtualScreen()

	// Frameless, topmost, translucent black overlay across the whole desktop.
	setPopupStyle(dlg.Handle())
	addExStyle(dlg.Handle(), win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW)
	dlg.SetBoundsPixels(walk.Rectangle{X: vx, Y: vy, Width: vw, Height: vh})
	setWindowAlpha(dlg.Handle(), 77) // ~0.30 opacity
	if bg, e := walk.NewSolidColorBrush(walk.RGB(0, 0, 0)); e == nil {
		dlg.SetBackground(bg)
	}

	var (
		startX, startY int
		curX, curY     int
		dragging       bool
		haveResult     bool
		result         config.Region
		pen            walk.Color = parseHexColor(config.Theme.Accent)
	)

	cw, err := walk.NewCustomWidgetPixels(dlg, 0, func(canvas *walk.Canvas, _ walk.Rectangle) error {
		if !dragging {
			return nil
		}
		x1, y1 := minInt(startX, curX), minInt(startY, curY)
		x2, y2 := maxInt(startX, curX), maxInt(startY, curY)
		br, _ := walk.NewSolidColorBrush(pen)
		if br == nil {
			return nil
		}
		defer br.Dispose()
		const t = 2
		for _, r := range []walk.Rectangle{
			{X: x1, Y: y1, Width: x2 - x1, Height: t},
			{X: x1, Y: y2 - t, Width: x2 - x1, Height: t},
			{X: x1, Y: y1, Width: t, Height: y2 - y1},
			{X: x2 - t, Y: y1, Width: t, Height: y2 - y1},
		} {
			_ = canvas.FillRectanglePixels(br, r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	cw.SetBoundsPixels(walk.Rectangle{X: 0, Y: 0, Width: vw, Height: vh})

	cw.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		startX, startY, curX, curY = x, y, x, y
		dragging = true
	})
	cw.MouseMove().Attach(func(x, y int, button walk.MouseButton) {
		if dragging {
			curX, curY = x, y
			cw.Invalidate()
		}
	})
	cw.MouseUp().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton || !dragging {
			return
		}
		dragging = false
		x1, y1 := minInt(startX, x), minInt(startY, y)
		x2, y2 := maxInt(startX, x), maxInt(startY, y)
		result = config.Region{Left: vx + x1, Top: vy + y1, Width: x2 - x1, Height: y2 - y1}
		haveResult = result.Width > 0 && result.Height > 0
		dlg.Accept()
	})

	dlg.KeyPress().Attach(func(key walk.Key) {
		if key == walk.KeyEscape {
			dlg.Cancel()
		}
	})

	dlg.Run()

	if !haveResult {
		return nil, ErrSelectionCancelled
	}
	return &result, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
