package wrapper

import (
	"context"
	"io"
	"os/exec"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/codex"
	"github.com/kxn/codex-remote-feishu/internal/adapter/relayws"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/debuglog"
	"github.com/kxn/codex-remote-feishu/internal/execlaunch"
	relayruntime "github.com/kxn/codex-remote-feishu/internal/runtime"
)

type childSession struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.Reader
	stderr      io.Reader
	waitErr     chan error
	cancel      context.CancelFunc
	writeCancel context.CancelFunc
}

func (a *App) launchChildSession(ctx context.Context, rawLogger *debuglog.RawLogger, reportProblem func(agentproto.ErrorInfo)) (*childSession, error) {
	childCtx, childCancel := context.WithCancel(ctx)
	childArgs, childEnv := a.buildCodexChildLaunch(a.config.Args)
	cmd := execlaunch.CommandContext(childCtx, a.config.CodexRealBinary, childArgs...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Dir = a.config.WorkspaceRoot
	cmd.Env = childEnv

	childStdin, childStdout, childStderr, err := startChild(cmd)
	if err != nil {
		childCancel()
		return nil, err
	}
	a.debugf("child started: binary=%s pid=%d cwd=%s", a.config.CodexRealBinary, cmd.Process.Pid, a.config.WorkspaceRoot)

	bootstrappedStdout, err := a.bootstrapHeadlessCodex(childStdin, childStdout, rawLogger, reportProblem)
	if err != nil {
		childCancel()
		_ = cmd.Wait()
		return nil, err
	}

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- cmd.Wait()
	}()

	return &childSession{
		cmd:     cmd,
		stdin:   childStdin,
		stdout:  bootstrappedStdout,
		stderr:  childStderr,
		waitErr: waitErr,
		cancel:  childCancel,
	}, nil
}

func startChildSessionIO(ctx context.Context, session *childSession, parentStdout, parentStderr io.Writer, writeCh chan []byte, translator *codex.Translator, client *relayws.Client, commandResponses *commandResponseTracker, errCh chan<- error, debugf func(string, ...any), rawLogger *debuglog.RawLogger, reportProblem func(agentproto.ErrorInfo)) {
	if session == nil {
		return
	}
	writeCtx, writeCancel := context.WithCancel(ctx)
	session.writeCancel = writeCancel
	go writeLoop(writeCtx, session.stdin, writeCh, errCh, debugf, rawLogger, reportProblem)
	go stdoutLoop(ctx, session.stdout, parentStdout, writeCh, translator, client, commandResponses, errCh, debugf, rawLogger, reportProblem)
	go streamCopy(session.stderr, parentStderr, errCh)
}

func stopChildSession(session *childSession, debugf func(string, ...any)) {
	if session == nil {
		return
	}
	if session.writeCancel != nil {
		session.writeCancel()
	}
	if session.cmd != nil && session.cmd.Process != nil && session.cmd.Process.Pid > 0 {
		if err := relayruntime.TerminateProcess(session.cmd.Process.Pid, wrapperChildStopGrace); err != nil && debugf != nil {
			debugf("child stop failed: pid=%d err=%v", session.cmd.Process.Pid, err)
		}
	}
	if session.cancel != nil {
		session.cancel()
	}
	select {
	case <-session.waitErr:
	case <-time.After(wrapperChildWaitTimeout):
	}
}

func (a *App) restartChildSession(ctx context.Context, current *childSession, parentStdout, parentStderr io.Writer, writeCh chan []byte, client *relayws.Client, commandResponses *commandResponseTracker, errCh chan<- error, rawLogger *debuglog.RawLogger, reportProblem func(agentproto.ErrorInfo)) (*childSession, error) {
	next, err := a.launchChildSession(ctx, rawLogger, reportProblem)
	if err != nil {
		return nil, agentproto.ErrorInfo{
			Code:      "child_restart_launch_failed",
			Layer:     "wrapper",
			Stage:     "restart_child_launch",
			Operation: string(agentproto.CommandProcessChildRestart),
			Message:   "wrapper 无法拉起新的 Codex 子进程。",
			Details:   err.Error(),
		}
	}
	stopChildSession(current, a.debugf)
	startChildSessionIO(ctx, next, parentStdout, parentStderr, writeCh, a.translator, client, commandResponses, errCh, a.debugf, rawLogger, reportProblem)
	if err := a.restoreChildSessionContext(ctx, writeCh, commandResponses); err != nil {
		return next, err
	}
	return next, nil
}

func (a *App) restoreChildSessionContext(ctx context.Context, writeCh chan []byte, commandResponses *commandResponseTracker) error {
	frame, requestID, ok, err := a.translator.BuildChildRestartRestoreFrame()
	if err != nil {
		return agentproto.ErrorInfo{
			Code:      "child_restart_restore_build_failed",
			Layer:     "wrapper",
			Stage:     "restart_child_restore_build",
			Operation: string(agentproto.CommandProcessChildRestart),
			Message:   "wrapper 无法构造重启后的 thread 恢复请求。",
			Details:   err.Error(),
		}
	}
	if !ok {
		return nil
	}
	waitCh := commandResponses.Register(requestID, agentproto.ErrorInfo{
		Code:      "child_restart_restore_failed",
		Layer:     "wrapper",
		Stage:     "restart_child_restore_response",
		Operation: string(agentproto.CommandProcessChildRestart),
		Message:   "重启后的 Codex 子进程未能恢复先前 thread 上下文。",
	}, true)
	select {
	case writeCh <- frame:
	case <-ctx.Done():
		commandResponses.Cancel(requestID)
		a.translator.CancelChildRestartRestore(requestID)
		return ctx.Err()
	}
	if err := waitCommandResponse(ctx, waitCh, wrapperChildRestoreTimeout, agentproto.ErrorInfo{
		Code:      "child_restart_restore_timeout",
		Layer:     "wrapper",
		Stage:     "restart_child_restore_response",
		Operation: string(agentproto.CommandProcessChildRestart),
		Message:   "等待重启后的 Codex 子进程恢复 thread 上下文时超时。",
	}); err != nil {
		commandResponses.Cancel(requestID)
		a.translator.CancelChildRestartRestore(requestID)
		return err
	}
	return nil
}
