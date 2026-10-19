// Command hearthd is the Go core of hearth.
package main

import (
	"fmt"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	for _, a := range os.Args[1:] {
		if a == "--version" {
			fmt.Println(version)
			return
		}
	}
}
