//go:build windows

package ui

import (
	"os"
	"path/filepath"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
	"github.com/mphpmaster/word-bomb-tool-go/internal/logging"
)

// About window content.
const (
	aboutAuthor      = "Mohammad Al-Safadi"
	aboutAuthorEmail = "mPhpMaster@gmail.com"
	aboutRepo        = "https://github.com/mPhpMaster/word-bomb-tool-go"
	aboutDescription = "Reads the Word Bomb prompt on your screen and types a matching word for you — in English or Arabic."
)

var aboutLinks = []struct{ label, url string }{
	{"GitHub", "https://github.com/mPhpMaster"},
	{"LinkedIn", "https://www.linkedin.com/in/mohammad-al-safadi/"},
	{"Discord", "http://discord.com/invite/BRgVPum"},
	{"Email", "mailto:mPhpMaster@gmail.com?subject=Word%20Bomb%20Tool"},
}

// About window colours, from the app theme (One Dark).
const (
	aboutBg      = "#21252b"
	aboutText    = "#abb2bf"
	aboutStrong  = "#dcdfe4"
	aboutMuted   = "#7f848e"
	aboutOutline = "#3e4452"
	aboutFont    = "Segoe UI"
)

// openURL opens a link or file with its default handler.
func openURL(target string) {
	if err := windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), windows.StringToUTF16Ptr(target),
		nil, nil, windows.SW_SHOWNORMAL); err != nil {
		logging.Warnf("Could not open %s: %v", target, err)
	}
}

// openDocument opens a document shipped next to the exe (LICENSE,
// THIRD_PARTY_NOTICES.md), or its copy in the repository when it isn't there.
func openDocument(name string) {
	path := filepath.Join(config.BaseDir, name)
	if _, err := os.Stat(path); err != nil {
		openURL(aboutRepo + "/blob/main/" + name)
		return
	}
	// Files without an associated app (LICENSE has no extension) open in Notepad.
	if err := windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), windows.StringToUTF16Ptr(path),
		nil, nil, windows.SW_SHOWNORMAL); err != nil {
		_ = windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), windows.StringToUTF16Ptr("notepad.exe"),
			windows.StringToUTF16Ptr(`"`+path+`"`), nil, windows.SW_SHOWNORMAL)
	}
}

type linkStyle int

const (
	linkOutlined linkStyle = iota // bordered button (GitHub, LinkedIn, ...)
	linkSmall                     // small muted text link (License, ...)
	linkAccent                    // full-width accent button (Close)
)

