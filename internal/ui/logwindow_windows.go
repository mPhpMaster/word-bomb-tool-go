//go:build windows

package ui

import (
	"time"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
	"github.com/mphpmaster/word-bomb-tool-go/internal/logging"
)

// LogWindow is the main application window: a menu plus a scrolling log view,
// with a system-tray icon. It is the port of ui_manager.LogDisplay and
// tray_manager.TrayIcon.
type LogWindow struct {
	mw           *walk.MainWindow
	te           *walk.TextEdit
	queue        *logging.Queue
	cb           Callbacks
	onVisibility func(bool)
	tray         *walk.NotifyIcon
	visible      bool
	stop         chan struct{}
}

// NewLogWindow builds (but does not run) the main window and starts draining the
// log queue into the view. onVisibility, if non-nil, is called whenever the
// window is shown/hidden so the overlays can follow.
func NewLogWindow(queue *logging.Queue, cb Callbacks, onVisibility func(bool)) (*LogWindow, error) {
	lw := &LogWindow{queue: queue, cb: cb, onVisibility: onVisibility, visible: true, stop: make(chan struct{})}

	if err := (decl.MainWindow{
		AssignTo: &lw.mw,
		Title:    "WBT",
		Name:     "WBT",
		Bounds:   decl.Rectangle{X: 10, Y: 10, Width: 750, Height: 350},
		Layout:   decl.VBox{MarginsZero: true},
		Font:     decl.Font{Family: config.Theme.FontFamily, PointSize: config.Theme.FontSize},
		MenuItems: []decl.MenuItem{
			decl.Menu{
				Text:  "File",
				Items: []decl.MenuItem{decl.Action{Text: "Exit", OnTriggered: cb.Exit}},
			},
			decl.Menu{
				Text:  "Options",
				Items: lw.optionsMenu(),
			},
			decl.Menu{
				Text: "Help",
				Items: []decl.MenuItem{
					decl.Action{Text: "Show Hotkeys\t.", OnTriggered: cb.ShowHelp},
				},
			},
		},
		Children: []decl.Widget{
			decl.TextEdit{
				AssignTo: &lw.te,
				ReadOnly: true,
				VScroll:  true,
			},
		},
	}).Create(); err != nil {
		return nil, err
	}

	// Topmost, semi-transparent, dark log view.
	setTopmost(lw.mw.Handle())
	setWindowAlpha(lw.mw.Handle(), byte(config.Theme.UnfocusedAlpha*255))
	if bg, err := walk.NewSolidColorBrush(parseHexColor(config.Theme.LogBg)); err == nil {
		lw.te.SetBackground(bg)
	}
	lw.te.SetTextColor(parseHexColor(config.Theme.LogFg))

	// Focus in/out changes opacity, like the Python window.
	lw.mw.Activating().Attach(func() { setWindowAlpha(lw.mw.Handle(), byte(config.Theme.FocusedAlpha*255)) })
	lw.mw.Deactivating().Attach(func() { setWindowAlpha(lw.mw.Handle(), byte(config.Theme.UnfocusedAlpha*255)) })

	// Closing the window exits the app.
	lw.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		*canceled = true
		go cb.Exit()
	})

	lw.setupTray()

	go lw.drainLoop()
	return lw, nil
}

