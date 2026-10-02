package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
)

func withConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ocr_config.json")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := config.ConfigFile
	config.ConfigFile = path
	t.Cleanup(func() { config.ConfigFile = old })
	return path
}

func TestFastTypingDefaultsToOn(t *testing.T) {
	if !NewManager().Snapshot().FastTyping {
		t.Fatal("fast typing must be on by default")
	}
	// Configs written before fast typing existed have no fast_typing key.
	withConfigFile(t, `{"typing_delay": 0.3}`)
	m := NewManager()
	m.LoadState()
	if !m.Snapshot().FastTyping {
		t.Fatal("missing fast_typing must keep the default (on)")
	}
}

func TestFastTypingIsPersisted(t *testing.T) {
	withConfigFile(t, "")
	m := NewManager()
	m.Mutate(func(s *AppState) { s.FastTyping = false })
	m.SaveState()

	loaded := NewManager()
	loaded.LoadState()
	if loaded.Snapshot().FastTyping {
		t.Fatal("fast_typing=false was not restored")
	}
}
