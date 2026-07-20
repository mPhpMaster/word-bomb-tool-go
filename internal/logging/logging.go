// Package logging provides the application's file logger and the thread-safe,
// colored in-memory log queue consumed by the GUI. It is the Go port of
// logging_utils.py.
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
)

// Level is a log severity used by the UI-facing queue.
type Level string

const (
	Info    Level = "INFO"
	Warning Level = "WARNING"
	Error   Level = "ERROR"
)

// Logger is the package-level file logger. It is safe for concurrent use and is
// configured by Setup. Before Setup is called it discards output.
var Logger = log.New(io.Discard, "", 0)

// Setup configures Logger to write timestamped records to the rotating log file
// (and, optionally, also to stdout). It returns the underlying writer so the
// caller can close it on shutdown if desired.
func Setup(alsoStdout bool) (io.Closer, error) {
	rw, err := newRotatingWriter(config.LogFile, 5*1024*1024, 3)
	if err != nil {
		return nil, err
	}
	var w io.Writer = rw
	if alsoStdout {
		w = io.MultiWriter(rw, os.Stdout)
	}
	Logger = log.New(w, "", 0)
	return rw, nil
}

// Infof, Warnf and Errorf write leveled, timestamped records to the file log.
func Infof(format string, a ...interface{})  { writeLine("INFO", format, a...) }
func Warnf(format string, a ...interface{})  { writeLine("WARNING", format, a...) }
func Errorf(format string, a ...interface{}) { writeLine("ERROR", format, a...) }

func writeLine(level, format string, a ...interface{}) {
	ts := time.Now().Format("2006-01-02 15:04:05,000")
	Logger.Printf("%s - %s - %s", ts, level, fmt.Sprintf(format, a...))
}

// rotatingWriter is a minimal size-based rotating file writer (a small stand-in
// for Python's RotatingFileHandler). When the active file exceeds maxBytes it is
// rotated to name.1 ... name.N, dropping the oldest.
type rotatingWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	backups  int
	f        *os.File
	size     int64
}

func newRotatingWriter(path string, maxBytes int64, backups int) (*rotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &rotatingWriter{path: path, maxBytes: maxBytes, backups: backups, f: f, size: info.Size()}, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > w.maxBytes {
		w.rotate()
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) rotate() {
	_ = w.f.Close()
	// Shift name.(k) -> name.(k+1), dropping the oldest.
	for i := w.backups; i >= 1; i-- {
		src := w.path + "." + itoa(i)
		dst := w.path + "." + itoa(i+1)
		if i == w.backups {
			_ = os.Remove(src)
			continue
		}
		_ = os.Rename(src, dst)
	}
	_ = os.Rename(w.path, w.path+".1")

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err == nil {
		w.f = f
		w.size = 0
	}
}

// Close closes the underlying file.
func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }

// Entry is a formatted line ready for the GUI, with a hex color.
type Entry struct {
	Message string
	Color   string
}

// Queue is a thread-safe, bounded FIFO of colored log lines for the UI.
type Queue struct {
	mu      sync.Mutex
	items   []Entry
	maxSize int
}

// NewQueue returns a queue bounded to maxSize entries (0 uses the config default).
func NewQueue(maxSize int) *Queue {
	if maxSize <= 0 {
		maxSize = config.MaxLogQueueSize
	}
	return &Queue{maxSize: maxSize}
}

// Add appends a timestamped, colored message. Level selects the color.
func (q *Queue) Add(message string, level Level) {
	var color string
	switch level {
	case Error:
		color = config.Theme.Error
	case Warning:
		color = config.Theme.Warning
	default:
		color = config.Theme.LogFg
	}
	formatted := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message)

	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, Entry{Message: formatted, Color: color})
	if len(q.items) > q.maxSize {
		q.items = q.items[len(q.items)-q.maxSize:]
	}
}

// PopAll returns and clears every queued entry.
func (q *Queue) PopAll() []Entry {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	out := make([]Entry, len(q.items))
	copy(out, q.items)
	q.items = q.items[:0]
	return out
}

// HasMessages reports whether any entries are queued.
func (q *Queue) HasMessages() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items) > 0
}