func (lw *LogWindow) optionsMenu() []decl.MenuItem {
	items := []decl.MenuItem{
		decl.Action{Text: "Select Region\tTab", OnTriggered: lw.cb.SelectRegion},
		decl.Action{Text: "Clear turn region\tCtrl+F2", OnTriggered: lw.cb.ClearTurnRegion},
		decl.Separator{},
	}

	searchItems := make([]decl.MenuItem, 0, len(config.SearchModes))
	for i, m := range config.SearchModes {
		idx := i
		searchItems = append(searchItems, decl.Action{Text: m, OnTriggered: func() { lw.cb.SetSearchMode(idx) }})
	}
	sortItems := make([]decl.MenuItem, 0, len(config.SortModes))
	for i, m := range config.SortModes {
		idx := i
		sortItems = append(sortItems, decl.Action{Text: m, OnTriggered: func() { lw.cb.SetSortMode(idx) }})
	}

	items = append(items,
		decl.Menu{Text: "Search Mode", Items: searchItems},
		decl.Menu{Text: "Sort Mode", Items: sortItems},
		decl.Action{Text: "Typing delay...", OnTriggered: lw.cb.SetTypingDelay},
		decl.Action{Text: "OCR interval...", OnTriggered: lw.cb.SetOCRInterval},
		decl.Separator{},
		decl.Action{Text: "Clear Typed History\tDelete", OnTriggered: lw.cb.ClearHistory},
		decl.Action{Text: "Undo Last Word\tCtrl+Z", OnTriggered: lw.cb.UndoWord},
	)
	return items
}

func (lw *LogWindow) setupTray() {
	tray, err := walk.NewNotifyIcon(lw.mw)
	if err != nil {
		logging.Warnf("Tray icon disabled: %v", err)
		return
	}
	if icon, err := walk.NewIconFromImage(makeTrayImage()); err == nil {
		_ = tray.SetIcon(icon)
	}
	_ = tray.SetToolTip("WBT")

	add := func(text string, fn func()) {
		a := walk.NewAction()
		_ = a.SetText(text)
		if fn != nil {
			a.Triggered().Attach(fn)
		} else {
			_ = a.SetEnabled(false)
		}
		_ = tray.ContextMenu().Actions().Add(a)
	}

	add("Select Region", lw.cb.SelectRegion)
	_ = tray.ContextMenu().Actions().Add(walk.NewSeparatorAction())
	add("Fetch Suggestions", lw.cb.FetchSuggestions)
	add("Fetch Definitions", lw.cb.FetchDefinitions)
	_ = tray.ContextMenu().Actions().Add(walk.NewSeparatorAction())
	add("Toggle Window", lw.cb.ToggleWindow)
	_ = tray.ContextMenu().Actions().Add(walk.NewSeparatorAction())
	add("Exit", lw.cb.Exit)

	// Left-click toggles the window, matching the tray behaviour.
	tray.MouseUp().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			lw.cb.ToggleWindow()
		}
	})
	_ = tray.SetVisible(true)
	lw.tray = tray
}

// drainLoop periodically flushes queued log entries into the text view via the
// GUI thread.
func (lw *LogWindow) drainLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-lw.stop:
			return
		case <-ticker.C:
			if !lw.queue.HasMessages() {
				continue
			}
			entries := lw.queue.PopAll()
			lw.mw.Synchronize(func() {
				for _, e := range entries {
					lw.te.AppendText(e.Message + "\r\n")
				}
			})
		}
	}
}

// Run starts the message loop; it returns when the window closes.
func (lw *LogWindow) Run() int {
	return lw.mw.Run()
}

// MainWindow returns the underlying walk window (used as a dialog owner).
func (lw *LogWindow) MainWindow() *walk.MainWindow { return lw.mw }

// Synchronize runs f on the GUI thread.
func (lw *LogWindow) Synchronize(f func()) { lw.mw.Synchronize(f) }

// ToggleVisibility shows or hides the window (Caps Lock / tray click).
func (lw *LogWindow) ToggleVisibility() {
	lw.mw.Synchronize(func() {
		if lw.mw.Visible() {
			lw.mw.SetVisible(false)
			lw.visible = false
		} else {
			lw.mw.SetVisible(true)
			lw.mw.BringToTop()
			lw.visible = true
		}
		if lw.onVisibility != nil {
			lw.onVisibility(lw.visible)
		}
	})
}

// Dispose releases the tray icon and stops the drain loop.
func (lw *LogWindow) Dispose() {
	select {
	case <-lw.stop:
	default:
		close(lw.stop)
	}
	if lw.tray != nil {
		_ = lw.tray.Dispose()
	}
}
