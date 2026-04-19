package wrapper

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	feishuMCPServerID      = "codex_remote_feishu"
	feishuMCPBearerEnvName = "CODEX_REMOTE_FEISHU_MCP_BEARER"
)

type childToolServiceInfo struct {
	URL       string `json:"url"`
	Token     string `json:"token"`
	TokenType string `json:"tokenType"`
}

func (a *App) buildCodexChildLaunch(baseArgs []string) ([]string, []string) {
	args := append([]string{}, baseArgs...)
	env := childEnvWithProxy(a.config.ChildProxyEnv, args)
	if !a.feishuMCPPublicationEligible() {
		return args, env
	}

	info, err := readChildToolServiceInfo(a.config.RuntimePaths.ToolServiceFile)
	if err != nil {
		a.debugf("feishu mcp publication skipped: read state failed path=%s err=%v", a.config.RuntimePaths.ToolServiceFile, err)
		return args, env
	}
	if strings.TrimSpace(info.URL) == "" || strings.TrimSpace(info.Token) == "" {
		a.debugf("feishu mcp publication skipped: incomplete state path=%s", a.config.RuntimePaths.ToolServiceFile)
		return args, env
	}
	if tokenType := strings.TrimSpace(info.TokenType); tokenType != "" && !strings.EqualFold(tokenType, "bearer") {
		a.debugf("feishu mcp publication skipped: unsupported token type=%s", tokenType)
		return args, env
	}

	args = append(
		args,
		"-c", codexMCPOverride("url", info.URL),
		"-c", codexMCPOverride("bearer_token_env_var", feishuMCPBearerEnvName),
	)
	env = upsertEnvValue(env, feishuMCPBearerEnvName, strings.TrimSpace(info.Token))
	return args, env
}

func (a *App) feishuMCPPublicationEligible() bool {
	return !strings.EqualFold(strings.TrimSpace(a.config.Source), "vscode")
}

func codexMCPOverride(field, value string) string {
	return fmt.Sprintf("mcp_servers.%s.%s=%s", feishuMCPServerID, strings.TrimSpace(field), strconv.Quote(strings.TrimSpace(value)))
}

func readChildToolServiceInfo(path string) (childToolServiceInfo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return childToolServiceInfo{}, fmt.Errorf("tool service state path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return childToolServiceInfo{}, err
	}
	var info childToolServiceInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return childToolServiceInfo{}, err
	}
	return info, nil
}

func upsertEnvValue(env []string, key, value string) []string {
	key = strings.TrimSpace(key)
	if key == "" {
		return append([]string{}, env...)
	}
	entry := key + "=" + value
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, item := range env {
		k, _, ok := strings.Cut(item, "=")
		if ok && k == key {
			if !replaced {
				out = append(out, entry)
				replaced = true
			}
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, entry)
	}
	return out
}
