package codex

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func TestInternalHelperThreadLifecycleIsAnnotatedInsteadOfSuppressed(t *testing.T) {
	tr := NewTranslator("inst-1")
	if _, err := tr.ObserveClient([]byte(`{"id":"helper-thread-1","method":"thread/start","params":{"cwd":"/tmp/project","approvalPolicy":"never","sandbox":"read-only","ephemeral":true,"persistExtendedHistory":false}}`)); err != nil {
		t.Fatalf("observe helper thread start: %v", err)
	}

	result, err := tr.ObserveServer([]byte(`{"id":"helper-thread-1","result":{"thread":{"id":"helper-thread"}}}`))
	if err != nil {
		t.Fatalf("observe helper thread response: %v", err)
	}
	if result.Suppress {
		t.Fatalf("helper thread response must still reach parent stdout: %#v", result)
	}
	if len(result.Events) != 0 {
		t.Fatalf("helper thread response should not emit canonical events, got %#v", result.Events)
	}

	started, err := tr.ObserveServer([]byte(`{"method":"thread/started","params":{"thread":{"id":"helper-thread","cwd":"/tmp/project"}}}`))
	if err != nil {
		t.Fatalf("observe helper thread started: %v", err)
	}
	if started.Suppress {
		t.Fatalf("helper thread notification must still reach parent stdout: %#v", started)
	}
	if len(started.Events) != 1 {
		t.Fatalf("expected annotated helper thread event, got %#v", started.Events)
	}
	if started.Events[0].Kind != agentproto.EventThreadDiscovered ||
		started.Events[0].TrafficClass != agentproto.TrafficClassInternalHelper ||
		started.Events[0].Initiator.Kind != agentproto.InitiatorInternalHelper {
		t.Fatalf("unexpected helper thread event: %#v", started.Events[0])
	}
}

func TestStructuredHelperTurnLifecycleIsAnnotatedInsteadOfSuppressed(t *testing.T) {
	tr := NewTranslator("inst-1")
	if _, err := tr.ObserveClient([]byte(`{"id":"helper-thread-1","method":"thread/start","params":{"cwd":"/tmp/project","approvalPolicy":"never","sandbox":"read-only","ephemeral":true,"persistExtendedHistory":false}}`)); err != nil {
		t.Fatalf("observe helper thread start: %v", err)
	}
	if _, err := tr.ObserveServer([]byte(`{"id":"helper-thread-1","result":{"thread":{"id":"helper-thread"}}}`)); err != nil {
		t.Fatalf("observe helper thread response: %v", err)
	}
	if _, err := tr.ObserveServer([]byte(`{"method":"thread/started","params":{"thread":{"id":"helper-thread","cwd":"/tmp/project"}}}`)); err != nil {
		t.Fatalf("observe helper thread started: %v", err)
	}
	if _, err := tr.ObserveClient([]byte(`{"id":"helper-turn-1","method":"turn/start","params":{"threadId":"helper-thread","cwd":"/tmp/project","outputSchema":{"type":"object","properties":{"title":{"type":"string"}}}}}`)); err != nil {
		t.Fatalf("observe helper turn start: %v", err)
	}
	result, err := tr.ObserveServer([]byte(`{"id":"helper-turn-1","result":{"turn":{"id":"turn-helper"}}}`))
	if err != nil {
		t.Fatalf("observe helper turn response: %v", err)
	}
	if result.Suppress {
		t.Fatalf("helper turn response must still reach parent stdout: %#v", result)
	}
	if len(result.Events) != 0 {
		t.Fatalf("helper turn response should not emit canonical events, got %#v", result.Events)
	}

	started, err := tr.ObserveServer([]byte(`{"method":"turn/started","params":{"threadId":"helper-thread","turn":{"id":"turn-helper"}}}`))
	if err != nil {
		t.Fatalf("observe helper turn started: %v", err)
	}
	if started.Suppress {
		t.Fatalf("helper turn started must still reach parent stdout: %#v", started)
	}
	if len(started.Events) != 1 {
		t.Fatalf("expected annotated helper turn started event, got %#v", started.Events)
	}
	if started.Events[0].Kind != agentproto.EventTurnStarted ||
		started.Events[0].TrafficClass != agentproto.TrafficClassInternalHelper ||
		started.Events[0].Initiator.Kind != agentproto.InitiatorInternalHelper {
		t.Fatalf("unexpected helper turn started event: %#v", started.Events[0])
	}

	delta, err := tr.ObserveServer([]byte(`{"method":"item/agentMessage/delta","params":{"threadId":"helper-thread","turnId":"turn-helper","itemId":"item-1","delta":"{\"title\":\"ok\"}"}}`))
	if err != nil {
		t.Fatalf("observe helper delta: %v", err)
	}
	if delta.Suppress {
		t.Fatalf("helper item delta must still reach parent stdout: %#v", delta)
	}
	if len(delta.Events) != 1 {
		t.Fatalf("expected annotated helper delta event, got %#v", delta.Events)
	}
	if delta.Events[0].Kind != agentproto.EventItemDelta ||
		delta.Events[0].TrafficClass != agentproto.TrafficClassInternalHelper ||
		delta.Events[0].Initiator.Kind != agentproto.InitiatorInternalHelper {
		t.Fatalf("unexpected helper delta event: %#v", delta.Events[0])
	}

	completed, err := tr.ObserveServer([]byte(`{"method":"turn/completed","params":{"threadId":"helper-thread","turn":{"id":"turn-helper","status":"completed"}}}`))
	if err != nil {
		t.Fatalf("observe helper turn completed: %v", err)
	}
	if completed.Suppress {
		t.Fatalf("helper turn completed must still reach parent stdout: %#v", completed)
	}
	if len(completed.Events) != 1 {
		t.Fatalf("expected annotated helper turn completed event, got %#v", completed.Events)
	}
	if completed.Events[0].Kind != agentproto.EventTurnCompleted ||
		completed.Events[0].TrafficClass != agentproto.TrafficClassInternalHelper ||
		completed.Events[0].Initiator.Kind != agentproto.InitiatorInternalHelper {
		t.Fatalf("unexpected helper turn completed event: %#v", completed.Events[0])
	}
}

