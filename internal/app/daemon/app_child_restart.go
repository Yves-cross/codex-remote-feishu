package daemon

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func (a *App) restartRelayChildCodex(instanceID string) error {
	_, err := a.restartRelayChildCodexWithCommandID(instanceID)
	return err
}

func (a *App) restartRelayChildCodexWithCommandID(instanceID string) (string, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return "", fmt.Errorf("missing instance id for child restart")
	}
	if a.sendAgentCommand == nil {
		return "", fmt.Errorf("agent command sender is unavailable")
	}
	commandID := a.nextCommandID()
	if err := a.sendAgentCommand(instanceID, agentproto.Command{
		CommandID: commandID,
		Kind:      agentproto.CommandProcessChildRestart,
	}); err != nil {
		return "", err
	}
	return commandID, nil
}
