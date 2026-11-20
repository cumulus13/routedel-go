package main

// File: cmd/routedel/main.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-10-26
// Description: Entry point for routedel.
// License: MIT

import (
	"os"

	"github.com/cumulus13/routedel/internal/routedel"
)

func main() {
	os.Exit(routedel.Main(os.Args[1:]))
}
