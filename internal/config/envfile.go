package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type WrapperConfig struct {
	RelayServerURL  string
	CodexRealBinary string
	NameMode        string
	IntegrationMode string
	ConfigPath      string
}

type ServicesConfig struct {
	RelayPort            string
	RelayAPIPort         string
	FeishuAppID          string
	FeishuAppSecret      string
	FeishuUseSystemProxy bool
	ConfigPath           string
}

func LoadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid env line: %q", line)
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values, scanner.Err()
}

func WriteEnvFile(path string, values map[string]string) error {
	builder := strings.Builder{}
	keys := []string{
		"RELAY_SERVER_URL",
		"CODEX_REAL_BINARY",
		"CODEX_RELAY_WRAPPER_NAME_MODE",
		"CODEX_RELAY_WRAPPER_INTEGRATION_MODE",
		"RELAY_PORT",
		"RELAY_API_PORT",
		"FEISHU_APP_ID",
		"FEISHU_APP_SECRET",
		"FEISHU_USE_SYSTEM_PROXY",
	}
	written := map[string]bool{}
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(value)
		builder.WriteString("\n")
		written[key] = true
	}
	for key, value := range values {
		if written[key] {
			continue
		}
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(value)
		builder.WriteString("\n")
	}
	return os.WriteFile(path, []byte(builder.String()), 0o600)
}

func LoadWrapperConfig() (WrapperConfig, error) {
	configPath := firstEnv(
		os.Getenv("CODEX_RELAY_WRAPPER_CONFIG"),
		xdgConfigPath("codex-relay", "wrapper.env"),
		filepath.Join(mustGetwd(), ".env"),
	)
	values, err := loadOptionalEnv(configPath)
	if err != nil {
		return WrapperConfig{}, err
	}
	cfg := WrapperConfig{
		RelayServerURL:  chooseNonEmpty(os.Getenv("RELAY_SERVER_URL"), values["RELAY_SERVER_URL"], "ws://127.0.0.1:9500/ws/agent"),
		CodexRealBinary: chooseNonEmpty(os.Getenv("CODEX_REAL_BINARY"), values["CODEX_REAL_BINARY"], "codex"),
		NameMode: chooseNonEmpty(
			os.Getenv("CODEX_RELAY_WRAPPER_NAME_MODE"),
			values["CODEX_RELAY_WRAPPER_NAME_MODE"],
			"workspace_basename",
		),
		IntegrationMode: chooseNonEmpty(
			os.Getenv("CODEX_RELAY_WRAPPER_INTEGRATION_MODE"),
			values["CODEX_RELAY_WRAPPER_INTEGRATION_MODE"],
			"editor_settings",
		),
		ConfigPath: configPath,
	}
	return cfg, nil
}

func LoadServicesConfig() (ServicesConfig, error) {
	configPath := firstEnv(
		os.Getenv("CODEX_RELAY_SERVICES_CONFIG"),
		xdgConfigPath("codex-relay", "services.env"),
		filepath.Join(mustGetwd(), ".env"),
	)
	values, err := loadOptionalEnv(configPath)
	if err != nil {
		return ServicesConfig{}, err
	}
	cfg := ServicesConfig{
		RelayPort:    chooseNonEmpty(os.Getenv("RELAY_PORT"), values["RELAY_PORT"], "9500"),
		RelayAPIPort: chooseNonEmpty(os.Getenv("RELAY_API_PORT"), values["RELAY_API_PORT"], "9501"),
		FeishuAppID: chooseNonEmpty(
			os.Getenv("FEISHU_APP_ID"),
			values["FEISHU_APP_ID"],
		),
		FeishuAppSecret: chooseNonEmpty(
			os.Getenv("FEISHU_APP_SECRET"),
			values["FEISHU_APP_SECRET"],
		),
		FeishuUseSystemProxy: chooseBool(
			os.Getenv("FEISHU_USE_SYSTEM_PROXY"),
			values["FEISHU_USE_SYSTEM_PROXY"],
			false,
		),
		ConfigPath: configPath,
	}
	return cfg, nil
}

func loadOptionalEnv(path string) (map[string]string, error) {
	if path == "" {
		return map[string]string{}, nil
	}
	values, err := LoadEnvFile(path)
	if err == nil {
		return values, nil
	}
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	return nil, err
}

func xdgConfigPath(parts ...string) string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(append([]string{base}, parts...)...)
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func firstEnv(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func chooseNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func chooseBool(primary, secondary string, fallback bool) bool {
	for _, value := range []string{primary, secondary} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
