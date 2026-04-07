package wrapper

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/adapter/relayws"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

type relayEventSender interface {
	SendEvents([]agentproto.Event) error
}

type problemReportRecord struct {
	lastEmittedAt time.Time
	suppressed    int
}

type problemReporter struct {
	mu           sync.Mutex
	client       relayEventSender
	pending      []agentproto.ErrorInfo
	now          func() time.Time
	dedupeWindow time.Duration
	maxRecords   int
	recent       map[string]*problemReportRecord
	order        []string
}

func (r *problemReporter) SetClient(client *relayws.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.initDefaultsLocked()
	r.client = client
}

func (r *problemReporter) Emit(problem agentproto.ErrorInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.initDefaultsLocked()
	problem = problem.Normalize()
	now := r.now()
	key := problemReportKey(problem)
	record := r.recent[key]
	if record != nil && now.Sub(record.lastEmittedAt) < r.dedupeWindow {
		record.suppressed++
		if record.suppressed == 1 || record.suppressed%10 == 0 {
			log.Printf("wrapper problem reporter suppressed duplicate: code=%s layer=%s stage=%s operation=%s suppressed=%d", problem.Code, problem.Layer, problem.Stage, problem.Operation, record.suppressed)
		}
		return
	}
	if record == nil {
		r.trimRecentLocked(now)
		r.evictOldestIfNeededLocked()
		record = &problemReportRecord{}
		r.recent[key] = record
		r.order = append(r.order, key)
	}
	record.lastEmittedAt = now
	record.suppressed = 0
	r.pending = append(r.pending, problem)
	r.flushLocked()
}

func (r *problemReporter) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.initDefaultsLocked()
	r.flushLocked()
}

func (r *problemReporter) flushLocked() {
	if r.client == nil || len(r.pending) == 0 {
		return
	}
	events := make([]agentproto.Event, 0, len(r.pending))
	for _, problem := range r.pending {
		events = append(events, agentproto.NewSystemErrorEvent(problem))
	}
	if err := r.client.SendEvents(events); err != nil {
		return
	}
	r.pending = nil
}

func (r *problemReporter) initDefaultsLocked() {
	if r.now == nil {
		r.now = time.Now
	}
	if r.dedupeWindow <= 0 {
		r.dedupeWindow = 5 * time.Second
	}
	if r.maxRecords <= 0 {
		r.maxRecords = 256
	}
	if r.recent == nil {
		r.recent = map[string]*problemReportRecord{}
	}
}

func (r *problemReporter) trimRecentLocked(now time.Time) {
	if len(r.order) == 0 {
		return
	}
	nextOrder := r.order[:0]
	for _, key := range r.order {
		record := r.recent[key]
		if record == nil || now.Sub(record.lastEmittedAt) >= r.dedupeWindow {
			delete(r.recent, key)
			continue
		}
		nextOrder = append(nextOrder, key)
	}
	r.order = nextOrder
}

func (r *problemReporter) evictOldestIfNeededLocked() {
	for len(r.order) >= r.maxRecords && len(r.order) > 0 {
		evicted := r.order[0]
		r.order = r.order[1:]
		delete(r.recent, evicted)
		log.Printf("wrapper problem reporter evicted old record: key=%q limit=%d", evicted, r.maxRecords)
	}
}

func problemReportKey(problem agentproto.ErrorInfo) string {
	return strings.Join([]string{
		problem.Code,
		problem.Layer,
		problem.Stage,
		problem.Operation,
		problem.Details,
	}, "\x00")
}
