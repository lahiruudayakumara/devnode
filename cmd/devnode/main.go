package main

import (
	"context"
	"os"

	"github.com/lahiruudayakumara/devnode/internal/cli"
)

func main() {
	app := cli.New(os.Stdin, os.Stdout, os.Stderr)
	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		app.PrintError(err)
		os.Exit(1)
	}
}
