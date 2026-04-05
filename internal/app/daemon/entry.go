package daemon

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/feishu"
	"github.com/kxn/codex-remote-feishu/internal/config"
	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

func RunMain(ctx context.Context, version string) error {
	cfg, err := config.LoadServicesConfig()
	if err != nil {
		return err
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

	paths, err := relayruntime.DefaultPaths()
	if err != nil {
		return err
	}
	lock, err := relayruntime.AcquireLock(ctx, paths.DaemonLockFile, false)
	if err != nil {
		return fmt.Errorf("acquire daemon runtime lock: %w", err)
	}
	defer lock.Release()

	startedAt := time.Now().UTC()
	identity, err := relayruntime.NewServerIdentity(version, cfg.ConfigPath, startedAt)
	if err != nil {
		return err
	}
	if err := relayruntime.WritePID(paths.PIDFile, identity.PID); err != nil {
		return err
	}
	defer os.Remove(paths.PIDFile)
	if err := relayruntime.WriteServerIdentity(paths.IdentityFile, identity); err != nil {
		return err
	}
	defer os.Remove(paths.IdentityFile)

	app := New(":"+cfg.RelayPort, ":"+cfg.RelayAPIPort, gateway, identity)
	if err := app.Run(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("run daemon: %w", err)
	}
	return nil
}
