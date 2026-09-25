//go:build windows

// Package app wires the Word Bomb Tool GUI together: OCR, the Datamuse client,
// state, hotkeys, typing and the walk UI. It is the Go port of main.py.
package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
	"github.com/mphpmaster/word-bomb-tool-go/internal/datamuse"
	"github.com/mphpmaster/word-bomb-tool-go/internal/input"
	"github.com/mphpmaster/word-bomb-tool-go/internal/logging"
	"github.com/mphpmaster/word-bomb-tool-go/internal/ocr"
	"github.com/mphpmaster/word-bomb-tool-go/internal/state"
	"github.com/mphpmaster/word-bomb-tool-go/internal/suggest"
	"github.com/mphpmaster/word-bomb-tool-go/internal/ui"
)

// App is the running application.
type App struct {
	state   *state.Manager
	ocr     *ocr.Processor
	api     *datamuse.Client
	queue   *logging.Queue
	hook    *input.Hook
	logWin  *ui.LogWindow
	overlay *ui.RegionOverlay

	sem              chan struct{} // bounds concurrent shift/alt handlers
	autoWatcherReset atomic.Bool

	// actionMu serializes whole OCR -> fetch -> type actions (Shift, Alt+1, auto
	// mode) so two actions never type at once (interleaved keys like "llikike").
	// Later commands wait for the running one to finish.
	actionMu sync.Mutex
	// pendingManual counts Shift actions queued or running; auto mode stays out
	// of the way meanwhile.
	pendingManual atomic.Int32
	// lastHandled holds the letters most recently handled by any action; auto
	// mode skips these. Guarded by lastHandledMu.
	lastHandledMu sync.Mutex
	lastHandled   string
}

// New constructs the application and loads persisted state.
func New() *App {
	a := &App{
		state: state.NewManager(),
		ocr:   ocr.NewProcessor(),
		api:   datamuse.New(),
		queue: logging.NewQueue(config.MaxLogQueueSize),
		hook:  input.NewHook(),
		sem:   make(chan struct{}, config.MaxWorkerThreads),
	}
	a.state.LoadState()
	return a
}

// log records a message to both the file log and the UI queue.
func (a *App) log(message, level string) {
	a.queue.Add(message, logging.Level(level))
	switch level {
	case "ERROR":
		logging.Errorf("%s", message)
	case "WARNING":
		logging.Warnf("%s", message)
	default:
		logging.Infof("%s", message)
	}
}

func (a *App) submit(fn func()) {
	go func() {
		a.sem <- struct{}{}
		defer func() { <-a.sem }()
		fn()
	}()
}

// ---- turn gate -----------------------------------------------------------

func (a *App) turnGateAccepts(text string) bool {
	if text == "" {
		return false
	}
	hasYour := strings.Contains(text, config.TurnGateNeedYour)
	hasTurn := strings.Contains(text, config.TurnGateNeedTurn)
	return (hasYour && hasTurn) || strings.Contains(text, "yourturn") || (hasYour && len(text) >= 4)
}

// autoModeTurnOK reports whether auto mode may type now, plus the turn OCR text.
func (a *App) autoModeTurnOK() (bool, string) {
	s := a.state.Snapshot()
	if s.TurnRegion == nil {
		return true, ""
	}
	text := a.ocr.PerformOCRTurnGate(*s.TurnRegion)
	if text == "" {
		return false, ""
	}
	return a.turnGateAccepts(text), text
}

// ---- display text --------------------------------------------------------

func (a *App) stateText() string {
	s := a.state.Snapshot()
	mode := config.SearchModes[clampIndex(s.CurrentModeIndex, len(config.SearchModes))]
	sortMode := config.SortModes[clampIndex(s.CurrentSortModeIndex, len(config.SortModes))]
	tg := "Off (no second region — auto types on any letter change)"
	if s.TurnRegion != nil {
		tg = "On (turn region must OCR as YOUR TURN)"
	}
	auto := "Off"
	if s.AutoModeActive {
		auto = "On"
	}
	return fmt.Sprintf(`
Current Mode: %s
Current Sort: %s
Typing delay: %gs (~avg between keys)
OCR interval: %gs (auto mode poll)
Auto mode only on your turn: %s
Auto Mode: %s
Words Typed: %d
API Status: %s
`, mode, sortMode, s.TypingDelay, s.OCRInterval, tg, auto, s.TotalTypedCount, s.APIStatus)
}

