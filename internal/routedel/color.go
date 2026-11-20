package routedel

// File: internal/routedel/color.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-10-26
// Description: ANSI color helpers matching Rich console and richcolorlog styling.
// License: MIT

import (
	"os"

	"golang.org/x/term"
)

const (
	ansiReset       = "\x1b[0m"
	ansiBold        = "\x1b[1m"
	ansiDim         = "\x1b[2m"
	ansiRed         = "\x1b[31m"
	ansiGreen       = "\x1b[32m"
	ansiYellow      = "\x1b[33m"
	ansiBlue        = "\x1b[34m"
	ansiMagenta     = "\x1b[35m"
	ansiCyan        = "\x1b[36m"
	ansiWhite       = "\x1b[37m"
	ansiBoldRed     = "\x1b[1;31m"
	ansiBoldGreen   = "\x1b[1;32m"
	ansiBoldYellow  = "\x1b[1;33m"
	ansiBoldBlue    = "\x1b[1;34m"
	ansiBoldMagenta = "\x1b[1;35m"
	ansiBoldCyan    = "\x1b[1;36m"
	ansiBoldWhite   = "\x1b[1;37m"
)

func useColor(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	return term.IsTerminal(int(f.Fd()))
}

func styleStdout(code string, text string) string {
	if !useColor(os.Stdout) {
		return text
	}
	return code + text + ansiReset
}

func styleStderr(code string, text string) string {
	if !useColor(os.Stderr) {
		return text
	}
	return code + text + ansiReset
}
