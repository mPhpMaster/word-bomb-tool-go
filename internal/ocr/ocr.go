// Package ocr captures screen regions and runs Tesseract on them. It is the Go
// port of ocr_processor.py. Rather than binding libtesseract, it shells out to
// the tesseract executable (found on PATH or at the default install location),
// so no CGO is required.
package ocr

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"image"
	"image/png"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/kbinani/screenshot"
	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
	"github.com/mphpmaster/word-bomb-tool-go/internal/logging"
)

const letterWhitelist = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

type cacheEntry struct {
	text string
	when time.Time
}

// Processor performs OCR with a short-lived result cache.
type Processor struct {
	mu            sync.Mutex
	cache         map[string]cacheEntry
	tesseractPath string
}

// NewProcessor creates a Processor and resolves the tesseract executable path.
func NewProcessor() *Processor {
	return &Processor{
		cache:         make(map[string]cacheEntry),
		tesseractPath: FindTesseractPath(),
	}
}

// TesseractPath returns the resolved tesseract executable path ("" if not found).
func (p *Processor) TesseractPath() string { return p.tesseractPath }

// FindTesseractPath looks for tesseract on PATH, then at the default Windows
// install location.
func FindTesseractPath() string {
	if path, err := exec.LookPath("tesseract"); err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		def := `C:\Program Files\Tesseract-OCR\tesseract.exe`
		if fileExists(def) {
			return def
		}
	}
	return ""
}

// Available reports whether tesseract can be invoked.
func (p *Processor) Available() bool {
	if p.tesseractPath == "" {
		p.tesseractPath = FindTesseractPath()
	}
	if p.tesseractPath == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, p.tesseractPath, "--version").Run() == nil
}

// ClearCache empties the OCR result cache.
func (p *Processor) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = make(map[string]cacheEntry)
	logging.Infof("WBT cache cleared")
}

// PerformOCR captures the region, runs the harsh (letters-only) pipeline and
// returns lowercase letters. ok is false when nothing was recognised.
func (p *Processor) PerformOCR(region config.Region) (letters string, ok bool) {
	start := time.Now()

	img, err := capture(region)
	if err != nil {
		logging.Errorf("WBT Error: %v", err)
		return "", false
	}

	hash := hashImage(img)
	if cached, found := p.cacheGet(hash); found {
		return cached, cached != ""
	}

	pre := preprocessLetters(img)
	raw, err := p.runTesseract(pre, "--psm", "7", "-c", "tessedit_char_whitelist="+letterWhitelist)
	if err != nil {
		logging.Errorf("WBT Error: %v", err)
		return "", false
	}

	letters = keepLetters(raw)
	if letters != "" {
		p.cachePut(hash, letters)
	}
	logging.Infof("WBT completed in %.2fms", float64(time.Since(start).Microseconds())/1000.0)
	return letters, letters != ""
}

// PerformOCRTurnGate captures the region and returns lowercase alphanumerics for
// auto-mode "YOUR TURN" detection, trying the soft pipeline with several PSMs
// and falling back to the harsh pipeline.
func (p *Processor) PerformOCRTurnGate(region config.Region) string {
	img, err := capture(region)
	if err != nil {
		logging.Errorf("Turn gate WBT error: %v", err)
		return ""
	}

	run := func(g *image.Gray, psm string) string {
		raw, err := p.runTesseract(g, "--psm", psm)
		if err != nil {
			return ""
		}
		return keepAlnum(raw)
	}

	// 1) Soft path (colored buttons + white text).
	soft := preprocessTurnGate(img)
	best := ""
	for _, psm := range []string{"6", "7", "8", "13"} {
		if t := run(soft, psm); len(t) > len(best) {
			best = t
		}
	}
	if best != "" {
		return best
	}

	// 2) Harsh binarization fallback.
	hard := preprocessLetters(img)
	for _, psm := range []string{"7", "6", "8"} {
		if t := run(hard, psm); len(t) > len(best) {
			best = t
		}
	}
	return best
}

func (p *Processor) cacheGet(hash string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.cache[hash]
	if !ok {
		return "", false
	}
	if time.Since(e.when) < config.CacheExpiryMinutes*time.Minute {
		return e.text, true
	}
	delete(p.cache, hash)
	return "", false
}

func (p *Processor) cachePut(hash, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[hash] = cacheEntry{text: text, when: time.Now()}
}

// runTesseract pipes a PNG of img to `tesseract stdin stdout <args...>` and
// returns the recognised text.
func (p *Processor) runTesseract(img *image.Gray, args ...string) (string, error) {
	if p.tesseractPath == "" {
		p.tesseractPath = FindTesseractPath()
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmdArgs := append([]string{"stdin", "stdout"}, args...)
	cmd := exec.CommandContext(ctx, p.tesseractPath, cmdArgs...)
	cmd.Stdin = &buf
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func capture(region config.Region) (*image.RGBA, error) {
	rect := image.Rect(region.Left, region.Top, region.Left+region.Width, region.Top+region.Height)
	return screenshot.CaptureRect(rect)
}

func hashImage(img *image.RGBA) string {
	sum := md5.Sum(img.Pix)
	return hex.EncodeToString(sum[:])
}

func keepLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func keepAlnum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
