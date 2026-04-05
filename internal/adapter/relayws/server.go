package relayws

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"

	"github.com/gorilla/websocket"
)

type ServerCallbacks struct {
	OnHello      func(context.Context, agentproto.Hello)
	OnEvents     func(context.Context, string, []agentproto.Event)
	OnCommandAck func(context.Context, string, agentproto.CommandAck)
	OnDisconnect func(context.Context, string)
}

type Server struct {
	upgrader  websocket.Upgrader
	callbacks ServerCallbacks
	identity  agentproto.ServerIdentity

	mu       sync.RWMutex
	conns    map[string]*serverConn
	shutdown chan struct{}
}

type serverConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewServer(callbacks ServerCallbacks) *Server {
	return &Server{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
		callbacks: callbacks,
		conns:     map[string]*serverConn{},
		shutdown:  make(chan struct{}),
	}
}

func (s *Server) SetServerIdentity(identity agentproto.ServerIdentity) {
	s.identity = identity
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	go s.serveConn(ctx, cancel, conn)
}

func (s *Server) Close() error {
	close(s.shutdown)
	s.mu.Lock()
	defer s.mu.Unlock()
	for instanceID, current := range s.conns {
		_ = current.conn.Close()
		delete(s.conns, instanceID)
	}
	return nil
}

func (s *Server) SendCommand(instanceID string, command agentproto.Command) error {
	s.mu.RLock()
	current := s.conns[instanceID]
	s.mu.RUnlock()
	if current == nil {
		return errors.New("instance offline")
	}
	payload, err := agentproto.MarshalEnvelope(agentproto.Envelope{
		Type:    agentproto.EnvelopeCommand,
		Command: &command,
	})
	if err != nil {
		return err
	}
	current.mu.Lock()
	defer current.mu.Unlock()
	current.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return current.conn.WriteMessage(websocket.TextMessage, payload)
}

func (s *Server) serveConn(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn) {
	defer conn.Close()
	defer cancel()
	var instanceID string
	defer func() {
		if instanceID == "" {
			return
		}
		s.mu.Lock()
		if current := s.conns[instanceID]; current != nil && current.conn == conn {
			delete(s.conns, instanceID)
		}
		s.mu.Unlock()
		if s.callbacks.OnDisconnect != nil {
			s.callbacks.OnDisconnect(ctx, instanceID)
		}
	}()

	for {
		messageType, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		envelope, err := agentproto.UnmarshalEnvelope(raw)
		if err != nil {
			_ = writeError(conn, "bad_envelope", err.Error())
			continue
		}
		switch envelope.Type {
		case agentproto.EnvelopeHello:
			if envelope.Hello == nil {
				_ = writeError(conn, "bad_hello", "missing hello payload")
				return
			}
			instanceID = envelope.Hello.Instance.InstanceID
			current := &serverConn{conn: conn}
			s.mu.Lock()
			if previous := s.conns[instanceID]; previous != nil && previous.conn != conn {
				_ = previous.conn.Close()
			}
			s.conns[instanceID] = current
			s.mu.Unlock()
			if s.callbacks.OnHello != nil {
				s.callbacks.OnHello(ctx, *envelope.Hello)
			}
			serverIdentity := s.identity
			var serverPtr *agentproto.ServerIdentity
			if serverIdentity.Product != "" || serverIdentity.Version != "" || serverIdentity.BuildFingerprint != "" || serverIdentity.PID != 0 {
				serverPtr = &serverIdentity
			}
			payload, _ := agentproto.MarshalEnvelope(agentproto.Envelope{
				Type: agentproto.EnvelopeWelcome,
				Welcome: &agentproto.Welcome{
					Protocol:   agentproto.WireProtocol,
					ServerTime: time.Now(),
					Server:     serverPtr,
				},
			})
			current.mu.Lock()
			err = current.conn.WriteMessage(websocket.TextMessage, payload)
			current.mu.Unlock()
			if err != nil {
				return
			}
		case agentproto.EnvelopeEventBatch:
			if envelope.EventBatch == nil {
				continue
			}
			if s.callbacks.OnEvents != nil {
				s.callbacks.OnEvents(ctx, envelope.EventBatch.InstanceID, envelope.EventBatch.Events)
			}
		case agentproto.EnvelopeCommandAck:
			if envelope.CommandAck == nil {
				continue
			}
			if instanceID == "" {
				instanceID = envelope.CommandAck.InstanceID
			}
			if s.callbacks.OnCommandAck != nil {
				s.callbacks.OnCommandAck(ctx, instanceID, *envelope.CommandAck)
			}
		}
	}
}

func writeError(conn *websocket.Conn, code, message string) error {
	payload, err := agentproto.MarshalEnvelope(agentproto.Envelope{
		Type: agentproto.EnvelopeError,
		Error: &agentproto.ErrorEnvelope{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, payload)
}
