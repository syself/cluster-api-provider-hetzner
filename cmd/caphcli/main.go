// Package main implements the caphcli command-line entrypoint.
package main

import (
	"os"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
