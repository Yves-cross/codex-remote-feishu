package wrapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

func TestBuildCodexChildLaunchAddsFeishuMCPForHeadless(t *testing.T) {
	statePath := writeToolServiceState(t, `{
  "url": "http://127.0.0.1:9702",
  "token": "secret-token",
  "tokenType": "bearer"
}`)
	app := New(Config{
		Source:       "headless",
		RuntimePaths: relayruntime.Paths{ToolServiceFile: statePath},
	})

	args, env := app.buildCodexChildLaunch([]string{"app-server", "-c", `model="gpt-5"`})

	if len(args) != 7 {
		t.Fatalf("expected base args plus MCP overrides, got %d args: %#v", len(args), args)
	}
	if args[0] != "app-server" || args[1] != "-c" || args[2] != `model="gpt-5"` {
		t.Fatalf("expected base args to stay intact, got %#v", args[:3])
	}
	if args[3] != "-c" || args[4] != `mcp_servers.codex_remote_feishu.url="http://127.0.0.1:9702"` {
		t.Fatalf("unexpected url override args: %#v", args[3:5])
	}
	if args[5] != "-c" || args[6] != `mcp_servers.codex_remote_feishu.bearer_token_env_var="CODEX_REMOTE_FEISHU_MCP_BEARER"` {
		t.Fatalf("unexpected bearer override args: %#v", args[5:7])
	}
	if got := lookupEnv(env, feishuMCPBearerEnvName); got != "secret-token" {
		t.Fatalf("expected injected bearer env, got %q", got)
	}
}

func TestBuildCodexChildLaunchSkipsFeishuMCPForVSCodeSource(t *testing.T) {
	statePath := writeToolServiceState(t, `{
  "url": "http://127.0.0.1:9702",
  "token": "secret-token",
  "tokenType": "bearer"
}`)
	app := New(Config{
		Source:       "vscode",
		RuntimePaths: relayruntime.Paths{ToolServiceFile: statePath},
	})

	args, env := app.buildCodexChildLaunch([]string{"app-server"})

	if len(args) != 1 || args[0] != "app-server" {
		t.Fatalf("expected args to remain unchanged, got %#v", args)
	}
	if got := lookupEnv(env, feishuMCPBearerEnvName); got != "" {
		t.Fatalf("expected no injected bearer env for vscode source, got %q", got)
	}
}

func TestBuildCodexChildLaunchSkipsFeishuMCPWhenStateMissing(t *testing.T) {
	app := New(Config{
		Source:       "headless",
		RuntimePaths: relayruntime.Paths{ToolServiceFile: filepath.Join(t.TempDir(), "missing.json")},
	})

	args, env := app.buildCodexChildLaunch([]string{"app-server"})

	if len(args) != 1 || args[0] != "app-server" {
		t.Fatalf("expected args to remain unchanged, got %#v", args)
	}
	if got := lookupEnv(env, feishuMCPBearerEnvName); got != "" {
		t.Fatalf("expected no injected bearer env when state is missing, got %q", got)
	}
}

func TestBuildCodexChildLaunchSkipsFeishuMCPForUnsupportedTokenType(t *testing.T) {
	statePath := writeToolServiceState(t, `{
  "url": "http://127.0.0.1:9702",
  "token": "secret-token",
  "tokenType": "basic"
}`)
	app := New(Config{
		Source:       "headless",
		RuntimePaths: relayruntime.Paths{ToolServiceFile: statePath},
	})

	args, env := app.buildCodexChildLaunch([]string{"app-server"})

	if len(args) != 1 || args[0] != "app-server" {
		t.Fatalf("expected args to remain unchanged, got %#v", args)
	}
	if got := lookupEnv(env, feishuMCPBearerEnvName); got != "" {
		t.Fatalf("expected no injected bearer env for unsupported token type, got %q", got)
	}
}

func writeToolServiceState(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tool-service.json")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(raw)+"\n"), 0o600); err != nil {
		t.Fatalf("write tool service state: %v", err)
	}
	return path
}

func lookupEnv(env []string, key string) string {
	for _, item := range env {
		k, v, ok := strings.Cut(item, "=")
		if ok && k == key {
			return v
		}
	}
	return ""
}
