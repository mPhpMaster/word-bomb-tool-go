// Package config holds application-wide constants, tunables, file paths and the
// UI theme. It is the Go port of the original config.py.
package config

import (
	"os"
	"path/filepath"
)

// Region describes a screen rectangle to capture, mirroring the Python dict
// with "left", "top", "width", "height" keys (JSON tags match so persisted
// config files are compatible with the original app).
type Region struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// baseDir returns the directory used for config, logs and bundled data. When
// running as a built executable this is the directory next to the binary,
// matching the PyInstaller "frozen" behaviour of the original.
func baseDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// BaseDir is the resolved application base directory.
var BaseDir = baseDir()

// File paths, written alongside the executable (compatible with the Python app).
var (
	ConfigFile             = filepath.Join(BaseDir, "ocr_config.json")
	LogFile                = filepath.Join(BaseDir, "ocr_helper.log")
	MetricsFile            = filepath.Join(BaseDir, "ocr_metrics.json")
	TesseractInstallerPath = filepath.Join(BaseDir, "tesseract_installer.exe")
)

// Theme mirrors the THEME dict from config.py. Colors are hex strings; the GUI
// layer converts them to native colors as needed.
var Theme = struct {
	Bg              string
	Fg              string
	LogBg           string
	LogFg           string
	SelectBg        string
	Accent          string
	Error           string
	Success         string
	Warning         string
	FontFamily      string
	DefinitionFont  int
	FontSize        int
	FontSizeSmall   int
	FocusedAlpha    float64
	UnfocusedAlpha  float64
}{
	Bg:             "#282c34",
	Fg:             "#abb2bf",
	LogBg:          "#21252b",
	LogFg:          "#98c379",
	SelectBg:       "#3e4452",
	Accent:         "#61afef",
	Error:          "#e06c75",
	Success:        "#98c379",
	Warning:        "#e5c07b",
	FontFamily:     "Consolas",
	DefinitionFont: 14,
	FontSize:       10,
	FontSizeSmall:  9,
	FocusedAlpha:   1.0,
	UnfocusedAlpha: 0.85,
}

// OCR / typing tunables.
const (
	OCRInterval    = 0.5
	OCRIntervalMin = 0.1
	OCRIntervalMax = 10.0
	OCRTimeout     = 1 // seconds; also used as the Datamuse HTTP timeout, as in the original

	// TypingDelay is the default typical seconds between keystrokes.
	TypingDelay    = 0.28
	TypingDelayMin = 0.01
	TypingDelayMax = 2.0
)

// Auto-mode turn gate: the turn_region OCR (letters+digits, lowercased) must
// contain both of these for "YOUR TURN".
const (
	TurnGateNeedYour = "your"
	TurnGateNeedTurn = "turn"
)

// ClampOCRInterval clamps an OCR poll interval; invalid values fall back to the
// default OCRInterval.
func ClampOCRInterval(v float64) float64 {
	if v != v { // NaN
		return OCRInterval
	}
	if v < OCRIntervalMin {
		return OCRIntervalMin
	}
	if v > OCRIntervalMax {
		return OCRIntervalMax
	}
	return v
}

// ClampTypingDelay clamps a typing delay; invalid values fall back to the
// default TypingDelay.
func ClampTypingDelay(v float64) float64 {
	if v != v { // NaN
		return TypingDelay
	}
	if v < TypingDelayMin {
		return TypingDelayMin
	}
	if v > TypingDelayMax {
		return TypingDelayMax
	}
	return v
}

// API settings.
const DatamuseAPI = "https://api.datamuse.com/words"

// Cache / history settings.
const (
	CacheExpiryMinutes   = 5
	MaxSuggestionsDisplay = 50
	MaxTypedHistory       = 1000
	UndoBufferSize        = 20
)

// Threading.
const MaxWorkerThreads = 2

// SearchModes and SortModes list the available modes in display order. Indices
// are persisted in the config file, so order must stay stable.
var (
	SearchModes = []string{"Starts With", "Ends With", "Contains", "Rhymes", "Related Words"}
	SortModes   = []string{"Shortest", "Longest", "Random", "Frequency"}
)

// Tesseract installer URL (Windows x64 5.5).
const TesseractInstallerURL = "https://github.com/tesseract-ocr/tesseract/releases/download/5.5.0/tesseract-ocr-w64-setup-5.5.0.20241111.exe"

// Logging.
const (
	LogLevel        = "INFO"
	MaxLogQueueSize = 500
)

// API status indicators (ASCII-safe for the Windows console).
const (
	StatusOnline  = "[OK] Online"
	StatusOffline = "[XX] Offline"
	StatusTimeout = "[--] Timeout"
	StatusError   = "[!!] Error"
)
