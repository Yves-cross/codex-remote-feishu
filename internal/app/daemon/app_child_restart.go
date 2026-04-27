package daemon

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func (a *App) restartRelayChildCodex(instanceID string) error {
	command, err := a.newRelayChildCodexRestartCommand(instanceID)
	if err != nil {
		return err
	}
	return a.sendRelayChildRestartCommand(instanceID, command)
}

func (a *App) newRelayChildCodexRestartCommand(instanceID string) (agentproto.Command, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return agentproto.Command{}, fmt.Errorf("missing instance id for child restart")
	}
	if a.sendAgentCommand == nil {
		return agentproto.Command{}, fmt.Errorf("agent command sender is unavailable")
	}
	return agentproto.Command{
		CommandID: a.nextCommandID(),
		Kind:      agentproto.CommandProcessChildRestart,
	}, nil
}

func (a *App) sendRelayChildRestartCommand(instanceID string, command agentproto.Command) error {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return fmt.Errorf("missing instance id for child restart")
	}
	if strings.TrimSpace(command.CommandID) == "" {
		return fmt.Errorf("missing command id for child restart")
	}
	if command.Kind != agentproto.CommandProcessChildRestart {
		return fmt.Errorf("unexpected child restart command kind: %s", command.Kind)
	}
	if a.sendAgentCommand == nil {
		return fmt.Errorf("agent command sender is unavailable")
	}
	return a.sendAgentCommand(instanceID, command)
}

func (a *App) restartRelayChildCodexWithCommandID(instanceID string) (string, error) {
	command, err := a.newRelayChildCodexRestartCommand(instanceID)
	if err != nil {
		return "", err
	}
	if err := a.sendRelayChildRestartCommand(instanceID, command); err != nil {
		return "", err
	}
	return command.CommandID, nil
}