func TestHelperTurnOnSameThreadDoesNotSuppressRemoteTurn(t *testing.T) {
	tr := NewTranslator("inst-1")
	if _, err := tr.ObserveClient([]byte(`{"id":"helper-turn-1","method":"turn/start","params":{"threadId":"thread-1","cwd":"/tmp/project","outputSchema":{"type":"object","properties":{"title":{"type":"string"}}}}}`)); err != nil {
		t.Fatalf("observe helper turn start: %v", err)
	}

	commands, err := tr.TranslateCommand(agentproto.Command{
		Kind:   agentproto.CommandPromptSend,
		Origin: agentproto.Origin{Surface: "surface-1", ChatID: "chat-1"},
		Target: agentproto.Target{ThreadID: "thread-1", CWD: "/tmp/project"},
		Prompt: agentproto.Prompt{Inputs: []agentproto.Input{{Type: agentproto.InputText, Text: "hello"}}},
	})
	if err != nil {
		t.Fatalf("translate remote command: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected one remote turn/start command, got %d", len(commands))
	}

	if _, err := tr.ObserveServer([]byte(`{"id":"helper-turn-1","result":{"turn":{"id":"turn-helper"}}}`)); err != nil {
		t.Fatalf("observe helper turn response: %v", err)
	}

	var remoteTurnStart map[string]any
	if err := json.Unmarshal(commands[0], &remoteTurnStart); err != nil {
		t.Fatalf("unmarshal remote turn/start: %v", err)
	}
	remoteRequestID, _ := remoteTurnStart["id"].(string)
	if remoteRequestID == "" {
		t.Fatalf("expected remote turn/start request id, got %#v", remoteTurnStart)
	}

	response, err := tr.ObserveServer([]byte(fmt.Sprintf(`{"id":%q,"result":{"turn":{"id":"turn-remote"}}}`, remoteRequestID)))
	if err != nil {
		t.Fatalf("observe remote turn response: %v", err)
	}
	if !response.Suppress {
		t.Fatalf("expected relay-owned remote turn/start response to stay suppressed, got %#v", response)
	}

	started, err := tr.ObserveServer([]byte(`{"method":"turn/started","params":{"threadId":"thread-1","turn":{"id":"turn-remote"}}}`))
	if err != nil {
		t.Fatalf("observe remote turn started: %v", err)
	}
	if len(started.Events) != 1 {
		t.Fatalf("expected one remote turn started event, got %#v", started.Events)
	}
	if started.Events[0].Kind != agentproto.EventTurnStarted || started.Events[0].Initiator.Kind != agentproto.InitiatorRemoteSurface {
		t.Fatalf("expected remote turn started event, got %#v", started.Events[0])
	}

	item, err := tr.ObserveServer([]byte(`{"method":"item/completed","params":{"threadId":"thread-1","turnId":"turn-remote","item":{"id":"item-1","type":"agentMessage","text":"您好"}}}`))
	if err != nil {
		t.Fatalf("observe remote item completed: %v", err)
	}
	if len(item.Events) != 1 || item.Events[0].ItemKind != "agent_message" {
		t.Fatalf("expected remote assistant item event, got %#v", item.Events)
	}

	completed, err := tr.ObserveServer([]byte(`{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-remote","status":"completed"}}}`))
	if err != nil {
		t.Fatalf("observe remote turn completed: %v", err)
	}
	if len(completed.Events) != 1 {
		t.Fatalf("expected one remote turn completed event, got %#v", completed.Events)
	}
	if completed.Events[0].Kind != agentproto.EventTurnCompleted || completed.Events[0].Initiator.Kind != agentproto.InitiatorRemoteSurface {
		t.Fatalf("expected remote turn completed event, got %#v", completed.Events[0])
	}
}

