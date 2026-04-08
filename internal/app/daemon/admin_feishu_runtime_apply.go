package daemon

import (
	"net/http"
	"strings"
	"time"
)

const (
	feishuRuntimeApplyActionUpsert = "upsert"
	feishuRuntimeApplyActionRemove = "remove"
)

type feishuRuntimeApplyPendingState struct {
	Summary   adminFeishuAppSummary
	Action    string
	Error     string
	UpdatedAt time.Time
}

func (a *App) snapshotFeishuRuntimeApplyPending() map[string]feishuRuntimeApplyPendingState {
	a.adminFeishuMu.RLock()
	defer a.adminFeishuMu.RUnlock()
	if len(a.feishuRuntimeApply) == 0 {
		return map[string]feishuRuntimeApplyPendingState{}
	}
	values := make(map[string]feishuRuntimeApplyPendingState, len(a.feishuRuntimeApply))
	for gatewayID, pending := range a.feishuRuntimeApply {
		values[gatewayID] = pending
	}
	return values
}

func (a *App) feishuRuntimeApplyPendingState(gatewayID string) (feishuRuntimeApplyPendingState, bool) {
	a.adminFeishuMu.RLock()
	defer a.adminFeishuMu.RUnlock()
	pending, ok := a.feishuRuntimeApply[canonicalGatewayID(gatewayID)]
	return pending, ok
}

func (a *App) markFeishuRuntimeApplyPending(summary adminFeishuAppSummary, action string, err error) feishuRuntimeApplyPendingState {
	now := time.Now().UTC()
	summary.RuntimeApply = nil
	pending := feishuRuntimeApplyPendingState{
		Summary:   summary,
		Action:    strings.TrimSpace(action),
		Error:     strings.TrimSpace(err.Error()),
		UpdatedAt: now,
	}
	a.adminFeishuMu.Lock()
	a.feishuRuntimeApply[canonicalGatewayID(summary.ID)] = pending
	a.adminFeishuMu.Unlock()
	return pending
}

func (a *App) clearFeishuRuntimeApplyPending(gatewayID string) {
	gatewayID = canonicalGatewayID(gatewayID)
	a.adminFeishuMu.Lock()
	delete(a.feishuRuntimeApply, gatewayID)
	a.adminFeishuMu.Unlock()
}

func applyFeishuRuntimePending(summary adminFeishuAppSummary, pending feishuRuntimeApplyPendingState) adminFeishuAppSummary {
	updatedAt := pending.UpdatedAt
	summary.RuntimeApply = &adminFeishuRuntimeApplyView{
		Pending:        true,
		Action:         pending.Action,
		Error:          pending.Error,
		UpdatedAt:      &updatedAt,
		RetryAvailable: true,
	}
	return summary
}

func (a *App) writeFeishuRuntimeApplyError(w http.ResponseWriter, gatewayID string, summary adminFeishuAppSummary, action string, message string, err error) {
	pending := a.markFeishuRuntimeApplyPending(summary, action, err)
	summary = applyFeishuRuntimePending(summary, pending)
	writeAPIError(w, http.StatusInternalServerError, apiError{
		Code:      "gateway_apply_failed",
		Message:   message,
		Retryable: true,
		Details: feishuRuntimeApplyErrorDetails{
			GatewayID: canonicalGatewayID(gatewayID),
			App:       &summary,
		},
	})
}