func (a *App) helpText() string {
	return a.stateText() + `
=====================================
HOTKEYS
=====================================
Fetch Suggestions:  SHIFT
Fetch Definitions:  Alt+1
Select Regions:     TAB — letters, then YOUR TURN box (Esc to skip second)
Clear turn region:  Ctrl+F2

Change Search Mode: Page Up
Change Sort Mode:   Page Down

Clear History:      Delete
Undo Last Word:     Ctrl+Z
Toggle Log + Region: Caps Lock
Toggle Auto Mode:   F1

Show This Window:   . (period)
Quit Application:   Ctrl+C
`
}

// ---- shift (suggestions) -------------------------------------------------

func (a *App) handleShiftPress() {
	s := a.state.Snapshot()
	resumeAuto := false
	if s.AutoModeActive {
		a.state.Mutate(func(st *state.AppState) { st.AutoModeActive = false })
		resumeAuto = true
	}
	if s.Region == nil {
		a.log("Cannot perform WBT: No region selected.", "ERROR")
		a.selectRegion()
		if resumeAuto {
			a.state.Mutate(func(st *state.AppState) { st.AutoModeActive = true })
		}
		return
	}
	a.pendingManual.Add(1)
	a.submit(func() {
		defer func() {
			a.pendingManual.Add(-1)
			if resumeAuto {
				a.state.Mutate(func(st *state.AppState) { st.AutoModeActive = true })
			}
		}()
		a.actionMu.Lock()
		defer a.actionMu.Unlock()
		a.handleShiftAsync("shift", "")
	})
}

func (a *App) getLastHandled() string {
	a.lastHandledMu.Lock()
	defer a.lastHandledMu.Unlock()
	return a.lastHandled
}

func (a *App) setLastHandled(letters string) {
	a.lastHandledMu.Lock()
	a.lastHandled = letters
	a.lastHandledMu.Unlock()
}

// stableOCR is letter OCR that only trusts a reading seen on two captures in a
// row. A single capture can land on a transition frame and misread letters that
// are not really on screen. Returns "" when no two consecutive readings agree.
func (a *App) stableOCR(region config.Region) string {
	prev, ok := a.ocr.PerformOCR(region)
	if !ok {
		prev = ""
	}
	for i := 1; i < config.OCRStableAttempts; i++ {
		time.Sleep(config.OCRStableGap)
		cur, ok := a.ocr.PerformOCR(region)
		if !ok {
			cur = ""
		}
		if cur != "" && cur == prev {
			return cur
		}
		prev = cur
	}
	return ""
}

// lettersChanged re-reads the letters and returns them if they differ from
// expected, otherwise "".
func (a *App) lettersChanged(region *config.Region, expected string) string {
	if region == nil || expected == "" {
		return ""
	}
	if current := a.stableOCR(*region); current != "" && current != expected {
		return current
	}
	return ""
}

// handleShiftAsync reads letters, fetches suggestions and types the first/next
// word. Callers must hold actionMu. Pass letters to reuse an OCR result already
// taken, or "" to OCR the region now. If the letters on screen change before the
// word is submitted, the stale word is dropped and the new letters are used.
func (a *App) handleShiftAsync(typingSource, letters string) {
	s := a.state.Snapshot()
	if s.Region == nil {
		return
	}

	a.log("Processing WBT...", "INFO")
	if letters == "" {
		letters = a.stableOCR(*s.Region)
	}
	if letters == "" {
		a.log("WBT returned no characters.", "WARNING")
		return
	}

	for i := 0; i <= config.MaxLetterChanges; i++ {
		newLetters := a.handleLetters(letters, typingSource, s.Region)
		if newLetters == "" {
			return
		}
		a.log(fmt.Sprintf("Letters changed on screen: '%s' -> '%s' (using new letters).", letters, newLetters), "INFO")
		letters = newLetters
	}
	a.log("Letters keep changing on screen; skipped.", "WARNING")
}

