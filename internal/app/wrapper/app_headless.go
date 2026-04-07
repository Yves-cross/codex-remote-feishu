package wrapper

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/debuglog"
)

func (a *App) bootstrapHeadlessCodex(childStdin io.Writer, rawLogger *debuglog.RawLogger, reportProblem func(agentproto.ErrorInfo)) error {
	frames, err := a.syntheticBootstrapFrames()
	if err != nil || len(frames) == 0 {
		return err
	}
	a.debugf("headless bootstrap: frames=%s", summarizeFrames(frames))
	for _, frame := range frames {
		if err := writeCodexFrame(childStdin, frame, a.debugf, rawLogger, reportProblem); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) syntheticBootstrapFrames() ([][]byte, error) {
	if !strings.EqualFold(strings.TrimSpace(a.config.Source), "headless") {
		return nil, nil
	}
	payload := map[string]any{
		"id":     "relay-bootstrap-initialize",
		"method": "initialize",
		"params": map[string]any{
			"clientInfo": map[string]any{
				"name":    "Codex Remote Headless",
				"title":   "Codex Remote Headless",
				"version": firstNonEmpty(a.config.Version, "dev"),
			},
			"capabilities": map[string]any{
				"experimentalApi": true,
			},
		},
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return [][]byte{append(bytes, '\n')}, nil
}
