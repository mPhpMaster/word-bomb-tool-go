//go:build !windows

// Package winocr reads text with the OCR engine built into Windows. On other
// platforms no engine is available.
package winocr

import (
	"errors"
	"image"
)

var errUnsupported = errors.New("windows OCR is only available on Windows")

// Available reports whether an OCR engine for the language prefix is installed.
func Available(lang string) bool { return false }

// LanguageTag returns the recognizer language used for the prefix, or "".
func LanguageTag(lang string) string { return "" }

// InitError returns why WinRT OCR could not start.
func InitError() error { return errUnsupported }

// Recognize always fails on this platform.
func Recognize(img *image.Gray, lang string) (string, error) { return "", errUnsupported }
