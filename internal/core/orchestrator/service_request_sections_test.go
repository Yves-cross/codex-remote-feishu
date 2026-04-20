package orchestrator

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func containsPromptSectionLine(section control.FeishuCardTextSection, want string) bool {
	for _, line := range section.Lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

func containsStatePromptSectionLine(section state.RequestPromptTextSectionRecord, want string) bool {
	for _, line := range section.Lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}
