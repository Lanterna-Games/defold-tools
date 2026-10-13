package main

import (
	"os"

	"github.com/Lanterna-Games/defold-tools/internal/pls"
)

func main() {
	os.Exit(pls.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