// handleLetters suggests and types a word for letters; it returns the new
// letters if they changed on screen before the word was submitted, else "".
func (a *App) handleLetters(letters, typingSource string, region *config.Region) string {
	a.setLastHandled(letters)

	s := a.state.Snapshot()
	mode := config.SearchModes[clampIndex(s.CurrentModeIndex, len(config.SearchModes))]

	if letters == s.LastOCRText && len(s.Suggestions) > 0 {
		return a.typeNextWord(typingSource, region)
	}

	a.state.Mutate(func(st *state.AppState) { st.LastOCRText = letters })
	a.log(fmt.Sprintf("--- WBT: %s ---", letters), "INFO")

	suggestions := a.api.Suggestions(letters, mode)
	a.state.Mutate(func(st *state.AppState) { st.APIStatus = a.api.Status() })

	if len(suggestions) > 0 {
		s = a.state.Snapshot()
		suggestions = suggest.Sort(suggestions, config.SortModes[clampIndex(s.CurrentSortModeIndex, len(config.SortModes))])
		a.state.Mutate(func(st *state.AppState) {
			st.Suggestions = suggestions
			st.SuggestionIndex = 0
		})
		a.log(fmt.Sprintf("Found %d suggestions.", len(suggestions)), "INFO")
		for i, sug := range suggestions {
			if i >= 3 {
				break
			}
			a.log(fmt.Sprintf("\t%d. %s", i+1, sug), "INFO")
		}
		if len(suggestions) > 3 {
			a.log(fmt.Sprintf("\n... and %d more", len(suggestions)-3), "INFO")
		}
	} else {
		a.state.Mutate(func(st *state.AppState) {
			st.Suggestions = nil
			st.SuggestionIndex = 0
		})
	}

	return a.typeNextWord(typingSource, region)
}

// typeNextWord types the next untyped suggestion. With a region, the letters are
// re-read before typing and before Enter; if they changed, the word is not
// submitted (erased if already typed) and the new letters are returned.
func (a *App) typeNextWord(typingSource string, region *config.Region) string {
	s := a.state.Snapshot()
	if len(s.Suggestions) == 0 {
		a.log("No suggestions loaded.", "WARNING")
		return ""
	}

	word, nextIdx := suggest.NextUntyped(s.Suggestions, s.SuggestionIndex, s.TypedWordsHistory)
	if word == "" {
		a.log("All available suggestions have been typed.", "WARNING")
		return ""
	}

	// "Thinking" pause before typing (auto slightly longer than Shift).
	if typingSource == "auto" {
		sleepSeconds(uniform(0.52, 1.12))
	} else {
		sleepSeconds(uniform(0.3, 0.72))
	}

	if changed := a.lettersChanged(region, s.LastOCRText); changed != "" {
		return changed
	}

	a.log(fmt.Sprintf("Typing: '%s'", word), "INFO")
	scale := 1.22
	if typingSource == "auto" {
		scale = 1.32
	}
	typeWordHumanLike(word, s.TypingDelay, scale)
	sleepSeconds(uniform(0.26, 0.62))

	if changed := a.lettersChanged(region, s.LastOCRText); changed != "" {
		a.log(fmt.Sprintf("Erasing '%s' (letters changed before Enter).", word), "INFO")
		for range []rune(word) {
			input.PressBackspace()
			sleepSeconds(uniform(0.03, 0.08))
		}
		return changed
	}

	input.PressEnter()

	a.state.AddTypingRecord(word, s.LastOCRText)
	a.state.Mutate(func(st *state.AppState) {
		st.TypedWordsHistory[word] = struct{}{}
		st.TotalTypedCount++
		st.SuggestionIndex = nextIdx
		// Bound the history like the original (best-effort eviction).
		if len(st.TypedWordsHistory) > config.MaxTypedHistory {
			for k := range st.TypedWordsHistory {
				delete(st.TypedWordsHistory, k)
				break
			}
		}
	})
	return ""
}

// ---- alt+1 (definitions) -------------------------------------------------

func (a *App) handleAlt1Press() {
	s := a.state.Snapshot()
	if s.Region == nil {
		a.log("Cannot perform WBT: No region selected.", "ERROR")
		a.selectRegion()
		return
	}
	a.submit(a.handleAlt1Async)
}

func (a *App) handleAlt1Async() {
	s := a.state.Snapshot()
	if s.Region == nil {
		return
	}

	a.log("Processing WBT...", "INFO")
	// Wait for any running action; hold the lock only for OCR + fetch, not the popup.
	a.actionMu.Lock()
	word := a.stableOCR(*s.Region)
	ok := word != ""
	var defs []string
	if ok {
		defs = a.api.Definitions(word)
	}
	a.actionMu.Unlock()
	if !ok || word == "" {
		a.log("WBT returned no definitions.", "WARNING")
		return
	}

	a.state.Mutate(func(st *state.AppState) { st.APIStatus = a.api.Status() })

	if len(defs) > 0 {
		a.state.Mutate(func(st *state.AppState) {
			st.Definitions = defs
			st.DefinitionIndex = 0
		})
		a.log(fmt.Sprintf("Found %d definitions.", len(defs)), "INFO")
	} else {
		a.state.Mutate(func(st *state.AppState) {
			st.Definitions = nil
			st.DefinitionIndex = 0
		})
		a.log("No definitions found.", "WARNING")
	}

	a.log(fmt.Sprintf("Showing definition for: '%s'", word), "INFO")
	a.logWin.Synchronize(func() {
		ui.ShowDefinition(a.logWin.MainWindow(), word, defs)
	})
}

