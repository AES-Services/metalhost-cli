package main

import (
	"os"

	"github.com/AES-Services/metalhost-cli/internal/command"
)

func main() {
	if err := command.NewRootCommand().Execute(); err != nil {
		os.Exit(command.HandleError(os.Stderr, err))
	}
}
