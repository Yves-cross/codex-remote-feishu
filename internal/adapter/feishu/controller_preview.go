package feishu

import (
	"context"
	"net/http"

	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
)

type gatewayPreviewRuntime interface {
	FinalBlockPreviewService
	FinalBlockPreviewMaintenanceService
	WebPreviewConfigurable
	WebPreviewRouteService
}

type noopGatewayPreviewer struct{}

func (noopGatewayPreviewer) RewriteFinalBlock(_ context.Context, req FinalBlockPreviewRequest) (FinalBlockPreviewResult, error) {
	return FinalBlockPreviewResult{Block: req.Block}, nil
}

func (noopGatewayPreviewer) RunBackgroundMaintenance(context.Context) {}

func (noopGatewayPreviewer) SetWebPreviewPublisher(WebPreviewPublisher) {}

func (noopGatewayPreviewer) ServeWebPreview(http.ResponseWriter, *http.Request, string, string, bool) bool {
	return false
}

func (c *MultiGatewayController) RewriteFinalBlock(ctx context.Context, req FinalBlockPreviewRequest) (result FinalBlockPreviewResult, err error) {
	result = FinalBlockPreviewResult{Block: req.Block}
	resolution := c.resolveGatewayTarget(eventcontract.TargetRef{
		GatewayID:        req.GatewayID,
		SurfaceSessionID: req.SurfaceSessionID,
		SelectionPolicy:  eventcontract.GatewaySelectionAllowSurfaceDerived,
		FailurePolicy:    eventcontract.GatewayFailureNoop,
	}, gatewayTargetRequirePreviewer)
	if !resolution.ok() {
		return result, nil
	}
	req.GatewayID = resolution.GatewayID
	return resolution.Worker.previewer.RewriteFinalBlock(ctx, req)
}

func (c *MultiGatewayController) SetWebPreviewPublisher(publisher WebPreviewPublisher) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.webPreviewPublisher = publisher
	for _, worker := range c.workers {
		if worker == nil || worker.previewer == nil {
			continue
		}
		worker.previewer.SetWebPreviewPublisher(publisher)
	}
}

func (c *MultiGatewayController) ServeWebPreview(w http.ResponseWriter, r *http.Request, scopePublicID, previewID string, download bool) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, worker := range c.workers {
		if worker == nil || worker.previewer == nil {
			continue
		}
		if worker.previewer.ServeWebPreview(w, r, scopePublicID, previewID, download) {
			return true
		}
	}
	return false
}