// ---- region selection ----------------------------------------------------

func (a *App) selectRegion() {
	a.logWin.Synchronize(func() {
		if a.overlay != nil {
			a.overlay.ShowRegion(nil, nil)
		}
		region, err := ui.SelectRegion(a.logWin.MainWindow())
		if err != nil {
			a.log("Region selection cancelled.", "WARNING")
			s := a.state.Snapshot()
			if a.overlay != nil {
				a.overlay.ShowRegion(s.Region, s.TurnRegion)
			}
			return
		}

		messageInfo(
			"Your turn region",
			"Select the box around YOUR TURN for auto mode (F1).\n\n"+
				"Press Esc in the next screen to skip — then auto mode will not wait for your turn.",
		)

		turnRegion, err := ui.SelectRegion(a.logWin.MainWindow())
		if err != nil {
			turnRegion = nil
			a.log("Turn region skipped — letters only.", "WARNING")
		}

		a.state.Mutate(func(st *state.AppState) {
			st.Region = region
			st.TurnRegion = turnRegion
		})
		if turnRegion != nil {
			a.log("Regions saved (letters + your turn).", "INFO")
		} else {
			a.log("Regions saved (letters).", "INFO")
		}
		if a.overlay != nil {
			a.overlay.ShowRegion(region, turnRegion)
		}
		a.state.SaveState()
	})
}

func (a *App) clearTurnRegion() {
	a.state.Mutate(func(st *state.AppState) { st.TurnRegion = nil })
	a.log("Turn region cleared — auto mode no longer waits for your turn.", "INFO")
	a.logWin.Synchronize(func() {
		s := a.state.Snapshot()
		if a.overlay != nil {
			a.overlay.ShowRegion(s.Region, nil)
		}
	})
	a.state.SaveState()
}

// ---- modes ----------------------------------------------------------------

func (a *App) setSearchMode(index int) {
	s := a.state.Snapshot()
	if s.CurrentModeIndex == index {
		return
	}
	a.state.Mutate(func(st *state.AppState) {
		st.CurrentModeIndex = index
		st.Suggestions = nil
		st.SuggestionIndex = 0
		st.LastOCRText = ""
	})
	a.log(fmt.Sprintf("Current Mode: %s", config.SearchModes[clampIndex(index, len(config.SearchModes))]), "INFO")
	a.state.SaveState()
}

func (a *App) setSortMode(index int) {
	s := a.state.Snapshot()
	if s.CurrentSortModeIndex == index {
		return
	}
	a.state.Mutate(func(st *state.AppState) { st.CurrentSortModeIndex = index })
	if len(s.Suggestions) > 0 {
		sorted := suggest.Sort(s.Suggestions, config.SortModes[clampIndex(index, len(config.SortModes))])
		a.state.Mutate(func(st *state.AppState) {
			st.Suggestions = sorted
			st.SuggestionIndex = 0
		})
	}
	a.log(fmt.Sprintf("Current Sort: %s", config.SortModes[clampIndex(index, len(config.SortModes))]), "INFO")
	a.state.SaveState()
}

func (a *App) setTypingDelay() {
	a.logWin.Synchronize(func() {
		s := a.state.Snapshot()
		prompt := fmt.Sprintf(
			"Typical delay between keystrokes in seconds (timing varies slightly).\n"+
				"Slower, more human-like values are often ~0.25–0.45.\n"+
				"Allowed range: %g to %g", config.TypingDelayMin, config.TypingDelayMax)
		val, ok := ui.AskFloat(a.logWin.MainWindow(), "Typing delay", prompt,
			config.TypingDelayMin, config.TypingDelayMax, round4(s.TypingDelay))
		if !ok {
			return
		}
		a.state.Mutate(func(st *state.AppState) { st.TypingDelay = val })
		a.state.SaveState()
		a.log(fmt.Sprintf("Typing delay set to %g s per character.", val), "INFO")
	})
}

