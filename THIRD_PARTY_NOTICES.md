# Third-party notices

Word Bomb Tool (Go edition) is licensed under the MIT License (see `LICENSE`).
It includes or uses the following third-party components.

## Bundled data

- **ENABLE word list** (`internal/wordlist/data/enable1.txt.gz`) — the English
  word list. Public domain.
- **Arabic word list** (`internal/wordlist/data/arabic-words.txt.gz`) — adapted
  from the Arabic list of
  [FrequencyWords](https://github.com/hermitdave/FrequencyWords) by Hermit Dave
  (built from OpenSubtitles 2018) and filtered with the
  [Ayaspell](http://ayaspell.sourceforge.net/) Hunspell dictionary. Licensed
  under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/); this
  licence covers that file only. Details in
  `internal/wordlist/data/ARABIC-WORDS-NOTICE.md`.
- **Go Bold font** (`golang.org/x/image/font/gofont/gobold`, used to draw the
  OCR anchor word and the About window's fallback initials) — by Bigelow &
  Holmes for the Go project, under the Go BSD 3-Clause License below.

## Go modules (vendored in `vendor/`, compiled into the executables)

Each module's full licence text is in its folder under `vendor/`.

| Module | Licence | Copyright |
| --- | --- | --- |
| [github.com/lxn/walk](https://github.com/lxn/walk) | BSD 3-Clause | The Walk Authors |
| [github.com/lxn/win](https://github.com/lxn/win) | BSD 3-Clause | The win Authors |
| [github.com/kbinani/screenshot](https://github.com/kbinani/screenshot) | MIT | kbinani |
| [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) | BSD 3-Clause | The Go Authors |
| [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) | BSD 3-Clause | The Go Authors |
| [golang.org/x/text](https://pkg.go.dev/golang.org/x/text) | BSD 3-Clause | The Go Authors |
| [github.com/Knetic/govaluate](https://github.com/Knetic/govaluate) | MIT | George Lester |

Vendored only as dependencies of `kbinani/screenshot` on Linux/BSD (not compiled
into the Windows executables):

| Module | Licence | Copyright |
| --- | --- | --- |
| [github.com/gen2brain/shm](https://github.com/gen2brain/shm) | BSD 2-Clause | Milan Nikolic |
| [github.com/godbus/dbus/v5](https://github.com/godbus/dbus) | BSD 2-Clause | Georg Reinke, Google |
| [github.com/jezek/xgb](https://github.com/jezek/xgb) | BSD 3-Clause | The XGB Authors |

The Go standard library and runtime compiled into the executables are under the
Go BSD 3-Clause License, © The Go Authors.

## Services and optional tools

- **[Datamuse API](https://www.datamuse.com/api/)** — used for Rhymes / Related
  Words suggestions and definitions. Subject to Datamuse's terms of use.
- **Windows OCR** (`Windows.Media.Ocr`) — part of Windows; used through the
  Windows Runtime API.
- **[Tesseract OCR](https://github.com/tesseract-ocr/tesseract)** — Apache
  License 2.0. Not bundled; only used (and only offered for download) when
  Windows has no OCR language installed.
