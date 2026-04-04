package renderer

import (
	"fmt"
	"strings"

	"fschannel/internal/core/render"
)

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) PlanAssistantBlocks(surfaceID, instanceID, threadID, turnID, itemID, text string) []render.Block {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return []render.Block{{
		ID:               fmt.Sprintf("%s-1", itemID),
		SurfaceSessionID: surfaceID,
		InstanceID:       instanceID,
		ThreadID:         threadID,
		TurnID:           turnID,
		ItemID:           itemID,
		Kind:             render.BlockAssistantMarkdown,
		Text:             text,
	}}
}
