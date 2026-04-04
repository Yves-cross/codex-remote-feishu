package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kxn/codex-remote-feishu/internal/app/wrapper"
)

func main() {
	cfg, err := wrapper.LoadConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	app := wrapper.New(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	exitCode, err := app.Run(ctx, os.Stdin, os.Stdout, os.Stderr)
	if err != nil && err != context.Canceled {
		log.Fatal(err)
	}
	os.Exit(exitCode)
}
