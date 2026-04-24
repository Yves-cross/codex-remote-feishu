package daemon

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func (a *App) restartRelayChildCodex(instanceID string) error {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return fmt.Errorf("missing instance id for child restart")
	}
	if a.sendAgentCommand == nil {
		return fmt.Errorf("agent command sender is unavailable")
	}
	return a.sendAgentCommand(instanceID, agentproto.Command{
		CommandID: a.nextCommandID(),
		Kind:      agentproto.CommandProcessChildRestart,
	})
}
