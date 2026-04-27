package control

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/app/install"
)

type UpgradeCommandMode string

const (
	UpgradeCommandShowStatus UpgradeCommandMode = "status"
	UpgradeCommandShowTrack  UpgradeCommandMode = "track_show"
	UpgradeCommandSetTrack   UpgradeCommandMode = "track_set"
	UpgradeCommandLatest     UpgradeCommandMode = "latest"
	UpgradeCommandCodex      UpgradeCommandMode = "codex"
	UpgradeCommandDev        UpgradeCommandMode = "dev"
	UpgradeCommandLocal      UpgradeCommandMode = "local"
)

type ParsedUpgradeCommand struct {
	Mode  UpgradeCommandMode
	Track install.ReleaseTrack
}

type UpgradeCommandPresentation string

const (
	UpgradeCommandPresentationPage    UpgradeCommandPresentation = "page"
	UpgradeCommandPresentationExecute UpgradeCommandPresentation = "execute"
)

func ParseFeishuUpgradeCommandText(text string) (ParsedUpgradeCommand, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ParsedUpgradeCommand{}, fmt.Errorf("缺少 /upgrade 子命令。")
	}
	fields := strings.Fields(strings.ToLower(trimmed))
	if len(fields) == 0 || fields[0] != "/upgrade" {
		return ParsedUpgradeCommand{}, fmt.Errorf("不支持的 /upgrade 子命令。")
	}
	switch len(fields) {
	case 1:
		return ParsedUpgradeCommand{Mode: UpgradeCommandShowStatus}, nil
	case 2:
		switch fields[1] {
		case "track":
			return ParsedUpgradeCommand{Mode: UpgradeCommandShowTrack}, nil
		case "latest":
			return ParsedUpgradeCommand{Mode: UpgradeCommandLatest}, nil
		case "codex":
			return ParsedUpgradeCommand{Mode: UpgradeCommandCodex}, nil
		case "dev":
			return ParsedUpgradeCommand{Mode: UpgradeCommandDev}, nil
		case "local":
			return ParsedUpgradeCommand{Mode: UpgradeCommandLocal}, nil
		default:
			return ParsedUpgradeCommand{}, fmt.Errorf("不支持的 /upgrade 子命令。")
		}
	case 3:
		if fields[1] != "track" {
			return ParsedUpgradeCommand{}, fmt.Errorf("不支持的 /upgrade 子命令。")
		}
		track := install.ParseReleaseTrack(fields[2])
		if track == "" {
			return ParsedUpgradeCommand{}, fmt.Errorf("track 只支持 alpha、beta、production。")
		}
		return ParsedUpgradeCommand{Mode: UpgradeCommandSetTrack, Track: track}, nil
	default:
		return ParsedUpgradeCommand{}, fmt.Errorf("不支持的 /upgrade 子命令。")
	}
}

func FeishuUpgradeCommandPresentationForText(text string) (UpgradeCommandPresentation, bool) {
	parsed, err := ParseFeishuUpgradeCommandText(text)
	if err != nil {
		return "", false
	}
	switch parsed.Mode {
	case UpgradeCommandShowStatus, UpgradeCommandShowTrack:
		return UpgradeCommandPresentationPage, true
	case UpgradeCommandSetTrack, UpgradeCommandLatest, UpgradeCommandCodex, UpgradeCommandDev, UpgradeCommandLocal:
		return UpgradeCommandPresentationExecute, true
	default:
		return "", false
	}
}

func FeishuUpgradeCommandRunsImmediately(text string) bool {
	presentation, ok := FeishuUpgradeCommandPresentationForText(text)
	return ok && presentation == UpgradeCommandPresentationExecute
}
