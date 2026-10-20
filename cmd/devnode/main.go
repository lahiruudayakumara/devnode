package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/lahiruudayakumara/devnode/internal/cli"
)

func main() {
	app := cli.New(os.Stdin, os.Stdout, os.Stderr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, os.Args[1:]); err != nil && ctx.Err() == nil {
		app.PrintError(err)
		os.Exit(1)
	}
}
