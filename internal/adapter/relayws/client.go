package relayws

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"

	"github.com/gorilla/websocket"
)

type ClientCallbacks struct {
	OnCommand func(context.Context, agentproto.Command) error
	OnWelcome func(context.Context, agentproto.Welcome) error
	OnConnect func(context.Context) error
}

type Client struct {
	url       string
	hello     agentproto.Hello
	callbacks ClientCallbacks

	mu      sync.RWMutex
	conn    *websocket.Conn
	outbox  chan agentproto.Envelope
	closed  chan struct{}
	closeMu sync.Once
}

func NewClient(url string, hello agentproto.Hello, callbacks ClientCallbacks) *Client {
	if hello.Protocol == "" {
		hello.Protocol = agentproto.WireProtocol
	}
	return &Client{
		url:       normalizeRelayURL(url),
		hello:     hello,
		callbacks: callbacks,
		outbox:    make(chan agentproto.Envelope, 512),
		closed:    make(chan struct{}),
	}
}

func (c *Client) Run(ctx context.Context) error {
	backoff := 200 * time.Millisecond
	for {
		err := c.RunOnce(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			return err
		}
		var fatalErr FatalError
		if errors.As(err, &fatalErr) {
			return err
		}
		log.Printf("relay client connect failed: url=%s err=%v", c.url, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

type FatalError struct {
	Err error
}

func (e FatalError) Error() string {
	if e.Err == nil {
		return "fatal relay error"
	}
	return e.Err.Error()
}

func (e FatalError) Unwrap() error {
	return e.Err
}

func normalizeRelayURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/ws/agent"
	}
	return parsed.String()
}

func (c *Client) Close() {
	c.closeMu.Do(func() {
		close(c.closed)
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	})
}

func (c *Client) SendEvents(events []agentproto.Event) error {
	if len(events) == 0 {
		return nil
	}
	return c.enqueue(agentproto.Envelope{
		Type: agentproto.EnvelopeEventBatch,
		EventBatch: &agentproto.EventBatch{
			InstanceID: c.hello.Instance.InstanceID,
			Events:     events,
		},
	})
}

func (c *Client) SendCommandAck(ack agentproto.CommandAck) error {
	if ack.InstanceID == "" {
		ack.InstanceID = c.hello.Instance.InstanceID
	}
	return c.enqueue(agentproto.Envelope{
		Type:       agentproto.EnvelopeCommandAck,
		CommandAck: &ack,
	})
}

func (c *Client) enqueue(envelope agentproto.Envelope) error {
	select {
	case <-c.closed:
		return context.Canceled
	default:
	}
	select {
	case c.outbox <- envelope:
		return nil
	default:
		return errors.New("relay client outbox full")
	}
}

func (c *Client) RunOnce(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, http.Header{})
	if err != nil {
		return err
	}
	defer conn.Close()

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	helloBytes, err := agentproto.MarshalEnvelope(agentproto.Envelope{
		Type:  agentproto.EnvelopeHello,
		Hello: &c.hello,
	})
	if err != nil {
		return err
	}
	if err := conn.WriteMessage(websocket.TextMessage, helloBytes); err != nil {
		return err
	}

	writeErr := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				writeErr <- ctx.Err()
				return
			case <-c.closed:
				writeErr <- context.Canceled
				return
			case envelope := <-c.outbox:
				payload, err := agentproto.MarshalEnvelope(envelope)
				if err != nil {
					writeErr <- err
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
					writeErr <- err
					return
				}
			}
		}
	}()

	for {
		select {
		case err := <-writeErr:
			return err
		default:
		}

		messageType, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		envelope, err := agentproto.UnmarshalEnvelope(raw)
		if err != nil {
			log.Printf("relay client decode failed: %v", err)
			continue
		}
		switch envelope.Type {
		case agentproto.EnvelopeWelcome:
			if envelope.Welcome == nil {
				continue
			}
			if c.callbacks.OnWelcome != nil {
				if err := c.callbacks.OnWelcome(ctx, *envelope.Welcome); err != nil {
					return err
				}
			}
			if c.callbacks.OnConnect != nil {
				if err := c.callbacks.OnConnect(ctx); err != nil {
					return err
				}
			}
		case agentproto.EnvelopeCommand:
			if envelope.Command == nil {
				continue
			}
			if c.callbacks.OnCommand != nil {
				if err := c.callbacks.OnCommand(ctx, *envelope.Command); err != nil {
					_ = c.SendCommandAck(agentproto.CommandAck{
						CommandID: envelope.Command.CommandID,
						Accepted:  false,
						Error:     err.Error(),
					})
					return err
				}
			}
			_ = c.SendCommandAck(agentproto.CommandAck{
				CommandID: envelope.Command.CommandID,
				Accepted:  true,
			})
		}
	}
}
