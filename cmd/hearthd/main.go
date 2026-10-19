// Command hearthd is the Go core of hearth.
package main

import (
	"fmt"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		os.Exit(runServe(os.Args[2:]))
	}
	for _, a := range os.Args[1:] {
		if a == "--version" {
			fmt.Println(version)
			return
		}
	}
}
