package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestIngressPumpRoundRobinKeepsPerInstanceFIFO(t *testing.T) {
	pump := newIngressPump()
	for _, item := range []ingressWorkItem{
		{
			instanceID: "inst-a",
			kind:       ingressWorkEvents,
			events:     []agentproto.Event{{Kind: agentproto.EventItemDelta, ItemID: "a-1"}},
		},
		{
			instanceID: "inst-a",
			kind:       ingressWorkEvents,
			events:     []agentproto.Event{{Kind: agentproto.EventItemDelta, ItemID: "a-2"}},
		},
		{
			instanceID: "inst-b",
			kind:       ingressWorkEvents,
			events:     []agentproto.Event{{Kind: agentproto.EventItemDelta, ItemID: "b-1"}},
		},
	} {
		if err := pump.Enqueue(item); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		pump.Close()
		pump.Wait()
	}()

	gotCh := make(chan string, 3)
	go func() {
		err := pump.Run(ctx, func(item ingressWorkItem) {
			gotCh <- item.instanceID + ":" + item.events[0].ItemID
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("pump run: %v", err)
		}
	}()

	want := []string{"inst-a:a-1", "inst-b:b-1", "inst-a:a-2"}
	for _, expected := range want {
		select {
		case got := <-gotCh:
			if got != expected {
				t.Fatalf("processing order = %q, want %q", got, expected)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %s", expected)
		}
	}
}

func TestIngressPumpEnqueueDoesNotBlockOnSlowHandler(t *testing.T) {
	pump := newIngressPump()
	if err := pump.Enqueue(ingressWorkItem{
		instanceID: "inst-a",
		kind:       ingressWorkEvents,
		events:     []agentproto.Event{{Kind: agentproto.EventItemDelta, ItemID: "a-1"}},
	}); err != nil {
		t.Fatalf("enqueue first item: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		pump.Close()
		pump.Wait()
	}()

	started := make(chan struct{})
	release := make(chan struct{})
	processed := make(chan string, 2)
	go func() {
		err := pump.Run(ctx, func(item ingressWorkItem) {
			if item.events[0].ItemID == "a-1" {
				close(started)
				<-release
			}
			processed <- item.events[0].ItemID
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("pump run: %v", err)
		}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for slow handler to start")
	}

	enqueueDone := make(chan error, 1)
	go func() {
		enqueueDone <- pump.Enqueue(ingressWorkItem{
			instanceID: "inst-a",
			kind:       ingressWorkEvents,
			events:     []agentproto.Event{{Kind: agentproto.EventItemDelta, ItemID: "a-2"}},
		})
	}()

	select {
	case err := <-enqueueDone:
		if err != nil {
			t.Fatalf("enqueue second item: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("enqueue blocked on slow handler")
	}

	close(release)

	for _, expected := range []string{"a-1", "a-2"} {
		select {
		case got := <-processed:
			if got != expected {
				t.Fatalf("processed item = %s, want %s", got, expected)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for processed item %s", expected)
		}
	}
}

func TestAppRelayCallbacksUseIngressPump(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.startIngressPump(ctx, nil)
	defer app.stopIngressPump()

	app.service.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "droid",
		WorkspaceRoot: "/data/dl/droid",
		WorkspaceKey:  "/data/dl/droid",
		ShortName:     "droid",
		Online:        true,
		Threads:       map[string]*state.ThreadRecord{},
	})

	app.enqueueEvents(context.Background(), "inst-1", []agentproto.Event{{
		Kind: agentproto.EventThreadsSnapshot,
		Threads: []agentproto.ThreadSnapshotRecord{{
			ThreadID: "thread-1",
			Name:     "修复登录流程",
			CWD:      "/data/dl/droid",
			Loaded:   true,
		}},
	}})

	waitForDaemonCondition(t, 2*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		inst := app.service.Instance("inst-1")
		return inst != nil && inst.Threads["thread-1"] != nil
	})
}

func waitForDaemonCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !check() {
		t.Fatal("condition not satisfied before timeout")
	}
}

func TestDaemonShutdownWithoutIngressPumpStartReturns(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.Shutdown(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown blocked without ingress pump start")
	}
}

func TestIngressPumpCloseRejectsNewWork(t *testing.T) {
	pump := newIngressPump()
	pump.Close()
	if err := pump.Enqueue(ingressWorkItem{
		instanceID: "inst-a",
		kind:       ingressWorkDisconnect,
	}); !errors.Is(err, errIngressPumpClosed) {
		t.Fatalf("expected closed pump error, got %v", err)
	}
}

func TestAppStopIngressPumpWaitsForRunner(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, agentproto.ServerIdentity{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.startIngressPump(ctx, nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.stopIngressPump()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stopIngressPump blocked")
	}
}

func TestIngressPumpRunReturnsOnContextCancel(t *testing.T) {
	pump := newIngressPump()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pump.Run(ctx, func(ingressWorkItem) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context canceled", err)
	}
}

func TestIngressPumpConcurrentEnqueueIsSafe(t *testing.T) {
	pump := newIngressPump()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		pump.Close()
		pump.Wait()
	}()

	var wg sync.WaitGroup
	gotCh := make(chan string, 4)
	go func() {
		err := pump.Run(ctx, func(item ingressWorkItem) {
			gotCh <- item.instanceID + ":" + item.kind.String()
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("pump run: %v", err)
		}
	}()

	items := []ingressWorkItem{
		{instanceID: "inst-a", kind: ingressWorkDisconnect},
		{instanceID: "inst-b", kind: ingressWorkDisconnect},
		{instanceID: "inst-a", kind: ingressWorkDisconnect},
		{instanceID: "inst-b", kind: ingressWorkDisconnect},
	}
	wg.Add(len(items))
	for _, item := range items {
		go func(item ingressWorkItem) {
			defer wg.Done()
			if err := pump.Enqueue(item); err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}(item)
	}
	wg.Wait()

	for range items {
		select {
		case <-gotCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent items")
		}
	}
}

func (k ingressWorkKind) String() string {
	return string(k)
}
