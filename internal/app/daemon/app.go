package daemon

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/feishu"
	"github.com/kxn/codex-remote-feishu/internal/adapter/relayws"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/orchestrator"
	"github.com/kxn/codex-remote-feishu/internal/core/renderer"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

type App struct {
	service   *orchestrator.Service
	projector *feishu.Projector
	gateway   feishu.Gateway
	relay     *relayws.Server

	relayServer *http.Server
	apiServer   *http.Server

	commandSeq uint64
	mu         sync.Mutex
}

func New(relayAddr, apiAddr string, gateway feishu.Gateway) *App {
	if gateway == nil {
		gateway = feishu.NopGateway{}
	}
	app := &App{
		service:   orchestrator.NewService(time.Now, orchestrator.Config{TurnHandoffWait: 800 * time.Millisecond}, renderer.NewPlanner()),
		projector: feishu.NewProjector(),
		gateway:   gateway,
	}
	app.relay = relayws.NewServer(relayws.ServerCallbacks{
		OnHello:      app.onHello,
		OnEvents:     app.onEvents,
		OnCommandAck: app.onCommandAck,
		OnDisconnect: app.onDisconnect,
	})

	relayMux := http.NewServeMux()
	relayMux.Handle("/ws/agent", app.relay)
	relayMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	app.relayServer = &http.Server{Addr: relayAddr, Handler: relayMux}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	apiMux.HandleFunc("/v1/status", app.handleStatus)
	app.apiServer = &http.Server{Addr: apiAddr, Handler: apiMux}
	return app
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 3)

	go func() {
		if err := a.relayServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		if err := a.apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		if err := a.gateway.Start(ctx, a.HandleAction); err != nil && err != context.Canceled {
			errCh <- err
		}
	}()
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				a.onTick(ctx, now)
			}
		}
	}()

	select {
	case <-ctx.Done():
		_ = a.Shutdown(context.Background())
		return nil
	case err := <-errCh:
		_ = a.Shutdown(context.Background())
		return err
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	_ = a.relay.Close()
	_ = a.relayServer.Shutdown(ctx)
	_ = a.apiServer.Shutdown(ctx)
	return nil
}

func (a *App) HandleAction(ctx context.Context, action control.Action) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if action.Kind == control.ActionStatus {
		log.Printf("surface status requested: surface=%s chat=%s actor=%s message=%s", action.SurfaceSessionID, action.ChatID, action.ActorUserID, action.MessageID)
	}
	events := a.service.ApplySurfaceAction(action)
	a.handleUIEvents(ctx, events)
}

func (a *App) Service() *orchestrator.Service {
	return a.service
}

func (a *App) onHello(ctx context.Context, hello agentproto.Hello) {
	a.mu.Lock()
	defer a.mu.Unlock()

	inst := a.service.Instance(hello.Instance.InstanceID)
	if inst == nil {
		inst = &state.InstanceRecord{
			InstanceID: hello.Instance.InstanceID,
			Threads:    map[string]*state.ThreadRecord{},
		}
	}
	inst.DisplayName = hello.Instance.DisplayName
	inst.WorkspaceRoot = hello.Instance.WorkspaceRoot
	inst.WorkspaceKey = hello.Instance.WorkspaceKey
	inst.ShortName = hello.Instance.ShortName
	inst.Online = true
	a.service.UpsertInstance(inst)
	log.Printf("relay instance connected: id=%s workspace=%s display=%s", inst.InstanceID, inst.WorkspaceKey, inst.DisplayName)

	command := agentproto.Command{
		CommandID: a.nextCommandID(),
		Kind:      agentproto.CommandThreadsRefresh,
	}
	if err := a.relay.SendCommand(hello.Instance.InstanceID, command); err != nil {
		log.Printf("relay send command failed: instance=%s kind=%s err=%v", hello.Instance.InstanceID, command.Kind, err)
	}
}

func (a *App) onEvents(ctx context.Context, instanceID string, events []agentproto.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, event := range events {
		uiEvents := a.service.ApplyAgentEvent(instanceID, event)
		a.handleUIEvents(ctx, uiEvents)
	}
}

func (a *App) onCommandAck(context.Context, string, agentproto.CommandAck) {}

func (a *App) onDisconnect(ctx context.Context, instanceID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	inst := a.service.Instance(instanceID)
	if inst == nil {
		return
	}
	uiEvents := a.service.ApplyInstanceDisconnected(instanceID)
	log.Printf("relay instance disconnected: id=%s workspace=%s display=%s", inst.InstanceID, inst.WorkspaceKey, inst.DisplayName)
	a.handleUIEvents(ctx, uiEvents)
}

func (a *App) onTick(ctx context.Context, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	uiEvents := a.service.Tick(now)
	a.handleUIEvents(ctx, uiEvents)
}

func (a *App) handleUIEvents(ctx context.Context, events []control.UIEvent) {
	_ = ctx
	for _, event := range events {
		if event.Command != nil {
			if event.Command.CommandID == "" {
				event.Command.CommandID = a.nextCommandID()
			}
			instanceID := a.service.AttachedInstanceID(event.SurfaceSessionID)
			if instanceID != "" {
				if err := a.relay.SendCommand(instanceID, *event.Command); err != nil {
					log.Printf("relay send command failed: instance=%s kind=%s err=%v", instanceID, event.Command.Kind, err)
				}
			}
			continue
		}
		chatID := a.service.SurfaceChatID(event.SurfaceSessionID)
		if chatID == "" {
			continue
		}
		operations := a.projector.Project(chatID, event)
		applyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := a.gateway.Apply(applyCtx, operations)
		cancel()
		if err != nil {
			log.Printf("gateway apply failed: chat=%s event=%s err=%v", chatID, event.Kind, err)
		}
	}
}

func (a *App) nextCommandID() string {
	return "cmd-" + strconv.FormatUint(atomic.AddUint64(&a.commandSeq, 1), 10)
}

func (a *App) handleStatus(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	payload := struct {
		Instances []*state.InstanceRecord `json:"instances"`
	}{
		Instances: a.service.Instances(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
