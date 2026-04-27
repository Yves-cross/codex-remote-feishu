package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/app/install"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "self install target error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	flagSet := flag.NewFlagSet("self-install-target", flag.ContinueOnError)
	flagSet.SetOutput(stderr)
	format := flagSet.String("format", "json", "output format: json or shell")
	if err := flagSet.Parse(args); err != nil {
		return err
	}

	info, err := install.ResolveCurrentDaemonTargetInfo()
	if err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	case "shell":
		_, err := fmt.Fprint(stdout, shellAssignments(info))
		return err
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}
}

func shellAssignments(info install.CurrentDaemonTargetInfo) string {
	values := []struct {
		key   string
		value string
	}{
		{"CODEX_REMOTE_SELF_TARGET_RESOLVER_SOURCE", info.ResolverSource},
		{"CODEX_REMOTE_SELF_TARGET_RUNTIME_INSTANCE_ID", info.RuntimeInstanceID},
		{"CODEX_REMOTE_SELF_TARGET_RUNTIME_INSTANCE_SOURCE", info.RuntimeInstanceSource},
		{"CODEX_REMOTE_SELF_TARGET_RUNTIME_MANAGED", shellBool(info.RuntimeManaged)},
		{"CODEX_REMOTE_SELF_TARGET_RUNTIME_LIFETIME", info.RuntimeLifetime},
		{"CODEX_REMOTE_SELF_TARGET_INSTANCE_ID", info.InstanceID},
		{"CODEX_REMOTE_SELF_TARGET_BASE_DIR", info.BaseDir},
		{"CODEX_REMOTE_SELF_TARGET_CONFIG_PATH", info.ConfigPath},
		{"CODEX_REMOTE_SELF_TARGET_CONFIG_EXISTS", shellBool(info.ConfigExists)},
		{"CODEX_REMOTE_SELF_TARGET_STATE_PATH", info.StatePath},
		{"CODEX_REMOTE_SELF_TARGET_STATE_EXISTS", shellBool(info.StateExists)},
		{"CODEX_REMOTE_SELF_TARGET_SERVICE_NAME", info.ServiceName},
		{"CODEX_REMOTE_SELF_TARGET_SERVICE_UNIT_PATH", info.ServiceUnitPath},
		{"CODEX_REMOTE_SELF_TARGET_LOG_PATH", info.LogPath},
		{"CODEX_REMOTE_SELF_TARGET_RAW_LOG_PATH", info.RawLogPath},
		{"CODEX_REMOTE_SELF_TARGET_PID_PATH", info.PIDPath},
		{"CODEX_REMOTE_SELF_TARGET_LOCAL_UPGRADE_ARTIFACT_PATH", info.LocalUpgradeArtifact},
		{"CODEX_REMOTE_SELF_TARGET_CURRENT_VERSION", info.CurrentVersion},
		{"CODEX_REMOTE_SELF_TARGET_CURRENT_BINARY_PATH", info.CurrentBinaryPath},
		{"CODEX_REMOTE_SELF_TARGET_PENDING_UPGRADE_PHASE", info.PendingUpgradePhase},
		{"CODEX_REMOTE_SELF_TARGET_RELAY_LISTEN_HOST", info.Relay.ListenHost},
		{"CODEX_REMOTE_SELF_TARGET_RELAY_LISTEN_PORT", strconv.Itoa(info.Relay.ListenPort)},
		{"CODEX_REMOTE_SELF_TARGET_RELAY_URL", info.Relay.URL},
		{"CODEX_REMOTE_SELF_TARGET_RELAY_SERVER_URL", info.Relay.ServerURL},
		{"CODEX_REMOTE_SELF_TARGET_ADMIN_LISTEN_HOST", info.Admin.ListenHost},
		{"CODEX_REMOTE_SELF_TARGET_ADMIN_LISTEN_PORT", strconv.Itoa(info.Admin.ListenPort)},
		{"CODEX_REMOTE_SELF_TARGET_ADMIN_URL", info.Admin.URL},
		{"CODEX_REMOTE_SELF_TARGET_TOOL_LISTEN_HOST", info.Tool.ListenHost},
		{"CODEX_REMOTE_SELF_TARGET_TOOL_LISTEN_PORT", strconv.Itoa(info.Tool.ListenPort)},
		{"CODEX_REMOTE_SELF_TARGET_TOOL_URL", info.Tool.URL},
		{"CODEX_REMOTE_SELF_TARGET_EXTERNAL_ACCESS_LISTEN_HOST", info.ExternalAccess.ListenHost},
		{"CODEX_REMOTE_SELF_TARGET_EXTERNAL_ACCESS_LISTEN_PORT", strconv.Itoa(info.ExternalAccess.ListenPort)},
		{"CODEX_REMOTE_SELF_TARGET_EXTERNAL_ACCESS_URL", info.ExternalAccess.URL},
		{"CODEX_REMOTE_SELF_TARGET_PPROF_ENABLED", shellBool(info.Pprof.Enabled)},
		{"CODEX_REMOTE_SELF_TARGET_PPROF_LISTEN_HOST", info.Pprof.ListenHost},
		{"CODEX_REMOTE_SELF_TARGET_PPROF_LISTEN_PORT", strconv.Itoa(info.Pprof.ListenPort)},
		{"CODEX_REMOTE_SELF_TARGET_PPROF_URL", info.Pprof.URL},
	}

	var b strings.Builder
	for _, item := range values {
		b.WriteString(item.key)
		b.WriteByte('=')
		b.WriteString(shellQuote(item.value))
		b.WriteByte('\n')
	}
	return b.String()
}

func shellBool(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
