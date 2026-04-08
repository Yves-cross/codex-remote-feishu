package daemon

import (
	"fmt"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

const oldInboundActionWindow = 2 * time.Minute

func daemonLifecycleID(identity agentproto.ServerIdentity, startedAt time.Time) string {
	startedAt = startedAt.UTC()
	switch {
	case !startedAt.IsZero() && identity.PID > 0:
		return fmt.Sprintf("%s|pid:%d", startedAt.Format(time.RFC3339Nano), identity.PID)
	case !startedAt.IsZero():
		return startedAt.Format(time.RFC3339Nano)
	case identity.PID > 0:
		return fmt.Sprintf("pid:%d", identity.PID)
	default:
		return "unknown"
	}
}

func (a *App) classifyInboundAction(action control.Action) control.Action {
	if action.Inbound == nil {
		action.Inbound = &control.ActionInboundMeta{}
	}
	meta := action.Inbound
	cardLifecycleID := strings.TrimSpace(meta.CardDaemonLifecycleID)
	if cardLifecycleID != "" && cardLifecycleID != strings.TrimSpace(a.daemonLifecycleID) {
		meta.LifecycleVerdict = control.InboundLifecycleOldCard
		meta.LifecycleReason = "card_lifecycle_mismatch"
		return action
	}

	switch {
	case a.inboundTimeIsOld(meta.MessageCreateTime):
		meta.LifecycleVerdict = control.InboundLifecycleOld
		meta.LifecycleReason = "message_before_start_window"
	case a.inboundTimeIsOld(meta.MenuClickTime):
		meta.LifecycleVerdict = control.InboundLifecycleOld
		meta.LifecycleReason = "menu_before_start_window"
	default:
		meta.LifecycleVerdict = control.InboundLifecycleCurrent
		meta.LifecycleReason = ""
	}
	return action
}

func (a *App) inboundTimeIsOld(ts time.Time) bool {
	if ts.IsZero() || a.daemonStartedAt.IsZero() {
		return false
	}
	return !ts.After(a.daemonStartedAt.Add(-oldInboundActionWindow))
}

func inboundVerdict(action control.Action) control.InboundLifecycleVerdict {
	if action.Inbound == nil {
		return control.InboundLifecycleCurrent
	}
	if action.Inbound.LifecycleVerdict == "" {
		return control.InboundLifecycleCurrent
	}
	return action.Inbound.LifecycleVerdict
}

func inboundReason(action control.Action) string {
	if action.Inbound == nil {
		return ""
	}
	return strings.TrimSpace(action.Inbound.LifecycleReason)
}

func inboundTimeValue(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}
