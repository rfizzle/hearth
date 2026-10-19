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
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "login":
			os.Exit(runLogin(os.Args[2:]))
		case "logout":
			os.Exit(runLogout(os.Args[2:]))
		case "say":
			os.Exit(runSay(os.Args[2:]))
		}
	}
	for _, a := range os.Args[1:] {
		if a == "--version" {
			fmt.Println(version)
			return
		}
	}
}
