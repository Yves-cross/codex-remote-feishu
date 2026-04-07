package daemon

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

var errIngressPumpClosed = errors.New("daemon ingress pump closed")

type ingressWorkKind string

const (
	ingressWorkHello      ingressWorkKind = "hello"
	ingressWorkEvents     ingressWorkKind = "events"
	ingressWorkCommandAck ingressWorkKind = "command_ack"
	ingressWorkDisconnect ingressWorkKind = "disconnect"
)

type ingressWorkItem struct {
	instanceID string
	kind       ingressWorkKind
	hello      *agentproto.Hello
	events     []agentproto.Event
	ack        *agentproto.CommandAck
}

type ingressPump struct {
	mu       sync.Mutex
	queues   map[string][]ingressWorkItem
	ready    []string
	readySet map[string]bool
	notify   chan struct{}
	closed   chan struct{}
	done     chan struct{}
}

func newIngressPump() *ingressPump {
	return &ingressPump{
		queues:   map[string][]ingressWorkItem{},
		readySet: map[string]bool{},
		notify:   make(chan struct{}, 1),
		closed:   make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (p *ingressPump) Enqueue(item ingressWorkItem) error {
	instanceID := strings.TrimSpace(item.instanceID)
	if instanceID == "" {
		return errors.New("daemon ingress requires instance id")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.closed:
		return errIngressPumpClosed
	default:
	}

	p.queues[instanceID] = append(p.queues[instanceID], item)
	if !p.readySet[instanceID] {
		p.ready = append(p.ready, instanceID)
		p.readySet[instanceID] = true
	}
	select {
	case p.notify <- struct{}{}:
	default:
	}
	return nil
}

func (p *ingressPump) Run(ctx context.Context, process func(ingressWorkItem)) error {
	defer close(p.done)
	for {
		item, ok := p.dequeue()
		if ok {
			process(item)
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.closed:
			return nil
		case <-p.notify:
		}
	}
}

func (p *ingressPump) Close() {
	select {
	case <-p.closed:
	default:
		close(p.closed)
	}
}

func (p *ingressPump) Wait() {
	<-p.done
}

func (p *ingressPump) dequeue() (ingressWorkItem, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.ready) == 0 {
		return ingressWorkItem{}, false
	}

	instanceID := p.ready[0]
	p.ready = p.ready[1:]

	queue := p.queues[instanceID]
	item := queue[0]
	queue = queue[1:]
	if len(queue) == 0 {
		delete(p.queues, instanceID)
		delete(p.readySet, instanceID)
	} else {
		p.queues[instanceID] = queue
		p.ready = append(p.ready, instanceID)
	}
	return item, true
}
