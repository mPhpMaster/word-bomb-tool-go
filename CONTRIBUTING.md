# Contributing to Word Bomb Tool (Go edition)

Thanks for considering a contribution! This is a small hobby project, so the
process is intentionally lightweight.

## Getting set up

1. Install [Go 1.24+](https://go.dev/dl/).
2. Clone the repo and build:

   ```bash
   git clone https://github.com/mPhpMaster/word-bomb-tool-go.git
   cd word-bomb-tool-go
   go build -mod=vendor ./...
   go test -mod=vendor ./...
   ```

3. The GUI (`cmd/wordbombgui`, `internal/ui`, `internal/input`, `internal/app`)
   is Windows-only — it depends on Win32 syscalls (keyboard hooks,
   `SendInput`, `lxn/walk`). The cross-platform packages (`internal/config`,
   `internal/datamuse`, `internal/suggest`, `internal/ocr`'s preprocessing)
   build and test on any OS and are the easiest place to iterate if you're not
   on Windows.

## Project layout

See the [README's project layout section](README.md#project-layout) for
where things live. Keep platform-agnostic logic in the cross-platform
packages above so it stays unit-testable without a Windows box.

## Making changes

- Keep pull requests focused — one change per PR is easier to review.
- Run `go vet ./...` and `go test ./...` before opening a PR. Note: `go vet`
  reports two expected "possible misuse of unsafe.Pointer" false positives in
  `internal/input/hook_windows.go` (documented in the README) — that's normal,
  not a regression.
- Add or update tests for logic changes in the cross-platform packages.
- Dependencies are vendored (`vendor/`, built with `-mod=vendor`) since the
  build environment this project originated in has no network access to the
  Go module proxy. If you add a dependency, run `go mod vendor` and commit the
  result.
- If you change the config file format (`ocr_config.json`), please keep it
  backward-compatible where possible — it's shared with the original Python
  and the C#/WPF versions.

## Reporting bugs / requesting features

Open an [issue](../../issues) with:

- What you expected to happen vs. what actually happened.
- Repro steps, if applicable.
- Your Windows version and Go version (`go version`).
- Relevant excerpts from `ocr_helper.log` (written next to the executable),
  if the issue involves OCR, typing, or hotkeys.

## Code of Conduct

By participating, you agree to abide by the [Code of Conduct](CODE_OF_CONDUCT.md).
