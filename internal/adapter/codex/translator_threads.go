package codex

import "github.com/kxn/codex-remote-feishu/internal/core/agentproto"

func parseThreadList(result any) []agentproto.ThreadSnapshotRecord {
	var raw []any
	switch value := result.(type) {
	case map[string]any:
		switch current := value["threads"].(type) {
		case []any:
			raw = current
		}
		if len(raw) == 0 {
			switch current := value["data"].(type) {
			case []any:
				raw = current
			}
		}
	case []any:
		raw = value
	}
	output := make([]agentproto.ThreadSnapshotRecord, 0, len(raw))
	for index, current := range raw {
		switch item := current.(type) {
		case string:
			output = append(output, agentproto.ThreadSnapshotRecord{ThreadID: item, Loaded: true, ListOrder: index + 1})
		case map[string]any:
			record := parseThreadRecord(item)
			record.Loaded = true
			record.ListOrder = index + 1
			if record.ThreadID != "" {
				output = append(output, record)
			}
		}
	}
	return output
}

func parseThreadRecord(result any) agentproto.ThreadSnapshotRecord {
	var object map[string]any
	switch value := result.(type) {
	case map[string]any:
		if thread, ok := value["thread"].(map[string]any); ok {
			object = thread
		} else {
			object = value
		}
	default:
		return agentproto.ThreadSnapshotRecord{}
	}
	return agentproto.ThreadSnapshotRecord{
		ThreadID: choose(
			lookupStringFromAny(object["id"]),
			lookupStringFromAny(object["threadId"]),
		),
		Name: choose(
			lookupStringFromAny(object["name"]),
			lookupStringFromAny(object["title"]),
		),
		Preview: choose(
			lookupStringFromAny(object["preview"]),
			lookupStringFromAny(object["summary"]),
		),
		CWD: choose(
			lookupStringFromAny(object["cwd"]),
			lookupStringFromAny(object["path"]),
		),
		Model: choose(
			lookupString(object, "latestCollaborationMode", "settings", "model"),
			lookupString(object, "collaborationMode", "settings", "model"),
			lookupStringFromAny(object["model"]),
		),
		ReasoningEffort: choose(
			lookupString(object, "latestCollaborationMode", "settings", "reasoning_effort"),
			lookupString(object, "collaborationMode", "settings", "reasoning_effort"),
			lookupString(object, "config", "model_reasoning_effort"),
			lookupString(object, "config", "reasoning_effort"),
			lookupStringFromAny(object["effort"]),
		),
		Loaded:   lookupBoolFromAny(object["loaded"]),
		Archived: lookupBoolFromAny(object["archived"]),
		State:    lookupStringFromAny(object["state"]),
		ListOrder: lookupIntFromAny(chooseAny(
			object["listOrder"],
			object["list_order"],
		)),
	}
}
