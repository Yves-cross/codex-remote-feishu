package codex

import (
	"encoding/json"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func TestTranslateThreadCompactStart(t *testing.T) {
	tr := NewTranslator("inst-1")
	commands, err := tr.TranslateCommand(agentproto.Command{
		Kind:   agentproto.CommandThreadCompactStart,
		Origin: agentproto.Origin{Surface: "surface-1"},
		Target: agentproto.Target{ThreadID: "thread-1"},
	})
	if err != nil {
		t.Fatalf("translate compact command: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected one compact command, got %#v", commands)
	}
	var payload map[string]any
	if err := json.Unmarshal(commands[0], &payload); err != nil {
		t.Fatalf("unmarshal compact command: %v", err)
	}
	if payload["method"] != "thread/compact/start" {
		t.Fatalf("expected thread/compact/start payload, got %#v", payload)
	}
	params, _ := payload["params"].(map[string]any)
	if params["threadId"] != "thread-1" {
		t.Fatalf("unexpected compact params: %#v", params)
	}
}

func TestObserveTurnStartedAfterCompactUsesRemoteSurfaceInitiator(t *testing.T) {
	tr := NewTranslator("inst-1")
	_, err := tr.TranslateCommand(agentproto.Command{
		Kind:   agentproto.CommandThreadCompactStart,
		Origin: agentproto.Origin{Surface: "surface-1"},
		Target: agentproto.Target{ThreadID: "thread-1"},
	})
	if err != nil {
		t.Fatalf("translate compact command: %v", err)
	}

	result, err := tr.ObserveServer([]byte(`{"method":"turn/started","params":{"threadId":"thread-1","turn":{"id":"turn-compact-1"}}}`))
	if err != nil {
		t.Fatalf("observe turn started: %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected one event, got %#v", result.Events)
	}
	if result.Events[0].Initiator.Kind != agentproto.InitiatorRemoteSurface || result.Events[0].Initiator.SurfaceSessionID != "surface-1" {
		t.Fatalf("unexpected initiator after compact start: %#v", result.Events[0].Initiator)
	}
}

func TestObserveCompactStartErrorEmitsSystemError(t *testing.T) {
	tr := NewTranslator("inst-1")
	_, err := tr.TranslateCommand(agentproto.Command{
		Kind:   agentproto.CommandThreadCompactStart,
		Origin: agentproto.Origin{Surface: "surface-1"},
		Target: agentproto.Target{ThreadID: "thread-1"},
	})
	if err != nil {
		t.Fatalf("translate compact command: %v", err)
	}

	result, err := tr.ObserveServer([]byte(`{"id":"relay-thread-compact-start-0","error":{"message":"thread busy"}}`))
	if err != nil {
		t.Fatalf("observe compact error: %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].Kind != agentproto.EventSystemError || result.Events[0].Problem == nil {
		t.Fatalf("expected compact start system error, got %#v", result.Events)
	}
	if result.Events[0].Problem.Operation != "thread.compact.start" || result.Events[0].Problem.SurfaceSessionID != "surface-1" {
		t.Fatalf("unexpected compact start problem: %#v", result.Events[0].Problem)
	}
}
