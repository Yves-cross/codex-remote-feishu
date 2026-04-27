package daemon

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func TestRestartRelayChildCodexSendsRestartCommand(t *testing.T) {
	app := &App{}
	var (
		gotInstance string
		gotCommand  agentproto.Command
	)
	app.sendAgentCommand = func(instanceID string, command agentproto.Command) error {
		gotInstance = instanceID
		gotCommand = command
		return nil
	}

	if err := app.restartRelayChildCodex("inst-1"); err != nil {
		t.Fatalf("restartRelayChildCodex: %v", err)
	}
	if gotInstance != "inst-1" {
		t.Fatalf("expected instance inst-1, got %q", gotInstance)
	}
	if gotCommand.Kind != agentproto.CommandProcessChildRestart {
		t.Fatalf("expected process.child.restart command, got %#v", gotCommand)
	}
	if gotCommand.CommandID == "" {
		t.Fatal("expected generated command id")
	}
}

func TestNewRelayChildCodexRestartCommandGeneratesCommand(t *testing.T) {
	app := &App{sendAgentCommand: func(string, agentproto.Command) error { return nil }}

	command, err := app.newRelayChildCodexRestartCommand("inst-1")
	if err != nil {
		t.Fatalf("newRelayChildCodexRestartCommand: %v", err)
	}
	if command.Kind != agentproto.CommandProcessChildRestart {
		t.Fatalf("expected process.child.restart command, got %#v", command)
	}
	if command.CommandID == "" {
		t.Fatal("expected generated command id")
	}
}
