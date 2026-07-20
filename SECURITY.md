# Security Policy

## Supported versions

Only the latest release is supported with security fixes.

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities. Instead,
report it privately via GitHub's
[private vulnerability reporting](https://github.com/mPhpMaster/word-bomb-tool-go/security/advisories/new)
(Security tab → "Report a vulnerability"), or by contacting the maintainer
directly.

Please include:

- A description of the vulnerability and its potential impact.
- Steps to reproduce (or a proof of concept).
- The version/commit you tested against.

You should expect an initial response within a few days. This is a
small hobby project maintained in spare time, so please be patient.

## Scope notes

This application installs a global low-level keyboard hook and uses
`SendInput` to type on the user's behalf — that's the core feature (auto-typing
game suggestions), not a vulnerability by itself, but please do flag anything
that lets a remote party (e.g. a malicious API response) influence what gets
typed or trigger unintended keystrokes/actions beyond what the OCR'd game
text should allow.
