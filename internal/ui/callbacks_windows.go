//go:build windows

package ui

// Callbacks bundles every action the GUI can invoke, supplied by the app layer.
type Callbacks struct {
	SelectRegion     func()
	ClearTurnRegion  func()
	SetSearchMode    func(index int)
	SetSortMode      func(index int)
	ClearHistory     func()
	UndoWord         func()
	ShowHelp         func()
	ToggleWindow     func()
	FetchSuggestions func()
	FetchDefinitions func()
	SetTypingDelay   func()
	SetOCRInterval   func()
	Exit             func()
}