func TestInternalHelperThreadMarkerDoesNotPoisonLaterRemoteTurnOnSameThread(t *testing.T) {
	tr := NewTranslator("inst-1")

	if _, err := tr.ObserveClient([]byte(`{"method":"thread/resume","params":{"threadId":"thread-1","cwd":"/tmp/project"}}`)); err != nil {
		t.Fatalf("observe active thread resume: %v", err)
	}

	if _, err := tr.ObserveClient([]byte(`{"id":"helper-thread-1","method":"thread/start","params":{"cwd":"/tmp/project","approvalPolicy":"never","sandbox":"read-only","ephemeral":true,"persistExtendedHistory":false}}`)); err != nil {
		t.Fatalf("observe helper thread start: %v", err)
	}
	if _, err := tr.ObserveServer([]byte(`{"id":"helper-thread-1","result":{"thread":{"id":"thread-1"}}}`)); err != nil {
		t.Fatalf("observe helper thread response: %v", err)
	}
	if _, err := tr.ObserveServer([]byte(`{"method":"thread/started","params":{"thread":{"id":"thread-1","cwd":"/tmp/project"}}}`)); err != nil {
		t.Fatalf("observe helper thread started: %v", err)
	}

	commands, err := tr.TranslateCommand(agentproto.Command{
		Kind:   agentproto.CommandPromptSend,
		Origin: agentproto.Origin{Surface: "surface-1", ChatID: "chat-1"},
		Target: agentproto.Target{ThreadID: "thread-1", CWD: "/tmp/project"},
		Prompt: agentproto.Prompt{Inputs: []agentproto.Input{{Type: agentproto.InputText, Text: "hello"}}},
	})
	if err != nil {
		t.Fatalf("translate remote command: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected one remote turn/start command, got %d", len(commands))
	}

	started, err := tr.ObserveServer([]byte(`{"method":"turn/started","params":{"threadId":"thread-1","turn":{"id":"turn-remote"}}}`))
	if err != nil {
		t.Fatalf("observe remote turn started: %v", err)
	}
	if len(started.Events) != 1 {
		t.Fatalf("expected one remote turn started event, got %#v", started.Events)
	}
	if started.Events[0].TrafficClass != agentproto.TrafficClassPrimary || started.Events[0].Initiator.Kind != agentproto.InitiatorRemoteSurface {
		t.Fatalf("expected remote primary turn started event, got %#v", started.Events[0])
	}

	item, err := tr.ObserveServer([]byte(`{"method":"item/completed","params":{"threadId":"thread-1","turnId":"turn-remote","item":{"id":"item-1","type":"agentMessage","text":"您好"}}}`))
	if err != nil {
		t.Fatalf("observe remote item completed: %v", err)
	}
	if len(item.Events) != 1 {
		t.Fatalf("expected one remote item event, got %#v", item.Events)
	}
	if item.Events[0].TrafficClass != agentproto.TrafficClassPrimary || item.Events[0].Initiator.Kind != agentproto.InitiatorRemoteSurface {
		t.Fatalf("expected remote primary item event, got %#v", item.Events[0])
	}

	completed, err := tr.ObserveServer([]byte(`{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-remote","status":"completed"}}}`))
	if err != nil {
		t.Fatalf("observe remote turn completed: %v", err)
	}
	if len(completed.Events) != 1 {
		t.Fatalf("expected one remote turn completed event, got %#v", completed.Events)
	}
	if completed.Events[0].TrafficClass != agentproto.TrafficClassPrimary || completed.Events[0].Initiator.Kind != agentproto.InitiatorRemoteSurface {
		t.Fatalf("expected remote primary turn completed event, got %#v", completed.Events[0])
	}
}

