package wrapper

import (
	"context"
	"fmt"
	"io"
)

func RunMain(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, version string) (int, error) {
	if len(args) == 0 || args[0] != "app-server" {
		return 2, fmt.Errorf("wrapper role only supports codex app-server mode")
	}

	cfg, err := LoadConfig(args)
	if err != nil {
		return 1, err
	}
	if version != "" {
		cfg.Version = version
	}

	app := New(cfg)
	exitCode, err := app.Run(ctx, stdin, stdout, stderr)
	if err != nil && err != context.Canceled {
		return exitCode, err
	}
	return exitCode, nil
}
