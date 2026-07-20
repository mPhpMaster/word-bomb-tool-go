# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
