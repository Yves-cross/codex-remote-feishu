package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"fschannel/internal/adapter/feishu"
	"fschannel/internal/app/daemon"
	"fschannel/internal/config"
)

func main() {
	cfg, err := config.LoadServicesConfig()
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.FeishuUseSystemProxy {
		config.CaptureAndClearProxyEnv()
	}

	var gateway feishu.Gateway = feishu.NopGateway{}
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		gateway = feishu.NewLiveGateway(feishu.LiveGatewayConfig{
			AppID:          cfg.FeishuAppID,
			AppSecret:      cfg.FeishuAppSecret,
			TempDir:        os.TempDir(),
			UseSystemProxy: cfg.FeishuUseSystemProxy,
		})
	}

	app := daemon.New(":"+cfg.RelayPort, ":"+cfg.RelayAPIPort, gateway)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