// linkWidget is a custom-drawn clickable label, since native push buttons
// ignore the dark theme colours.
func linkWidget(text string, style linkStyle, onClick func(), assign **walk.CustomWidget, tooltip string) decl.CustomWidget {
	var font *walk.Font
	switch style {
	case linkSmall:
		font, _ = walk.NewFont(aboutFont, 9, 0)
	case linkAccent:
		font, _ = walk.NewFont(aboutFont, 11, walk.FontBold)
	default:
		font, _ = walk.NewFont(aboutFont, 10, walk.FontBold)
	}
	bg, _ := walk.NewSolidColorBrush(parseHexColor(aboutBg))
	accent, _ := walk.NewSolidColorBrush(parseHexColor(config.Theme.Accent))
	outline, _ := walk.NewCosmeticPen(walk.PenSolid, parseHexColor(aboutOutline))

	size := decl.Size{Width: 32 + 8*len(text), Height: 38}
	maxSize := size
	switch style {
	case linkSmall:
		size = decl.Size{Width: 12 + 7*len(text), Height: 22}
		maxSize = size
	case linkAccent:
		// Full width: only the height is fixed.
		size = decl.Size{Width: 120, Height: 42}
		maxSize = decl.Size{Height: 42}
	}

	return decl.CustomWidget{
		AssignTo:            assign,
		MinSize:             size,
		MaxSize:             maxSize,
		ToolTipText:         tooltip,
		InvalidatesOnResize: true,
		PaintMode:           decl.PaintBuffered,
		PaintPixels: func(c *walk.Canvas, _ walk.Rectangle) error {
			if assign == nil || *assign == nil {
				return nil
			}
			b := (*assign).ClientBoundsPixels()
			_ = c.FillRectanglePixels(bg, b)
			fg := parseHexColor(aboutStrong)
			radius := walk.Size{Width: 14, Height: 14}
			switch style {
			case linkOutlined:
				_ = c.DrawRoundedRectanglePixels(outline, walk.Rectangle{X: b.X, Y: b.Y, Width: b.Width - 1, Height: b.Height - 1}, radius)
			case linkSmall:
				fg = parseHexColor(aboutMuted)
			case linkAccent:
				_ = c.FillRoundedRectanglePixels(accent, b, radius)
				fg = parseHexColor(aboutBg)
			}
			return c.DrawTextPixels(text, font, fg, b, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
		},
		OnMouseUp: func(x, y int, button walk.MouseButton) {
			if button == walk.LeftButton && onClick != nil {
				onClick()
			}
		},
	}
}

func aboutLabel(text string, pt int, bold bool, hex string) decl.Label {
	f := decl.Font{Family: aboutFont, PointSize: pt, Bold: bold}
	return decl.Label{Text: text, Font: f, TextColor: parseHexColor(hex)}
}

func centeredRow(spacing int, children ...decl.Widget) decl.Composite {
	bg := decl.SolidColorBrush{Color: parseHexColor(aboutBg)}
	all := append([]decl.Widget{decl.HSpacer{}}, children...)
	all = append(all, decl.HSpacer{})
	return decl.Composite{
		Background: bg,
		Layout:     decl.HBox{MarginsZero: true, Spacing: spacing},
		Children:   all,
	}
}

func centeredText(text string, pt int, hex string, wrapWidth int) decl.TextLabel {
	return decl.TextLabel{
		Text:          text,
		Font:          decl.Font{Family: aboutFont, PointSize: pt},
		TextColor:     parseHexColor(hex),
		TextAlignment: decl.AlignHCenterVNear,
		MinSize:       decl.Size{Width: wrapWidth},
	}
}

// ShowAbout displays the About window (modal, topmost). Must run on the GUI
// thread.
func ShowAbout(owner walk.Form) {
	var (
		dlg      *walk.Dialog
		avatarIV *walk.ImageView
		iconIV   *walk.ImageView
	)

	bgColor := hexToNRGBA(aboutBg)
	accent := hexToNRGBA(config.Theme.Accent)
	avatar, _ := walk.NewBitmapFromImageForDPI(avatarImage(84, bgColor, accent, "MA"), 96)
	appIcon, _ := walk.NewBitmapFromImageForDPI(makeTrayImage(), 96)

	links := make([]*walk.CustomWidget, len(aboutLinks)+4)
	var linkRow []decl.Widget
	for i, l := range aboutLinks {
		url := l.url
		linkRow = append(linkRow, linkWidget(l.label, linkOutlined, func() { openURL(url) }, &links[i], url))
	}
	n := len(aboutLinks)
	docRow := []decl.Widget{
		linkWidget("License", linkSmall, func() { openDocument("LICENSE") }, &links[n], "LICENSE"),
		aboutLabel("·", 9, false, aboutMuted),
		linkWidget("Third-party notices", linkSmall, func() { openDocument("THIRD_PARTY_NOTICES.md") }, &links[n+1], "THIRD_PARTY_NOTICES.md"),
		aboutLabel("·", 9, false, aboutMuted),
		linkWidget("Source code", linkSmall, func() { openURL(aboutRepo) }, &links[n+2], aboutRepo),
	}

	err := decl.Dialog{
		AssignTo:   &dlg,
		Title:      "About " + config.AppName,
		Background: decl.SolidColorBrush{Color: parseHexColor(aboutBg)},
		Font:       decl.Font{Family: aboutFont, PointSize: 10},
		MinSize:    decl.Size{Width: 500},
		FixedSize:  true,
		Layout:     decl.VBox{Margins: decl.Margins{Left: 36, Top: 26, Right: 36, Bottom: 26}, Spacing: 10},
		Children: []decl.Widget{
			decl.TextLabel{Text: "About", Font: decl.Font{Family: aboutFont, PointSize: 17, Bold: true},
				TextColor: parseHexColor(aboutStrong), TextAlignment: decl.AlignHCenterVNear},
			centeredRow(14,
				decl.ImageView{AssignTo: &avatarIV, Image: avatar, Mode: decl.ImageViewModeIdeal, ToolTipText: aboutAuthor,
					MinSize: decl.Size{Width: 84, Height: 84}, MaxSize: decl.Size{Width: 84, Height: 84}},
				decl.ImageView{AssignTo: &iconIV, Image: appIcon, Mode: decl.ImageViewModeIdeal,
					MinSize: decl.Size{Width: 64, Height: 64}, MaxSize: decl.Size{Width: 64, Height: 64}},
			),
			centeredRow(6,
				aboutLabel(config.AppName, 14, true, aboutStrong),
				aboutLabel("v"+config.Version, 10, false, aboutMuted),
			),
			centeredText(aboutDescription, 10, aboutMuted, 400),
			centeredRow(0,
				aboutLabel("Made by ", 10, false, aboutText),
				aboutLabel(aboutAuthor, 10, true, aboutStrong),
			),
			centeredText(aboutAuthorEmail, 10, aboutMuted, 0),
			decl.VSpacer{Size: 4},
			centeredRow(10, linkRow...),
			centeredRow(2, docRow...),
			centeredText("Copyright © 2026 "+aboutAuthor+"\r\nLicensed under the MIT License.\r\nThis program comes with ABSOLUTELY NO WARRANTY.", 8, aboutMuted, 0),
			decl.VSpacer{Size: 6},
			linkWidget("Close", linkAccent, func() { dlg.Accept() }, &links[n+3], ""),
		},
	}.Create(owner)
	if err != nil {
		logging.Errorf("About window: %v", err)
		return
	}

	for _, w := range links {
		if w != nil {
			w.SetCursor(walk.CursorHand())
		}
	}
	// Esc closes the window. (A dialog's cancel button would need to be a
	// visible native button, which ignores the dark theme.)
	esc := walk.NewAction()
	_ = esc.SetShortcut(walk.Shortcut{Key: walk.KeyEscape})
	esc.Triggered().Attach(func() { dlg.Cancel() })
	_ = dlg.ShortcutActions().Add(esc)
	if ic, err := walk.NewIconFromImage(makeTrayImage()); err == nil {
		_ = dlg.SetIcon(ic)
	}
	setTopmost(dlg.Handle())
	dlg.Run()
}