func (a *App) setOCRInterval() {
	a.logWin.Synchronize(func() {
		s := a.state.Snapshot()
		prompt := fmt.Sprintf(
			"Seconds between OCR checks in auto mode (F1).\nAllowed range: %g to %g",
			config.OCRIntervalMin, config.OCRIntervalMax)
		val, ok := ui.AskFloat(a.logWin.MainWindow(), "OCR interval", prompt,
			config.OCRIntervalMin, config.OCRIntervalMax, round4(s.OCRInterval))
		if !ok {
			return
		}
		a.state.Mutate(func(st *state.AppState) { st.OCRInterval = val })
		a.state.SaveState()
		a.log(fmt.Sprintf("OCR interval set to %g s.", val), "INFO")
	})
}

// ---- history --------------------------------------------------------------

func (a *App) clearTypedHistory() {
	a.state.Mutate(func(st *state.AppState) {
		st.TypedWordsHistory = make(map[string]struct{})
		st.TypingRecords = nil
		st.TotalTypedCount = 0
	})
	a.log("Cleared history of typed words.", "INFO")
	a.state.SaveState()
}

func (a *App) undoLastWord() {
	if word := a.state.UndoLastWord(); word != "" {
		a.log(fmt.Sprintf("Undone: '%s'", word), "INFO")
	} else {
		a.log("Nothing to undo.", "WARNING")
	}
}

// ---- auto mode ------------------------------------------------------------

func (a *App) toggleAutoMode() {
	s := a.state.Snapshot()
	newState := !s.AutoModeActive
	a.state.Mutate(func(st *state.AppState) { st.AutoModeActive = newState })
	if newState {
		a.autoWatcherReset.Store(true)
		a.ocr.ClearCache()
		a.log("Auto mode ENABLED (fresh OCR scan).", "INFO")
	} else {
		a.log("Auto mode DISABLED.", "INFO")
	}
}

func (a *App) autoModeWatcher() {
	var lastWarnEmpty, lastWarnGate time.Time

	for {
		s := a.state.Snapshot()
		poll := time.Duration(s.OCRInterval * float64(time.Second))
		if !s.AutoModeActive || s.Region == nil {
			time.Sleep(poll)
			continue
		}

		if a.autoWatcherReset.Swap(false) {
			a.setLastHandled("")
		}

		// A Shift action is queued/running: let it finish instead of racing it.
		if a.pendingManual.Load() > 0 {
			time.Sleep(poll)
			continue
		}

		letters, ok := a.ocr.PerformOCR(*s.Region)
		now := time.Now()
		if !ok || letters == "" {
			if now.Sub(lastWarnEmpty) > 8*time.Second {
				a.log("Auto mode: letter OCR is empty — check the letter region (TAB).", "WARNING")
				lastWarnEmpty = now
			}
			time.Sleep(poll)
			continue
		}

		if letters != a.getLastHandled() {
			// Confirm on consecutive captures so a one-off misread is never typed.
			letters = a.stableOCR(*s.Region)
			if letters == "" || letters == a.getLastHandled() {
				time.Sleep(poll)
				continue
			}
			gateOK, turnOCR := a.autoModeTurnOK()
			if !gateOK {
				if s.TurnRegion != nil && now.Sub(lastWarnGate) > 8*time.Second {
					a.log(fmt.Sprintf("Auto mode: waiting for YOUR TURN (turn OCR: %q)", turnOCR), "WARNING")
					lastWarnGate = now
				}
				time.Sleep(poll)
				continue
			}
			// Run inline (not in a goroutine) so the watcher waits for typing to
			// finish before polling again.
			a.actionMu.Lock()
			// Re-check after waiting: Shift may have paused auto mode or already
			// typed for these letters.
			if a.state.Snapshot().AutoModeActive && a.pendingManual.Load() == 0 && letters != a.getLastHandled() {
				a.log(fmt.Sprintf("Auto-detected: '%s'", letters), "INFO")
				a.handleShiftAsync("auto", letters)
			}
			a.actionMu.Unlock()
		}

		time.Sleep(poll)
	}
}

// ---- help -----------------------------------------------------------------

func (a *App) showHelp() {
	a.logWin.Synchronize(func() {
		ui.ShowHelp(a.logWin.MainWindow(), a.helpText())
	})
}

// ---- tesseract ------------------------------------------------------------

