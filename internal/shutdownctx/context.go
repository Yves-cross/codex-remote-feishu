package shutdownctx

import (
	"context"
	"sync"
	"sync/atomic"
)

type Mode string

const (
	ModeDefault      Mode = ""
	ModeConsoleClose Mode = "console_close"
)

type holder struct {
	mode atomic.Int32

	mu                  sync.Mutex
	consoleCloseHandler func()
	consoleCloseHandled bool
}

type contextKey struct{}

func WithHolder(parent context.Context) (context.Context, func(Mode)) {
	h := &holder{}
	ctx := context.WithValue(parent, contextKey{}, h)
	return ctx, func(mode Mode) {
		h.mode.Store(int32(modeToIndex(mode)))
	}
}

func ModeFrom(ctx context.Context) Mode {
	if ctx == nil {
		return ModeDefault
	}
	h, _ := ctx.Value(contextKey{}).(*holder)
	if h == nil {
		return ModeDefault
	}
	return modeFromIndex(int(h.mode.Load()))
}

func SetConsoleCloseHandler(ctx context.Context, handler func()) bool {
	if ctx == nil {
		return false
	}
	h, _ := ctx.Value(contextKey{}).(*holder)
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.consoleCloseHandler = handler
	return true
}

func HandleConsoleClose(ctx context.Context) {
	if ctx == nil {
		return
	}
	h, _ := ctx.Value(contextKey{}).(*holder)
	if h == nil {
		return
	}
	h.mode.Store(int32(modeToIndex(ModeConsoleClose)))
	h.mu.Lock()
	if h.consoleCloseHandled {
		h.mu.Unlock()
		return
	}
	h.consoleCloseHandled = true
	handler := h.consoleCloseHandler
	h.mu.Unlock()
	if handler != nil {
		handler()
	}
}

func modeToIndex(mode Mode) int {
	switch mode {
	case ModeConsoleClose:
		return 1
	default:
		return 0
	}
}

func modeFromIndex(value int) Mode {
	switch value {
	case 1:
		return ModeConsoleClose
	default:
		return ModeDefault
	}
}
