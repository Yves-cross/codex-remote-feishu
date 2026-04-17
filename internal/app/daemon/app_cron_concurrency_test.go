package daemon

import (
	"testing"
	"time"

	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

func TestCronSchedulerAllowsSecondRunWithinConcurrencyLimit(t *testing.T) {
	workspace := t.TempDir()
	app := New(":0", ":0", nil, agentproto.ServerIdentity{StartedAt: time.Now().UTC()})
	setCronGatewayLookup(app, "gateway-1", "app-1")
	app.headlessRuntime.Paths.StateDir = t.TempDir()
	app.cronLoaded = true
	app.cronState = &cronStateFile{
		GatewayID: "gateway-1",
		Bitable: &cronBitableState{
			AppToken: "app-1",
			Tables: cronTableIDs{
				Tasks: "tbl-tasks",
				Runs:  "tbl-runs",
			},
		},
		Jobs: []cronJobState{{
			RecordID:        "rec-task-1",
			Name:            "Nightly",
			ScheduleType:    cronScheduleTypeInterval,
			IntervalMinutes: 5,
			WorkspaceKey:    workspace,
			Prompt:          "check CI",
			MaxConcurrency:  2,
			TimeoutMinutes:  15,
			NextRunAt:       time.Now().Add(-time.Minute),
		}},
	}
	app.SetHeadlessRuntime(HeadlessRuntimeConfig{
		BinaryPath: "/tmp/codex-remote",
		ConfigPath: "/tmp/config.json",
		Paths: relayruntime.Paths{
			StateDir: app.headlessRuntime.Paths.StateDir,
		},
	})
	app.cronRuns["inst-running"] = &cronRunState{
		InstanceID:     "inst-running",
		JobRecordID:    "rec-task-1",
		JobName:        "Nightly",
		WorkspaceKey:   workspace,
		TriggeredAt:    time.Now().Add(-2 * time.Minute),
		TimeoutMinutes: 15,
	}
	app.cronJobActiveRuns[cronJobActiveKey("rec-task-1", "Nightly")] = map[string]struct{}{"inst-running": {}}

	var launches int
	app.startHeadless = func(opts relayruntime.HeadlessLaunchOptions) (int, error) {
		launches++
		return 4321, nil
	}

	app.mu.Lock()
	app.maybeScheduleCronJobsLocked(time.Now())
	app.mu.Unlock()

	if launches != 1 {
		t.Fatalf("launches = %d, want 1", launches)
	}
	if len(app.cronRuns) != 2 {
		t.Fatalf("cronRuns = %#v, want two active runs", app.cronRuns)
	}
	active := app.cronJobActiveRuns[cronJobActiveKey("rec-task-1", "Nightly")]
	if len(active) != 2 {
		t.Fatalf("active cron runs = %#v, want two instances", active)
	}
}

func TestCronJobFromRecordParsesAndNormalizesConcurrency(t *testing.T) {
	now := time.Now()
	workspaces := map[string]cronWorkspaceRow{
		"rec-workspace-1": {Key: "/tmp/project", Name: "project", Status: "可用"},
	}
	cases := []struct {
		name      string
		value     any
		wantLimit int
	}{
		{name: "explicit", value: "3", wantLimit: 3},
		{name: "invalid", value: "oops", wantLimit: 1},
		{name: "empty", value: "", wantLimit: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := &larkbitable.AppTableRecord{
				RecordId: stringPtr("rec-task-1"),
				Fields: map[string]any{
					"任务名":                    "Nightly",
					"启用":                     true,
					"调度类型":                   cronScheduleTypeInterval,
					"间隔":                     "15分钟",
					"工作区":                    []any{"rec-workspace-1"},
					"提示词":                    "check CI",
					cronTaskConcurrencyField: tc.value,
				},
			}
			job, skip, err := cronJobFromRecord(record, workspaces, now)
			if err != nil {
				t.Fatalf("cronJobFromRecord: %v", err)
			}
			if skip {
				t.Fatalf("expected enabled job, got skip")
			}
			if job.MaxConcurrency != tc.wantLimit {
				t.Fatalf("MaxConcurrency = %d, want %d", job.MaxConcurrency, tc.wantLimit)
			}
		})
	}
}
