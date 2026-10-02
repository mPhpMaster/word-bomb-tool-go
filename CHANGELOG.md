# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-10-02

### Changed

- **Much faster from prompt to typed word** (about 0.2s instead of several
  seconds).
  - Letters are read with the OCR engine built into Windows
    (`Windows.Media.Ocr`, in-process, ~10-20ms per read) instead of launching
    `tesseract.exe` for every read (~0.5s each). Go has no WinRT projection,
    so `internal/winocr` is a small hand-written WinRT client. Short prompts
    such as `AB` or `TR` are read by placing the prompt after a known word,
    three times, and taking the majority. Tesseract is only used when Windows
    has no OCR language installed.
  - Starts With / Ends With / Contains suggestions come from a built-in offline
    word list (ENABLE, public domain, ~173k words) in a few milliseconds
    instead of a Datamuse request. Rhymes and Related Words still use Datamuse.
    The CLI's `suggest` command uses the same lists.
  - **Fast typing is the default**: no "thinking" pause, 12ms between keys and
    a short pause before Enter. The human-like timing is still available via
    Options > Fast typing (on/off), saved as `fast_typing` in
    `ocr_config.json`, and shown in the state text.
  - Checking whether the letters changed needs one read when they did not, and
    the stable-read gap dropped from 120ms to 40ms.
- A capital I read as a lowercase l is corrected when the swapped letters match
  at least ten times as many words.
- The "YOUR TURN" check also uses the Windows engine.
- The app no longer asks to install Tesseract when the Windows engine is
  available, and the check runs after the window is shown.
- Requires Windows 10 version 2004 or later (the installer checks it).

### Added

- **Arabic prompts.** Arabic letters are read with the Windows Arabic OCR
  engine (when that OCR language is installed) and answered from a built-in
  Arabic word list (common words first, checked against the Ayaspell
  dictionary; see `internal/wordlist/data/ARABIC-WORDS-NOTICE.md`). Words are
  typed as Unicode, so no Arabic keyboard layout is needed. Rhymes / Related
  Words fall back to Contains for Arabic, since Datamuse is English-only.
- **About window** (Help > About Word Bomb Tool…, also in the tray menu) with
  the version, author, links, licence and third-party notices.
- `THIRD_PARTY_NOTICES.md`, installed next to the exe together with `LICENSE`.

### Fixed

- **The region border was read as letters.** The overlay border sat on the
  region's edge and was captured with it, adding an "i"/"l". It is now drawn
  3px outside the region and excluded from screen capture, and the image
  cleanup drops thin lines and frames.
- Small text inside the region (such as a "1K" counter) and background specks
  are removed before reading; only the prompt-sized glyphs (and their dots) are
  kept.
- Readings that no word contains are no longer sent to Datamuse (which returned
  junk that was then typed); nothing is typed for them. Readings that are not
  Latin a-z or Arabic, and one-letter reads, are ignored, and accented letters
  are folded to a-z.
- **Correct words were erased while the bomb shook.** Near the end of a turn
  the reading flickers between look-alikes ("ump" -> "ume" -> "ump"); each
  flicker counted as a change of letters, so a correct word was erased and a
  new one looked up. A change now needs 3 identical reads in a row of letters
  some word contains, and any read of the original letters cancels it.
- `go vet ./...` passes again (keyboard hook pointer conversion).

## [1.0.0] - 2026-07-20

### Added

- Initial Go port of
  [mPhpMaster/word-bomb-tool](https://github.com/mPhpMaster/word-bomb-tool)
  (the original Python implementation), with full feature parity: screen-region
  OCR, Datamuse-backed word suggestions (5 search modes, 4 sort modes),
  auto-typing, global hotkeys, region overlays, system tray integration, and a
  GUI-less CLI.
- Pure-Go implementation — no CGO, no C toolchain — using Win32 syscalls
  directly for the keyboard hook, `SendInput` typing, and window management,
  and shelling out to the `tesseract` executable for OCR instead of binding
  `libtesseract`.
- Vendored dependencies (`vendor/`) so the project builds offline.
- Unit tests for the cross-platform packages (`config`, `datamuse`, `suggest`,
  and the `ocr` image preprocessing pipeline).
- `WordBombGUI.exe` (Windows, `lxn/walk` UI) and a cross-platform
  `WordBombCLI.exe`.

### Notes

- Config and metrics files (`ocr_config.json`, `ocr_metrics.json`,
  `ocr_helper.log`) stay format-compatible with the original Python version.
- The log view renders in a single text color (a plain Win32 edit control
  can't color individual lines), unlike the later C#/WPF port's per-line
  coloring.
