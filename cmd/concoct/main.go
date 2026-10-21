package main

import (
	"context"
	"os"
	"os/signal"
)

const version = "0.6.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, liveOpen))
}