func (a *App) checkAndInstallTesseract() bool {
	if a.ocr.Available() {
		return true
	}

	a.log("Tesseract WBT not found.", "WARNING")
	if !messageYesNo("Tesseract Not Found", "Tesseract WBT not found. Download and install?") {
		return false
	}

	a.log("Downloading Tesseract...", "INFO")
	if err := downloadFile(config.TesseractInstallerURL, config.TesseractInstallerPath); err != nil {
		a.log(fmt.Sprintf("Installation failed: %v", err), "ERROR")
		return false
	}

	a.log("Running installer...", "INFO")
	cmd := exec.Command(config.TesseractInstallerPath)
	if err := cmd.Run(); err != nil {
		a.log(fmt.Sprintf("Installation failed: %v", err), "ERROR")
		return false
	}

	// Re-resolve after installation.
	a.ocr = ocr.NewProcessor()
	return a.ocr.Available()
}

// ---- lifecycle ------------------------------------------------------------

// Run starts the application: creates the UI, installs hotkeys, launches the
// auto-mode watcher, and enters the GUI message loop.
func (a *App) Run() error {
	logging.Infof("========== WBT STARTED ==========")

	if !a.checkAndInstallTesseract() {
		a.log("Tesseract is required for OCR features.", "WARNING")
	}

	overlay, err := ui.NewRegionOverlay()
	if err != nil {
		return err
	}
	a.overlay = overlay

	logWin, err := ui.NewLogWindow(a.queue, a.callbacks(), func(vis bool) {
		if a.overlay != nil {
			a.overlay.SetBundleVisible(vis)
		}
	})
	if err != nil {
		return err
	}
	a.logWin = logWin

	a.log("========== WBT STARTED ==========", "INFO")
	for _, line := range strings.Split(a.stateText(), "\n") {
		a.log(line, "INFO")
	}

	s := a.state.Snapshot()
	if s.Region == nil {
		a.log("Press TAB to select regions", "WARNING")
	} else {
		a.overlay.ShowRegion(s.Region, s.TurnRegion)
	}

	a.registerHotkeys()
	go a.hook.Start()
	go a.autoModeWatcher()

	a.logWin.Run()
	return nil
}

func (a *App) callbacks() ui.Callbacks {
	return ui.Callbacks{
		SelectRegion:     a.selectRegion,
		ClearTurnRegion:  a.clearTurnRegion,
		SetSearchMode:    a.setSearchMode,
		SetSortMode:      a.setSortMode,
		ClearHistory:     a.clearTypedHistory,
		UndoWord:         a.undoLastWord,
		ShowHelp:         a.showHelp,
		ToggleWindow:     func() { a.logWin.ToggleVisibility() },
		FetchSuggestions: a.handleShiftPress,
		FetchDefinitions: a.handleAlt1Press,
		SetTypingDelay:   a.setTypingDelay,
		SetOCRInterval:   a.setOCRInterval,
		Exit:             func() { a.gracefulExit(0) },
	}
}

func (a *App) registerHotkeys() {
	a.hook.Register("shift", a.handleShiftPress)
	a.hook.Register("alt+1", a.handleAlt1Press)
	a.hook.Register("tab", a.selectRegion)
	a.hook.Register("page up", func() {
		s := a.state.Snapshot()
		a.setSearchMode((s.CurrentModeIndex + 1) % len(config.SearchModes))
	})
	a.hook.Register("page down", func() {
		s := a.state.Snapshot()
		a.setSortMode((s.CurrentSortModeIndex + 1) % len(config.SortModes))
	})
	a.hook.Register("delete", a.clearTypedHistory)
	a.hook.Register("caps lock", func() { a.logWin.ToggleVisibility() })
	a.hook.Register("f1", a.toggleAutoMode)
	a.hook.Register("ctrl+f2", a.clearTurnRegion)
	a.hook.Register(".", a.showHelp)
	a.hook.Register("ctrl+z", a.undoLastWord)
	a.hook.Register("ctrl+c", func() { a.gracefulExit(0) })
}

func (a *App) gracefulExit(code int) {
	a.log("Shutting down...", "INFO")
	a.state.Mutate(func(st *state.AppState) { st.AutoModeActive = false })
	a.state.SaveState()
	a.state.SaveMetrics()

	if a.hook != nil {
		a.hook.Stop()
	}
	if a.logWin != nil {
		a.logWin.Dispose()
	}
	// Give the UI a moment to tear down.
	time.Sleep(150 * time.Millisecond)
	os.Exit(code)
}

// ---- helpers --------------------------------------------------------------

func clampIndex(i, n int) int {
	if n == 0 {
		return 0
	}
	if i < 0 || i >= n {
		return ((i % n) + n) % n
	}
	return i
}

func round4(v float64) float64 {
	return float64(int64(v*10000+0.5)) / 10000
}
