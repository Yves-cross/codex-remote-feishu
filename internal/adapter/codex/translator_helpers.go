package codex

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func chooseAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func setDefault(target map[string]any, key string, value any) {
	if _, exists := target[key]; !exists {
		target[key] = value
	}
}

func isNull(value any) bool {
	return value == nil
}

func lookupString(value map[string]any, path ...string) string {
	var current any = value
	for _, part := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[part]
	}
	return lookupStringFromAny(current)
}

func lookupAny(value map[string]any, path ...string) any {
	var current any = value
	for _, part := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func lookupMap(value map[string]any, path ...string) map[string]any {
	current, _ := lookupAny(value, path...).(map[string]any)
	return current
}

func lookupMapFromAny(value any) map[string]any {
	current, _ := value.(map[string]any)
	if current == nil {
		return map[string]any{}
	}
	return cloneMap(current)
}

func lookupStringFromAny(value any) string {
	switch current := value.(type) {
	case string:
		return current
	default:
		return ""
	}
}

func lookupIntFromAny(value any) int {
	switch current := value.(type) {
	case int:
		return current
	case int32:
		return int(current)
	case int64:
		return int(current)
	case float64:
		return int(current)
	default:
		return 0
	}
}

func lookupBoolFromAny(value any) bool {
	current, _ := value.(bool)
	return current
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func extractJSONRPCErrorMessage(message map[string]any) string {
	if message == nil {
		return ""
	}
	errorMap, _ := message["error"].(map[string]any)
	return firstNonEmptyString(
		lookupStringFromAny(errorMap["message"]),
		lookupStringFromAny(message["error"]),
	)
}

func choose(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeItemKind(raw string) string {
	switch raw {
	case "agentMessage", "assistant_message", "assistantMessage":
		return "agent_message"
	case "userMessage", "user_message":
		return "user_message"
	case "plan":
		return "plan"
	case "reasoning":
		return "reasoning"
	case "commandExecution", "command_execution":
		return "command_execution"
	case "fileChange", "file_change":
		return "file_change"
	case "mcpToolCall", "mcp_tool_call":
		return "mcp_tool_call"
	case "dynamicToolCall", "dynamic_tool_call":
		return "dynamic_tool_call"
	case "collabAgentToolCall", "collab_agent_tool_call":
		return "collab_agent_tool_call"
	default:
		return raw
	}
}

func extractItemMetadata(itemKind string, item map[string]any) map[string]any {
	metadata := map[string]any{}
	if item == nil {
		return metadata
	}
	if text := extractItemText(item); text != "" {
		metadata["text"] = text
	}
	switch itemKind {
	case "reasoning":
		if summary := extractStringList(item["summary"]); len(summary) > 0 {
			metadata["summary"] = summary
		}
		if content := extractStringList(item["content"]); len(content) > 0 {
			metadata["content"] = content
		}
	}
	return metadata
}

func extractItemStatus(item map[string]any) string {
	if item == nil {
		return ""
	}
	return firstNonEmptyString(
		lookupStringFromAny(item["status"]),
		lookupString(item, "item", "status"),
	)
}

func extractFileChangeRecords(itemKind string, item map[string]any) []agentproto.FileChangeRecord {
	if itemKind != "file_change" || item == nil {
		return nil
	}
	source := item["changes"]
	if source == nil {
		source = item["fileChanges"]
	}
	if source == nil {
		source = lookupAny(item, "fileChange", "changes")
	}
	if source == nil {
		return nil
	}
	var rawChanges []any
	switch typed := source.(type) {
	case []any:
		rawChanges = typed
	case []map[string]any:
		rawChanges = make([]any, 0, len(typed))
		for _, current := range typed {
			rawChanges = append(rawChanges, current)
		}
	default:
		return nil
	}
	records := make([]agentproto.FileChangeRecord, 0, len(rawChanges))
	for _, raw := range rawChanges {
		record, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := firstNonEmptyString(
			lookupStringFromAny(record["path"]),
			lookupString(record, "file", "path"),
			lookupStringFromAny(record["new_path"]),
		)
		kind, movePath := extractPatchChangeKind(record["kind"])
		if movePath == "" {
			movePath = firstNonEmptyString(
				lookupStringFromAny(record["move_path"]),
				lookupStringFromAny(record["movePath"]),
			)
		}
		diff := firstNonEmptyString(
			lookupStringFromAny(record["diff"]),
			lookupStringFromAny(record["patch"]),
		)
		if path == "" && movePath == "" && diff == "" && kind == "" {
			continue
		}
		records = append(records, agentproto.FileChangeRecord{
			Path:     path,
			Kind:     kind,
			MovePath: movePath,
			Diff:     diff,
		})
	}
	if len(records) == 0 {
		return nil
	}
	return records
}

func extractPatchChangeKind(value any) (agentproto.FileChangeKind, string) {
	switch typed := value.(type) {
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "add":
			return agentproto.FileChangeAdd, ""
		case "delete":
			return agentproto.FileChangeDelete, ""
		case "update":
			return agentproto.FileChangeUpdate, ""
		}
	case map[string]any:
		kind, movePath := extractPatchChangeKind(typed["type"])
		if movePath == "" {
			movePath = firstNonEmptyString(
				lookupStringFromAny(typed["move_path"]),
				lookupStringFromAny(typed["movePath"]),
			)
		}
		return kind, movePath
	}
	return "", ""
}

func extractItemText(item map[string]any) string {
	if text := lookupStringFromAny(item["text"]); text != "" {
		return text
	}
	content, _ := item["content"].([]any)
	if len(content) == 0 {
		return ""
	}
	parts := make([]string, 0, len(content))
	for _, current := range content {
		entry, _ := current.(map[string]any)
		if entry == nil {
			continue
		}
		if lookupStringFromAny(entry["type"]) != "text" {
			continue
		}
		if text := lookupStringFromAny(entry["text"]); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func extractStringList(value any) []string {
	raw, _ := value.([]any)
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, current := range raw {
		if text := lookupStringFromAny(current); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func extractRequestPayload(message map[string]any) map[string]any {
	request := lookupMap(message, "params", "request")
	if len(request) > 0 {
		return request
	}
	request = lookupMap(message, "params", "serverRequest")
	if len(request) > 0 {
		return request
	}
	return map[string]any{}
}

func extractRequestID(message map[string]any, request map[string]any) string {
	return firstNonEmptyString(
		lookupStringFromAny(request["id"]),
		lookupString(message, "params", "requestId"),
		lookupString(message, "params", "id"),
	)
}

func extractRequestThreadID(message map[string]any, request map[string]any) string {
	return firstNonEmptyString(
		lookupString(message, "params", "thread", "id"),
		lookupString(message, "params", "threadId"),
		lookupString(request, "thread", "id"),
		lookupStringFromAny(request["threadId"]),
	)
}

func extractRequestTurnID(message map[string]any, request map[string]any) string {
	return firstNonEmptyString(
		lookupString(message, "params", "turn", "id"),
		lookupString(message, "params", "turnId"),
		lookupString(request, "turn", "id"),
		lookupStringFromAny(request["turnId"]),
	)
}

func extractRequestType(request, params map[string]any) string {
	switch raw := strings.ToLower(strings.TrimSpace(extractRawRequestType(request, params))); {
	case raw == "", raw == "approval", raw == "confirm", raw == "confirmation":
		return "approval"
	case strings.HasPrefix(raw, "approval"):
		return "approval"
	case strings.HasPrefix(raw, "confirm"):
		return "approval"
	default:
		return raw
	}
}

func extractRawRequestType(request, params map[string]any) string {
	return strings.TrimSpace(firstNonEmptyString(
		lookupStringFromAny(request["type"]),
		lookupStringFromAny(request["requestType"]),
		lookupStringFromAny(request["kind"]),
		lookupStringFromAny(params["type"]),
		lookupStringFromAny(params["requestType"]),
		lookupStringFromAny(params["kind"]),
	))
}

func extractRequestMetadata(requestType string, request, params map[string]any) map[string]any {
	metadata := map[string]any{}
	if requestType != "" {
		metadata["requestType"] = requestType
	}
	if rawType := extractRawRequestType(request, params); rawType != "" {
		metadata["requestKind"] = strings.ToLower(strings.TrimSpace(rawType))
	}
	title := firstNonEmptyString(
		lookupStringFromAny(request["title"]),
		lookupStringFromAny(request["name"]),
		lookupStringFromAny(params["title"]),
	)
	if title == "" {
		switch requestType {
		case "", "approval":
			title = "需要确认"
		default:
			title = "需要处理请求"
		}
	}
	if title != "" {
		metadata["title"] = title
	}
	body := firstNonEmptyString(
		lookupStringFromAny(request["message"]),
		lookupStringFromAny(request["description"]),
		lookupStringFromAny(request["body"]),
		lookupStringFromAny(request["prompt"]),
		lookupStringFromAny(request["reason"]),
		lookupStringFromAny(params["message"]),
		lookupStringFromAny(params["description"]),
		lookupStringFromAny(params["body"]),
	)
	command := extractRequestCommand(request, params)
	if command != "" {
		if body != "" {
			body += "\n\n"
		}
		body += "```text\n" + command + "\n```"
	}
	if body != "" {
		metadata["body"] = body
	}
	acceptLabel := firstNonEmptyString(
		lookupStringFromAny(request["acceptLabel"]),
		lookupStringFromAny(request["approveLabel"]),
		lookupStringFromAny(request["allowLabel"]),
		lookupStringFromAny(request["confirmLabel"]),
		lookupStringFromAny(params["acceptLabel"]),
	)
	if acceptLabel != "" {
		metadata["acceptLabel"] = acceptLabel
	}
	declineLabel := firstNonEmptyString(
		lookupStringFromAny(request["declineLabel"]),
		lookupStringFromAny(request["denyLabel"]),
		lookupStringFromAny(request["rejectLabel"]),
		lookupStringFromAny(params["declineLabel"]),
	)
	if declineLabel != "" {
		metadata["declineLabel"] = declineLabel
	}
	if options := extractRequestOptions(request, params); len(options) != 0 {
		metadata["options"] = options
	}
	return metadata
}

func extractResolvedRequestMetadata(requestType string, request, params map[string]any) map[string]any {
	metadata := map[string]any{}
	if requestType != "" {
		metadata["requestType"] = requestType
	}
	decision := firstNonEmptyString(
		lookupString(params, "result", "decision"),
		lookupString(params, "response", "decision"),
		lookupStringFromAny(params["decision"]),
		lookupString(request, "result", "decision"),
		lookupString(request, "response", "decision"),
		lookupStringFromAny(request["decision"]),
	)
	if decision != "" {
		metadata["decision"] = decision
	}
	return metadata
}

func extractRequestCommand(request, params map[string]any) string {
	command := firstNonEmptyString(
		lookupStringFromAny(request["command"]),
		lookupString(request, "command", "command"),
		lookupString(request, "command", "text"),
		lookupStringFromAny(params["command"]),
		lookupString(params, "command", "command"),
		lookupString(params, "command", "text"),
	)
	return strings.TrimSpace(command)
}

func extractRequestOptions(request, params map[string]any) []map[string]any {
	source := firstNonNil(
		request["options"],
		request["choices"],
		params["options"],
		params["choices"],
	)
	if source == nil {
		return nil
	}
	var rawOptions []any
	switch typed := source.(type) {
	case []any:
		rawOptions = typed
	case []map[string]any:
		rawOptions = make([]any, 0, len(typed))
		for _, item := range typed {
			rawOptions = append(rawOptions, item)
		}
	default:
		return nil
	}
	if len(rawOptions) == 0 {
		return nil
	}
	options := make([]map[string]any, 0, len(rawOptions))
	for _, raw := range rawOptions {
		record, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		optionID := normalizeRequestOptionID(firstNonEmptyString(
			lookupStringFromAny(record["id"]),
			lookupStringFromAny(record["optionId"]),
			lookupStringFromAny(record["decision"]),
			lookupStringFromAny(record["value"]),
			lookupStringFromAny(record["action"]),
		))
		if optionID == "" {
			continue
		}
		option := map[string]any{"id": optionID}
		label := firstNonEmptyString(
			lookupStringFromAny(record["label"]),
			lookupStringFromAny(record["title"]),
			lookupStringFromAny(record["text"]),
			lookupStringFromAny(record["name"]),
		)
		if label != "" {
			option["label"] = label
		}
		style := firstNonEmptyString(
			lookupStringFromAny(record["style"]),
			lookupStringFromAny(record["appearance"]),
			lookupStringFromAny(record["variant"]),
		)
		if style != "" {
			option["style"] = style
		}
		options = append(options, option)
	}
	return options
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func normalizeRequestOptionID(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	switch normalized {
	case "accept", "allow", "approve", "yes":
		return "accept"
	case "acceptforsession", "allowforsession", "allowthissession", "session":
		return "acceptForSession"
	case "decline", "deny", "reject", "no":
		return "decline"
	default:
		return strings.TrimSpace(value)
	}
}
