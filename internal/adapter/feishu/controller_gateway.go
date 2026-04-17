package feishu

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (c *MultiGatewayController) Start(ctx context.Context, handler ActionHandler) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = true
	c.startCtx = ctx
	c.actionHandler = handler
	workerIDs := c.workerIDsLocked()
	c.mu.Unlock()

	for _, gatewayID := range workerIDs {
		c.mu.Lock()
		_ = c.ensureWorkerRunningLocked(gatewayID)
		c.mu.Unlock()
	}

	<-ctx.Done()
	c.stopAllWorkers()
	c.mu.Lock()
	c.started = false
	c.startCtx = nil
	c.actionHandler = nil
	c.mu.Unlock()
	return nil
}

func (c *MultiGatewayController) Apply(ctx context.Context, operations []Operation) error {
	grouped := map[string][]int{}

	c.mu.RLock()
	workerCount := len(c.workers)
	soleGatewayID := ""
	if workerCount == 1 {
		for gatewayID := range c.workers {
			soleGatewayID = gatewayID
		}
	}
	c.mu.RUnlock()

	for i, operation := range operations {
		gatewayID := strings.TrimSpace(operation.GatewayID)
		if gatewayID == "" {
			if workerCount != 1 {
				return fmt.Errorf("gateway apply failed: missing gateway id")
			}
			gatewayID = soleGatewayID
		}
		grouped[gatewayID] = append(grouped[gatewayID], i)
	}

	for gatewayID, indexes := range grouped {
		c.mu.RLock()
		worker := c.workers[gatewayID]
		c.mu.RUnlock()
		if worker == nil || worker.runtime == nil {
			return fmt.Errorf("gateway apply failed: gateway %s is not running", gatewayID)
		}
		group := make([]Operation, 0, len(indexes))
		for _, index := range indexes {
			group = append(group, operations[index])
		}
		if err := worker.runtime.Apply(ctx, group); err != nil {
			c.updateWorkerError(gatewayID, err)
			return err
		}
		for i, index := range indexes {
			operations[index] = group[i]
		}
	}
	return nil
}

func (c *MultiGatewayController) SendIMFile(ctx context.Context, req IMFileSendRequest) (IMFileSendResult, error) {
	result := IMFileSendResult{
		GatewayID:        normalizeGatewayID(req.GatewayID),
		SurfaceSessionID: strings.TrimSpace(req.SurfaceSessionID),
	}

	c.mu.RLock()
	workerCount := len(c.workers)
	soleGatewayID := ""
	if workerCount == 1 {
		for gatewayID := range c.workers {
			soleGatewayID = gatewayID
		}
	}
	c.mu.RUnlock()

	gatewayID := normalizeGatewayID(req.GatewayID)
	if gatewayID == "" {
		if workerCount != 1 {
			return result, &IMFileSendError{
				Code: IMFileSendErrorGatewayNotRunning,
				Err:  fmt.Errorf("send file failed: missing gateway id"),
			}
		}
		gatewayID = soleGatewayID
	}

	req.GatewayID = gatewayID
	result.GatewayID = gatewayID

	c.mu.RLock()
	worker := c.workers[gatewayID]
	c.mu.RUnlock()
	if worker == nil || worker.runtime == nil {
		return result, &IMFileSendError{
			Code: IMFileSendErrorGatewayNotRunning,
			Err:  fmt.Errorf("send file failed: gateway %s is not running", gatewayID),
		}
	}
	result, err := worker.runtime.SendIMFile(ctx, req)
	if err != nil {
		c.updateWorkerError(gatewayID, err)
		return result, err
	}
	if result.GatewayID == "" {
		result.GatewayID = gatewayID
	}
	return result, nil
}

func (c *MultiGatewayController) ensureWorkerRunningLocked(gatewayID string) error {
	worker := c.workers[gatewayID]
	if worker == nil || !worker.config.Enabled || !workerHasCredentials(worker.config) {
		return nil
	}
	worker.generation++
	generation := worker.generation
	runtime := c.newGateway(worker.config)
	if runtime == nil {
		return fmt.Errorf("gateway %s runtime factory returned nil", gatewayID)
	}
	runtime.SetStateHook(func(state GatewayState, err error) {
		c.applyStateHook(gatewayID, generation, state, err)
	})
	worker.runtime = runtime
	worker.previewer = c.newPreviewer(runtime, worker.config)
	worker.previewer.SetWebPreviewPublisher(c.webPreviewPublisher)
	worker.status.Disabled = false
	worker.status.State = GatewayStateConnecting
	worker.status.LastError = ""

	workerCtx, cancel := context.WithCancel(c.startCtx)
	worker.cancel = cancel
	handler := c.actionHandler
	go func(runtime gatewayRuntime, ctx context.Context) {
		err := runtime.Start(ctx, handler)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.applyStateHook(gatewayID, generation, GatewayStateDegraded, err)
			return
		}
		c.applyStateHook(gatewayID, generation, GatewayStateStopped, nil)
	}(runtime, workerCtx)
	go worker.previewer.RunBackgroundMaintenance(workerCtx)
	return nil
}

func (c *MultiGatewayController) applyStateHook(gatewayID string, generation uint64, state GatewayState, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	worker := c.workers[gatewayID]
	if worker == nil || worker.generation != generation {
		return
	}
	worker.status.State = state
	worker.status.Disabled = state == GatewayStateDisabled
	if state == GatewayStateConnected {
		worker.status.LastConnectedAt = time.Now().UTC()
		worker.status.LastError = ""
		return
	}
	if err != nil {
		worker.status.LastError = err.Error()
	}
}

func (c *MultiGatewayController) updateWorkerError(gatewayID string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	worker := c.workers[gatewayID]
	if worker == nil {
		return
	}
	worker.status.LastError = err.Error()
	if worker.status.State == GatewayStateConnected {
		worker.status.State = GatewayStateDegraded
	}
}

func (c *MultiGatewayController) stopAllWorkers() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, worker := range c.workers {
		c.stopWorkerLocked(worker)
	}
}

func (c *MultiGatewayController) stopWorkerLocked(worker *gatewayWorker) {
	if worker == nil {
		return
	}
	if worker.cancel != nil {
		worker.cancel()
		worker.cancel = nil
	}
	worker.runtime = nil
	worker.previewer = nil
	if worker.config.Enabled {
		worker.status.State = GatewayStateStopped
	}
}

func (c *MultiGatewayController) workerIDsLocked() []string {
	ids := make([]string, 0, len(c.workers))
	for gatewayID := range c.workers {
		ids = append(ids, gatewayID)
	}
	sort.Strings(ids)
	return ids
}
