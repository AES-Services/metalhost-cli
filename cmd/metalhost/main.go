package main

import (
	"fmt"
	"os"

	"github.com/AES-Services/metalhost-cli/internal/command"
)

func main() {
	if err := command.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
