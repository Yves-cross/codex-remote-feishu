package daemon

import (
	"errors"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/config"
)

func TestBuildStartupAccessPlanUsesSSHSetupExposure(t *testing.T) {
	cfg := config.DefaultAppConfig()
	services := config.ServicesConfig{
		RelayHost:    "127.0.0.1",
		RelayPort:    "9500",
		RelayAPIHost: "127.0.0.1",
		RelayAPIPort: "9501",
	}
	plan := buildStartupAccessPlan(cfg, services, map[string]string{
		"SSH_CONNECTION": "198.51.100.10 55000 10.0.0.8 22",
	})

	if !plan.SetupRequired {
		t.Fatal("expected setup required")
	}
	if !plan.SSHSession {
		t.Fatal("expected ssh session")
	}
	if plan.AdminBindHost != "0.0.0.0" {
		t.Fatalf("admin bind host = %q, want 0.0.0.0", plan.AdminBindHost)
	}
	if plan.SetupURL != "http://10.0.0.8:9501/setup" {
		t.Fatalf("setup url = %q", plan.SetupURL)
	}
}

func TestBuildStartupAccessPlanUsesLocalhostForLocalSetup(t *testing.T) {
	cfg := config.DefaultAppConfig()
	services := config.ServicesConfig{
		RelayHost:    "127.0.0.1",
		RelayPort:    "9500",
		RelayAPIHost: "127.0.0.1",
		RelayAPIPort: "9501",
	}
	plan := buildStartupAccessPlan(cfg, services, map[string]string{
		"DISPLAY": ":0",
	})

	if !plan.SetupRequired {
		t.Fatal("expected setup required")
	}
	if plan.SSHSession {
		t.Fatal("did not expect ssh session")
	}
	if plan.SetupURL != "http://localhost:9501/setup" {
		t.Fatalf("setup url = %q", plan.SetupURL)
	}
}

func TestBuildStartupAccessPlanTreatsSavedCredentialsAsConfigured(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Feishu.Apps = []config.FeishuAppConfig{{
		ID:        "main",
		Name:      "Main",
		AppID:     "cli_xxx",
		AppSecret: "secret_xxx",
	}}
	services := config.ServicesConfig{
		RelayHost:    "127.0.0.1",
		RelayPort:    "9500",
		RelayAPIHost: "127.0.0.1",
		RelayAPIPort: "9501",
	}
	plan := buildStartupAccessPlan(cfg, services, map[string]string{})

	if plan.SetupRequired {
		t.Fatal("did not expect setup required")
	}
	if plan.ConfiguredAppCount != 1 {
		t.Fatalf("configured app count = %d, want 1", plan.ConfiguredAppCount)
	}
	if plan.AdminBindHost != "127.0.0.1" {
		t.Fatalf("admin bind host = %q, want 127.0.0.1", plan.AdminBindHost)
	}
}

func TestMaybeOpenSetupBrowserHonorsModeAndFlag(t *testing.T) {
	original := browserOpener
	defer func() { browserOpener = original }()

	called := 0
	browserOpener = func(url string, env map[string]string) error {
		called++
		if url != "http://localhost:9501/setup" {
			t.Fatalf("browser url = %q", url)
		}
		return nil
	}

	err := maybeOpenSetupBrowser(startupAccessPlan{
		SetupRequired:   true,
		AutoOpenBrowser: true,
		SetupURL:        "http://localhost:9501/setup",
	}, map[string]string{})
	if err != nil {
		t.Fatalf("maybeOpenSetupBrowser: %v", err)
	}
	if called != 1 {
		t.Fatalf("browser called = %d, want 1", called)
	}

	if err := maybeOpenSetupBrowser(startupAccessPlan{
		SetupRequired:   true,
		AutoOpenBrowser: false,
		SetupURL:        "http://localhost:9501/setup",
	}, map[string]string{}); err != nil {
		t.Fatalf("maybeOpenSetupBrowser(disabled): %v", err)
	}
	if called != 1 {
		t.Fatalf("browser called after disabled = %d, want 1", called)
	}

	browserOpener = func(string, map[string]string) error {
		return errors.New("unexpected")
	}
	if err := maybeOpenSetupBrowser(startupAccessPlan{
		SetupRequired:   true,
		AutoOpenBrowser: true,
		SSHSession:      true,
		SetupURL:        "http://localhost:9501/setup",
	}, map[string]string{}); err != nil {
		t.Fatalf("maybeOpenSetupBrowser(ssh): %v", err)
	}
}
