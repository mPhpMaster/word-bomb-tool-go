// Package state holds the central, thread-safe application state together with
// its persistence (config + metrics files). It is the Go port of state.py.
package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
	"github.com/mphpmaster/word-bomb-tool-go/internal/logging"
)

// TypingRecord records a typed word with its timestamp and the search term used.
type TypingRecord struct {
	Word       string
	Timestamp  time.Time
	SearchTerm string
}

// Metrics holds session telemetry.
type Metrics struct {
	TotalOCRAttempts   int
	SuccessfulOCRCount int
	FailedOCRCount     int
	APIRequests        int
	SuccessfulAPICalls int
	FailedAPICalls     int
	AverageOCRTimeMS   float64
	AverageAPITimeMS   float64
	SessionStartTime   time.Time
}

// AppState is the central application state.
type AppState struct {
	Region               *config.Region
	TurnRegion           *config.Region
	Suggestions          []string
	Definitions          []string
	LastOCRText          string
	AutoModeActive       bool
	CurrentModeIndex     int
	CurrentSortModeIndex int
	SuggestionIndex      int
	DefinitionIndex      int
	TypedWordsHistory    map[string]struct{}
	TypingRecords        []TypingRecord
	TotalTypedCount      int
	TypingDelay          float64
	OCRInterval          float64
	APIStatus            string
	Metrics              Metrics
}

func newAppState() AppState {
	return AppState{
		CurrentModeIndex:     2,
		CurrentSortModeIndex: 2,
		TypedWordsHistory:    make(map[string]struct{}),
		TypingDelay:          config.TypingDelay,
		OCRInterval:          config.OCRInterval,
		APIStatus:            config.StatusOnline,
		Metrics:              Metrics{SessionStartTime: time.Now()},
	}
}

// Manager provides thread-safe access to AppState.
type Manager struct {
	mu    sync.RWMutex
	state AppState
}

// NewManager returns a Manager initialised with defaults.
func NewManager() *Manager {
	return &Manager{state: newAppState()}
}

// Snapshot returns a defensive copy of the current state. Slices and the typed
// history map are copied so callers can read them without holding the lock.
func (m *Manager) Snapshot() AppState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.copyLocked()
}

func (m *Manager) copyLocked() AppState {
	s := m.state
	s.Suggestions = append([]string(nil), m.state.Suggestions...)
	s.Definitions = append([]string(nil), m.state.Definitions...)
	s.TypingRecords = append([]TypingRecord(nil), m.state.TypingRecords...)
	s.TypedWordsHistory = make(map[string]struct{}, len(m.state.TypedWordsHistory))
	for k := range m.state.TypedWordsHistory {
		s.TypedWordsHistory[k] = struct{}{}
	}
	return s
}

// Mutate runs fn under the write lock with direct access to the live state.
func (m *Manager) Mutate(fn func(s *AppState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(&m.state)
}

// AddTypingRecord appends a typing record for word/searchTerm.
func (m *Manager) AddTypingRecord(word, searchTerm string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.TypingRecords = append(m.state.TypingRecords, TypingRecord{
		Word:       word,
		Timestamp:  time.Now(),
		SearchTerm: searchTerm,
	})
}

// UndoLastWord removes the most recent typing record, forgets the word and
// decrements the counter, returning the word (or "" if there was nothing).
func (m *Manager) UndoLastWord() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.state.TypingRecords)
	if n == 0 {
		return ""
	}
	rec := m.state.TypingRecords[n-1]
	m.state.TypingRecords = m.state.TypingRecords[:n-1]
	delete(m.state.TypedWordsHistory, rec.Word)
	if m.state.TotalTypedCount > 0 {
		m.state.TotalTypedCount--
	}
	return rec.Word
}

// RecordOCRAttempt updates OCR metrics with a running average.
func (m *Manager) RecordOCRAttempt(success bool, durationMS float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mt := &m.state.Metrics
	mt.TotalOCRAttempts++
	if success {
		mt.SuccessfulOCRCount++
	} else {
		mt.FailedOCRCount++
	}
	if mt.TotalOCRAttempts > 0 {
		total := mt.AverageOCRTimeMS * float64(mt.TotalOCRAttempts-1)
		mt.AverageOCRTimeMS = (total + durationMS) / float64(mt.TotalOCRAttempts)
	}
}

