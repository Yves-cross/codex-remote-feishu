package relayws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"

	"github.com/gorilla/websocket"
)

func TestClientServerCommandAndEventFlow(t *testing.T) {
	helloCh := make(chan agentproto.Hello, 1)
	eventsCh := make(chan []agentproto.Event, 1)
	commandCh := make(chan agentproto.Command, 1)

	server := NewServer(ServerCallbacks{
		OnHello: func(_ context.Context, hello agentproto.Hello) {
			helloCh <- hello
		},
		OnEvents: func(_ context.Context, _ string, events []agentproto.Event) {
			eventsCh <- events
		},
	})
	defer server.Close()

	mux := http.NewServeMux()
	mux.Handle("/ws/agent", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	client := NewClient(wsURL, agentproto.Hello{
		Protocol: agentproto.WireProtocol,
		Instance: agentproto.InstanceHello{
			InstanceID:  "inst-1",
			DisplayName: "droid",
		},
	}, ClientCallbacks{
		OnCommand: func(_ context.Context, command agentproto.Command) error {
			commandCh <- command
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = client.Run(ctx)
	}()
	defer client.Close()

	select {
	case hello := <-helloCh:
		if hello.Instance.InstanceID != "inst-1" {
			t.Fatalf("unexpected hello: %#v", hello)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for hello")
	}

	if err := server.SendCommand("inst-1", agentproto.Command{
		CommandID: "cmd-1",
		Kind:      agentproto.CommandThreadsRefresh,
	}); err != nil {
		t.Fatalf("send command: %v", err)
	}

	select {
	case command := <-commandCh:
		if command.Kind != agentproto.CommandThreadsRefresh {
			t.Fatalf("unexpected command: %#v", command)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for command")
	}

	if err := client.SendEvents([]agentproto.Event{{Kind: agentproto.EventThreadFocused, ThreadID: "thread-1"}}); err != nil {
		t.Fatalf("send events: %v", err)
	}
	select {
	case events := <-eventsCh:
		if len(events) != 1 || events[0].Kind != agentproto.EventThreadFocused {
			t.Fatalf("unexpected events: %#v", events)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for events")
	}
}

func TestClientNormalizesDefaultRelayPath(t *testing.T) {
	helloCh := make(chan agentproto.Hello, 1)
	server := NewServer(ServerCallbacks{
		OnHello: func(_ context.Context, hello agentproto.Hello) {
			helloCh <- hello
		},
	})
	defer server.Close()

	mux := http.NewServeMux()
	mux.Handle("/ws/agent", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	client := NewClient(wsURL, agentproto.Hello{
		Protocol: agentproto.WireProtocol,
		Instance: agentproto.InstanceHello{
			InstanceID:  "inst-normalized",
			DisplayName: "droid",
		},
	}, ClientCallbacks{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = client.Run(ctx)
	}()
	defer client.Close()

	select {
	case hello := <-helloCh:
		if hello.Instance.InstanceID != "inst-normalized" {
			t.Fatalf("unexpected hello: %#v", hello)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for hello over normalized path")
	}
}

func TestClientReceivesWelcomeServerIdentity(t *testing.T) {
	helloCh := make(chan agentproto.Hello, 1)
	welcomeCh := make(chan agentproto.Welcome, 1)
	server := NewServer(ServerCallbacks{
		OnHello: func(_ context.Context, hello agentproto.Hello) {
			helloCh <- hello
		},
	})
	server.SetServerIdentity(agentproto.ServerIdentity{
		BinaryIdentity: agentproto.BinaryIdentity{
			Product:          "codex-remote",
			Version:          "1.0.0",
			BuildFingerprint: "fp-1",
			BinaryPath:       "/tmp/codex-remote",
		},
		PID: 12345,
	})
	defer server.Close()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	client := NewClient(wsURL, agentproto.Hello{
		Protocol: agentproto.WireProtocol,
		Instance: agentproto.InstanceHello{
			InstanceID: "inst-welcome",
		},
	}, ClientCallbacks{
		OnWelcome: func(_ context.Context, welcome agentproto.Welcome) error {
			welcomeCh <- welcome
			return context.Canceled
		},
	})

	if err := client.Run(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("client run: %v", err)
	}

	select {
	case hello := <-helloCh:
		if hello.Instance.InstanceID != "inst-welcome" {
			t.Fatalf("unexpected hello: %#v", hello)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for hello")
	}

	select {
	case welcome := <-welcomeCh:
		if welcome.Server == nil || welcome.Server.BuildFingerprint != "fp-1" || welcome.Server.PID != 12345 {
			t.Fatalf("unexpected welcome server identity: %#v", welcome)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for welcome")
	}
}

func TestServerCallbackContextSurvivesAfterUpgradeHandlerReturns(t *testing.T) {
	callbackErrCh := make(chan error, 2)
	server := NewServer(ServerCallbacks{
		OnHello: func(ctx context.Context, _ agentproto.Hello) {
			callbackErrCh <- ctx.Err()
		},
		OnEvents: func(ctx context.Context, _ string, _ []agentproto.Event) {
			callbackErrCh <- ctx.Err()
		},
	})
	defer server.Close()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	helloPayload, err := agentproto.MarshalEnvelope(agentproto.Envelope{
		Type: agentproto.EnvelopeHello,
		Hello: &agentproto.Hello{
			Protocol: agentproto.WireProtocol,
			Instance: agentproto.InstanceHello{InstanceID: "inst-ctx"},
		},
	})
	if err != nil {
		t.Fatalf("marshal hello: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, helloPayload); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read welcome: %v", err)
	}

	select {
	case err := <-callbackErrCh:
		if err != nil {
			t.Fatalf("expected hello callback context to be alive, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for hello callback")
	}

	eventPayload, err := agentproto.MarshalEnvelope(agentproto.Envelope{
		Type: agentproto.EnvelopeEventBatch,
		EventBatch: &agentproto.EventBatch{
			InstanceID: "inst-ctx",
			Events:     []agentproto.Event{{Kind: agentproto.EventThreadFocused, ThreadID: "thread-1"}},
		},
	})
	if err != nil {
		t.Fatalf("marshal events: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, eventPayload); err != nil {
		t.Fatalf("write events: %v", err)
	}

	select {
	case err := <-callbackErrCh:
		if err != nil {
			t.Fatalf("expected event callback context to be alive, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for event callback")
	}
}