func TestObserveServerCodexProblemAttachesToInterruptedTurn(t *testing.T) {
	tr := NewTranslator("inst-1")

	commands, err := tr.TranslateCommand(agentproto.Command{
		Kind:   agentproto.CommandPromptSend,
		Origin: agentproto.Origin{Surface: "surface-1", ChatID: "chat-1"},
		Target: agentproto.Target{ThreadID: "thread-1", CWD: "/tmp/project"},
		Prompt: agentproto.Prompt{Inputs: []agentproto.Input{{Type: agentproto.InputText, Text: "hello"}}},
	})
	if err != nil {
		t.Fatalf("translate remote command: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected one remote turn/start command, got %d", len(commands))
	}

	var remoteTurnStart map[string]any
	if err := json.Unmarshal(commands[0], &remoteTurnStart); err != nil {
		t.Fatalf("unmarshal remote turn/start: %v", err)
	}
	remoteRequestID, _ := remoteTurnStart["id"].(string)
	if remoteRequestID == "" {
		t.Fatalf("expected remote request id, got %#v", remoteTurnStart)
	}

	response, err := tr.ObserveServer([]byte(fmt.Sprintf(`{"id":%q,"result":{"turn":{"id":"turn-remote"}}}`, remoteRequestID)))
	if err != nil {
		t.Fatalf("observe remote turn response: %v", err)
	}
	if !response.Suppress {
		t.Fatalf("expected relay-owned remote turn/start response to stay suppressed, got %#v", response)
	}

	if _, err := tr.ObserveServer([]byte(`{"method":"turn/started","params":{"threadId":"thread-1","turn":{"id":"turn-remote"}}}`)); err != nil {
		t.Fatalf("observe remote turn started: %v", err)
	}

	problem, err := tr.ObserveServer([]byte(`{"method":"error","params":{"error":{"message":"Reconnecting... 1/5","codexErrorInfo":{"responseStreamDisconnected":{"httpStatusCode":null}},"additionalDetails":"stream disconnected before completion: stream closed before response.completed"},"willRetry":true,"threadId":"thread-1","turnId":"turn-remote"}}`))
	if err != nil {
		t.Fatalf("observe codex problem event: %v", err)
	}
	if len(problem.Events) != 0 {
		t.Fatalf("expected turn-bound codex problem to wait for terminal turn event, got %#v", problem.Events)
	}

	completed, err := tr.ObserveServer([]byte(`{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-remote","status":"interrupted","error":null}}}`))
	if err != nil {
		t.Fatalf("observe remote turn completed: %v", err)
	}
	if len(completed.Events) != 1 {
		t.Fatalf("expected one remote turn completed event, got %#v", completed.Events)
	}
	event := completed.Events[0]
	if event.Kind != agentproto.EventTurnCompleted || event.Initiator.Kind != agentproto.InitiatorRemoteSurface {
		t.Fatalf("expected remote turn completed event, got %#v", event)
	}
	if event.ErrorMessage != "stream disconnected before completion: stream closed before response.completed" {
		t.Fatalf("expected precise error message from codex problem, got %#v", event)
	}
	if event.Problem == nil || event.Problem.Code != "responseStreamDisconnected" || !event.Problem.Retryable {
		t.Fatalf("expected attached codex problem metadata, got %#v", event.Problem)
	}
}

func TestObserveServerCodexProblemWithoutTurnEmitsSystemError(t *testing.T) {
	tr := NewTranslator("inst-1")

	result, err := tr.ObserveServer([]byte(`{"method":"error","params":{"error":{"message":"unexpected status 503 Service Unavailable","codexErrorInfo":"other","additionalDetails":"request id: abc"},"willRetry":false,"threadId":"thread-1"}}`))
	if err != nil {
		t.Fatalf("observe codex problem event: %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected one system error event, got %#v", result.Events)
	}
	event := result.Events[0]
	if event.Kind != agentproto.EventSystemError || event.Problem == nil {
		t.Fatalf("expected system error event with problem payload, got %#v", event)
	}
	if event.Problem.Code != "other" || event.Problem.ThreadID != "thread-1" || event.Problem.Retryable {
		t.Fatalf("unexpected problem payload: %#v", event.Problem)
	}
}