// RecordAPICall updates API metrics with a running average.
func (m *Manager) RecordAPICall(success bool, durationMS float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mt := &m.state.Metrics
	mt.APIRequests++
	if success {
		mt.SuccessfulAPICalls++
	} else {
		mt.FailedAPICalls++
	}
	if mt.APIRequests > 0 {
		total := mt.AverageAPITimeMS * float64(mt.APIRequests-1)
		mt.AverageAPITimeMS = (total + durationMS) / float64(mt.APIRequests)
	}
}

// persistedConfig is the on-disk shape of ocr_config.json (compatible with the
// original Python app).
type persistedConfig struct {
	Region            *config.Region `json:"region"`
	TurnRegion        *config.Region `json:"turn_region"`
	CurrentModeIndex  int            `json:"current_mode_index"`
	CurrentSortIndex  int            `json:"current_sort_mode_index"`
	TotalTypedCount   int            `json:"total_typed_count"`
	TypingDelay       float64        `json:"typing_delay"`
	OCRInterval       float64        `json:"ocr_interval"`
}

// SaveState writes the persisted settings to ocr_config.json.
func (m *Manager) SaveState() {
	m.mu.RLock()
	cfg := persistedConfig{
		Region:           m.state.Region,
		TurnRegion:       m.state.TurnRegion,
		CurrentModeIndex: m.state.CurrentModeIndex,
		CurrentSortIndex: m.state.CurrentSortModeIndex,
		TotalTypedCount:  m.state.TotalTypedCount,
		TypingDelay:      m.state.TypingDelay,
		OCRInterval:      m.state.OCRInterval,
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logging.Errorf("Error saving config: %v", err)
		return
	}
	if err := os.WriteFile(config.ConfigFile, data, 0o644); err != nil {
		logging.Errorf("Error saving config: %v", err)
		return
	}
	logging.Infof("Configuration saved")
}

// LoadState loads persisted settings from ocr_config.json. Missing keys keep
// their defaults; a missing file is not an error.
func (m *Manager) LoadState() {
	data, err := os.ReadFile(config.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Infof("No config file found, using defaults")
			return
		}
		logging.Errorf("Error loading config: %v", err)
		return
	}

	// Decode into a map first so we can honour "missing key keeps default"
	// semantics for the scalar fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		logging.Errorf("Error loading config: %v", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if v, ok := raw["region"]; ok {
		var r *config.Region
		if json.Unmarshal(v, &r) == nil {
			m.state.Region = r
		}
	}
	if v, ok := raw["turn_region"]; ok {
		var r *config.Region
		if json.Unmarshal(v, &r) == nil {
			m.state.TurnRegion = r
		}
	}
	if v, ok := raw["current_mode_index"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			m.state.CurrentModeIndex = n
		}
	}
	if v, ok := raw["current_sort_mode_index"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			m.state.CurrentSortModeIndex = n
		}
	}
	if v, ok := raw["total_typed_count"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			m.state.TotalTypedCount = n
		}
	}
	if v, ok := raw["typing_delay"]; ok {
		var f float64
		if json.Unmarshal(v, &f) == nil {
			m.state.TypingDelay = config.ClampTypingDelay(f)
		}
	}
	if v, ok := raw["ocr_interval"]; ok {
		var f float64
		if json.Unmarshal(v, &f) == nil {
			m.state.OCRInterval = config.ClampOCRInterval(f)
		}
	}
	logging.Infof("Configuration loaded from file")
}

// SaveMetrics writes session metrics to ocr_metrics.json.
func (m *Manager) SaveMetrics() {
	m.mu.RLock()
	mt := m.state.Metrics
	m.mu.RUnlock()

	out := map[string]interface{}{
		"total_ocr_attempts":   mt.TotalOCRAttempts,
		"successful_ocr_count": mt.SuccessfulOCRCount,
		"failed_ocr_count":     mt.FailedOCRCount,
		"api_requests":         mt.APIRequests,
		"successful_api_calls": mt.SuccessfulAPICalls,
		"failed_api_calls":     mt.FailedAPICalls,
		"average_ocr_time_ms":  mt.AverageOCRTimeMS,
		"average_api_time_ms":  mt.AverageAPITimeMS,
		"session_start_time":   mt.SessionStartTime.Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		logging.Errorf("Error saving metrics: %v", err)
		return
	}
	if err := os.WriteFile(config.MetricsFile, data, 0o644); err != nil {
		logging.Errorf("Error saving metrics: %v", err)
		return
	}
	logging.Infof("Metrics saved")
}
