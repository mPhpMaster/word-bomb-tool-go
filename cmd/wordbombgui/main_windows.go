//go:build windows

// Command wordbombgui is the desktop Word Bomb Tool: global hotkeys, screen-region
// OCR, auto-typing, overlays and a system-tray icon. It is the Go port of main.py.
package main

import (
	"fmt"
	"os"

	"github.com/mphpmaster/word-bomb-tool-go/internal/app"
	"github.com/mphpmaster/word-bomb-tool-go/internal/logging"
)

func main() {
	closer, err := logging.Setup(true)
	if err == nil && closer != nil {
		defer closer.Close()
	}

	a := app.New()
	if err := a.Run(); err != nil {
		logging.Errorf("Fatal error: %v", err)
		fmt.Fprintf(os.Stderr, "FATAL ERROR: %v\n", err)
		os.Exit(1)
	}
}
