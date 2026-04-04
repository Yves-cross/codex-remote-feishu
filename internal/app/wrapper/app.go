package wrapper

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/codex"
	"github.com/kxn/codex-remote-feishu/internal/adapter/relayws"
	"github.com/kxn/codex-remote-feishu/internal/config"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

type App struct {
	config     Config
	translator *codex.Translator
}

type Config struct {
	RelayServerURL  string
	CodexRealBinary string
	NameMode        string
	Args            []string

	InstanceID    string
	DisplayName   string
	WorkspaceRoot string
	WorkspaceKey  string
	ShortName     string
	Version       string
	ChildProxyEnv []string
}

func LoadConfig(args []string) (Config, error) {
	loaded, err := config.LoadWrapperConfig()
	if err != nil {
		return Config{}, err
	}
	workspaceRoot, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}
	instanceID, err := generateInstanceID()
	if err != nil {
		return Config{}, err
	}
	shortName := filepath.Base(workspaceRoot)
	displayName := shortName
	if displayName == "." || displayName == "/" {
		displayName = workspaceRoot
	}
	return Config{
		RelayServerURL:  loaded.RelayServerURL,
		CodexRealBinary: loaded.CodexRealBinary,
		NameMode:        loaded.NameMode,
		Args:            args,
		InstanceID:      instanceID,
		DisplayName:     displayName,
		WorkspaceRoot:   workspaceRoot,
		WorkspaceKey:    workspaceRoot,
		ShortName:       shortName,
		Version:         "dev",
		ChildProxyEnv:   config.CaptureAndClearProxyEnv(),
	}, nil
}

func New(cfg Config) *App {
	return &App{
		config:     cfg,
		translator: codex.NewTranslator(cfg.InstanceID),
	}
}

func (a *App) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	childCtx, childCancel := context.WithCancel(ctx)
	defer childCancel()

	cmd := exec.CommandContext(childCtx, a.config.CodexRealBinary, a.config.Args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Dir = a.config.WorkspaceRoot
	cmd.Env = childEnvWithProxy(a.config.ChildProxyEnv)

	childStdin, childStdout, childStderr, err := startChild(cmd)
	if err != nil {
		return 1, err
	}

	writeCh := make(chan []byte, 128)
	errCh := make(chan error, 8)

	var client *relayws.Client
	client = relayws.NewClient(a.config.RelayServerURL, agentproto.Hello{
		Protocol: agentproto.WireProtocol,
		Instance: agentproto.InstanceHello{
			InstanceID:    a.config.InstanceID,
			DisplayName:   a.config.DisplayName,
			WorkspaceRoot: a.config.WorkspaceRoot,
			WorkspaceKey:  a.config.WorkspaceKey,
			ShortName:     a.config.ShortName,
			Version:       a.config.Version,
			PID:           os.Getpid(),
		},
		Capabilities: agentproto.Capabilities{ThreadsRefresh: true},
	}, relayws.ClientCallbacks{
		OnConnect: func(context.Context) error { return nil },
		OnCommand: func(ctx context.Context, command agentproto.Command) error {
			outbound, err := a.translator.TranslateCommand(command)
			if err != nil {
				return err
			}
			for _, line := range outbound {
				select {
				case writeCh <- line:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		},
	})

	go func() {
		if err := client.Run(ctx); err != nil && err != context.Canceled {
			errCh <- err
		}
	}()

	go writeLoop(ctx, childStdin, writeCh, errCh)
	go stdinLoop(ctx, stdin, writeCh, a.translator, client, errCh)
	go stdoutLoop(ctx, childStdout, stdout, writeCh, a.translator, client, errCh)
	go streamCopy(childStderr, stderr, errCh)

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- cmd.Wait()
	}()

	stopChild := func() {
		childCancel()
		select {
		case <-waitErr:
		case <-time.After(2 * time.Second):
		}
	}

	select {
	case err := <-waitErr:
		client.Close()
		if err == nil {
			return 0, nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	case err := <-errCh:
		client.Close()
		stopChild()
		if err == nil || err == context.Canceled {
			return 0, nil
		}
		return 1, err
	case <-ctx.Done():
		client.Close()
		stopChild()
		return 0, ctx.Err()
	}
}

func startChild(cmd *exec.Cmd) (io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	return stdin, stdout, stderr, nil
}

func stdinLoop(ctx context.Context, stdin io.Reader, writeCh chan<- []byte, translator *codex.Translator, client *relayws.Client, errCh chan<- error) {
	reader := bufio.NewReader(stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if result, parseErr := translator.ObserveClient(line); parseErr == nil {
				if sendErr := client.SendEvents(result.Events); sendErr != nil {
					log.Printf("relay send client events failed: %v", sendErr)
				}
			}
			select {
			case writeCh <- line:
			case <-ctx.Done():
				return
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return
		}
		errCh <- err
		return
	}
}

func stdoutLoop(ctx context.Context, childStdout io.Reader, parentStdout io.Writer, writeCh chan<- []byte, translator *codex.Translator, client *relayws.Client, errCh chan<- error) {
	reader := bufio.NewReader(childStdout)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			result, parseErr := translator.ObserveServer(line)
			if parseErr == nil {
				if sendErr := client.SendEvents(result.Events); sendErr != nil {
					log.Printf("relay send server events failed: %v", sendErr)
				}
				for _, followup := range result.OutboundToCodex {
					select {
					case writeCh <- followup:
					case <-ctx.Done():
						return
					}
				}
				if !result.Suppress {
					if _, writeErr := parentStdout.Write(line); writeErr != nil {
						errCh <- writeErr
						return
					}
				}
			} else {
				if _, writeErr := parentStdout.Write(line); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return
		}
		errCh <- err
		return
	}
}

func writeLoop(ctx context.Context, childStdin io.WriteCloser, writeCh <-chan []byte, errCh chan<- error) {
	defer childStdin.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case line := <-writeCh:
			if len(line) == 0 {
				continue
			}
			if _, err := childStdin.Write(line); err != nil {
				errCh <- err
				return
			}
		}
	}
}

func streamCopy(src io.Reader, dst io.Writer, errCh chan<- error) {
	if _, err := io.Copy(dst, src); err != nil && !strings.Contains(err.Error(), "file already closed") {
		errCh <- err
	}
}

func childEnvWithProxy(proxyEnv []string) []string {
	filtered := config.FilterEnvWithoutProxy(os.Environ())
	filtered = append(filtered, proxyEnv...)
	return filtered
}

func generateInstanceID() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("inst-%s", hex.EncodeToString(bytes[:])), nil
}
