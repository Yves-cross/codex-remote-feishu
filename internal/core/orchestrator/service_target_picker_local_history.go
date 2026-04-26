package orchestrator

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type targetPickerDialogueTurn struct {
	Ordinal   int
	User      string
	Assistant string
	IsCurrent bool
}

func targetPickerRecentLocalThreadHistoryLines(threadID string, limit int) []string {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}
	paths := targetPickerCodexSessionPaths(filepath.Join(home, ".codex", "sessions"), threadID)
	for _, path := range paths {
		lines := targetPickerRecentDialogueLinesFromSessionFile(path, limit)
		if len(lines) > 0 {
			return lines
		}
	}
	return nil
}

func targetPickerCodexSessionPaths(root, threadID string) []string {
	root = strings.TrimSpace(root)
	threadID = strings.TrimSpace(threadID)
	if root == "" || threadID == "" {
		return nil
	}
	type candidate struct {
		path    string
		modTime int64
	}
	var candidates []candidate
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") || !strings.Contains(name, threadID) {
			return nil
		}
		info, statErr := entry.Info()
		modTime := int64(0)
		if statErr == nil && info != nil {
			modTime = info.ModTime().UnixNano()
		}
		candidates = append(candidates, candidate{path: path, modTime: modTime})
		return nil
	})
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].modTime == candidates[j].modTime {
			return candidates[i].path > candidates[j].path
		}
		return candidates[i].modTime > candidates[j].modTime
	})
	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		paths = append(paths, candidate.path)
	}
	return paths
}

func targetPickerRecentDialogueLinesFromSessionFile(path string, limit int) []string {
	turns := targetPickerDialogueTurnsFromSessionFile(path)
	if len(turns) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	lines := make([]string, 0, limit)
	start := len(turns) - limit
	if start < 0 {
		start = 0
	}
	for i := start; i < len(turns); i++ {
		turn := turns[i]
		full := i == len(turns)-1
		input := targetPickerDialoguePreview(turn.User, full)
		output := targetPickerDialoguePreview(turn.Assistant, full)
		line := "#" + strconv.Itoa(turn.Ordinal) + " " + input
		if output != "" {
			line += " -> " + output
		}
		lines = append(lines, line)
	}
	return lines
}

func targetPickerDialogueTurnsFromSessionFile(path string) []targetPickerDialogueTurn {
	file, err := os.Open(strings.TrimSpace(path))
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var turns []targetPickerDialogueTurn
	for scanner.Scan() {
		role, text, ok := targetPickerResponseMessageFromJSONL(scanner.Bytes())
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		switch role {
		case "user":
			turns = append(turns, targetPickerDialogueTurn{
				Ordinal: len(turns) + 1,
				User:    strings.TrimSpace(text),
			})
		case "assistant":
			text = strings.TrimSpace(text)
			if len(turns) == 0 {
				turns = append(turns, targetPickerDialogueTurn{Ordinal: 1, Assistant: text})
				continue
			}
			if strings.TrimSpace(turns[len(turns)-1].Assistant) == "" {
				turns[len(turns)-1].Assistant = text
			} else {
				turns[len(turns)-1].Assistant += "\n" + text
			}
		}
	}
	return turns
}

func targetPickerResponseMessageFromJSONL(line []byte) (string, string, bool) {
	var envelope struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil || envelope.Type != "response_item" || len(envelope.Payload) == 0 {
		return "", "", false
	}
	var payload struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Phase   string `json:"phase"`
		Text    string `json:"text"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return "", "", false
	}
	if payload.Type != "message" {
		return "", "", false
	}
	role := strings.TrimSpace(payload.Role)
	if role != "user" && role != "assistant" {
		return "", "", false
	}
	if role == "assistant" && strings.TrimSpace(payload.Phase) == "commentary" {
		return "", "", false
	}
	parts := make([]string, 0, len(payload.Content)+1)
	if text := strings.TrimSpace(payload.Text); text != "" {
		parts = append(parts, text)
	}
	for _, item := range payload.Content {
		switch strings.TrimSpace(item.Type) {
		case "input_text", "output_text", "text":
			if text := strings.TrimSpace(item.Text); text != "" {
				parts = append(parts, text)
			}
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return "", "", false
	}
	return role, text, true
}
