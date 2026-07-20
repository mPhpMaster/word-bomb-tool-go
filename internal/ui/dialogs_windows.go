//go:build windows

package ui

import (
	"strings"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
)

// ShowHelp displays the hotkeys/help window (modal). Must run on the GUI thread.
func ShowHelp(owner walk.Form, text string) {
	var dlg *walk.Dialog
	var closeBtn *walk.PushButton
	_, _ = (decl.Dialog{
		AssignTo:      &dlg,
		Title:         "Help & Hotkeys",
		DefaultButton: &closeBtn,
		CancelButton:  &closeBtn,
		MinSize:       decl.Size{Width: 520, Height: 470},
		Layout:        decl.VBox{},
		Children: []decl.Widget{
			decl.TextEdit{
				ReadOnly: true,
				Text:     strings.TrimSpace(text),
				VScroll:  true,
				Font:     decl.Font{Family: config.Theme.FontFamily, PointSize: config.Theme.FontSize},
			},
			decl.PushButton{
				AssignTo:  &closeBtn,
				Text:      "Close",
				OnClicked: func() { dlg.Accept() },
			},
		},
	}).Run(owner)
}

// ShowDefinition displays a word's definitions (modal). Must run on the GUI
// thread. It is a no-op when there are no definitions.
func ShowDefinition(owner walk.Form, word string, definitions []string) {
	if len(definitions) == 0 {
		return
	}
	var b strings.Builder
	for i, d := range definitions {
		b.WriteString(itoa(i+1) + ". " + strings.TrimSpace(d) + "\r\n\r\n")
	}

	var dlg *walk.Dialog
	var closeBtn *walk.PushButton
	_, _ = (decl.Dialog{
		AssignTo:      &dlg,
		Title:         "Definition of '" + word + "'",
		DefaultButton: &closeBtn,
		CancelButton:  &closeBtn,
		MinSize:       decl.Size{Width: 640, Height: 480},
		Layout:        decl.VBox{},
		Children: []decl.Widget{
			decl.TextEdit{
				ReadOnly: true,
				Text:     b.String(),
				VScroll:  true,
				Font:     decl.Font{Family: config.Theme.FontFamily, PointSize: config.Theme.DefinitionFont},
			},
			decl.PushButton{
				AssignTo:  &closeBtn,
				Text:      "Close",
				OnClicked: func() { dlg.Accept() },
			},
		},
	}).Run(owner)
}

// AskFloat prompts for a float in [min,max]. Returns the value and true on OK,
// or (0,false) if cancelled. Must run on the GUI thread.
func AskFloat(owner walk.Form, title, prompt string, min, max, initial float64) (float64, bool) {
	var dlg *walk.Dialog
	var okBtn, cancelBtn *walk.PushButton
	var edit *walk.NumberEdit
	accepted := false

	_, _ = (decl.Dialog{
		AssignTo:      &dlg,
		Title:         title,
		DefaultButton: &okBtn,
		CancelButton:  &cancelBtn,
		MinSize:       decl.Size{Width: 420, Height: 180},
		Layout:        decl.VBox{},
		Children: []decl.Widget{
			decl.Label{Text: prompt},
			decl.NumberEdit{
				AssignTo:           &edit,
				Decimals:           4,
				MinValue:           min,
				MaxValue:           max,
				Value:              initial,
				SpinButtonsVisible: true,
			},
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.HSpacer{},
					decl.PushButton{
						AssignTo: &okBtn,
						Text:     "OK",
						OnClicked: func() {
							accepted = true
							dlg.Accept()
						},
					},
					decl.PushButton{
						AssignTo:  &cancelBtn,
						Text:      "Cancel",
						OnClicked: func() { dlg.Cancel() },
					},
				},
			},
		},
	}).Run(owner)

	if !accepted {
		return 0, false
	}
	return edit.Value(), true
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	p := len(buf)
	for i > 0 {
		p--
		buf[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		buf[p] = '-'
	}
	return string(buf[p:])
}
