package codex

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func (t *Translator) ObserveServer(raw []byte) (Result, error) {
	var message map[string]any
	if err := json.Unmarshal(raw, &message); err != nil {
		return Result{}, err
	}

	if id, ok := message["id"]; ok {
		requestID := fmt.Sprint(id)
		if pending, ok := t.pendingSuppressedResponse[requestID]; ok {
			delete(t.pendingSuppressedResponse, requestID)
			if errMsg := extractJSONRPCErrorMessage(message); errMsg != "" {
				delete(t.pendingRemoteTurnByThread, pending.ThreadID)
				t.debugf("observe server suppressed response error: request=%s action=%s thread=%s error=%s", requestID, pending.Action, pending.ThreadID, errMsg)
				if pending.Action == "turn/start" {
					return Result{Events: []agentproto.Event{{
						Kind:         agentproto.EventTurnCompleted,
						ThreadID:     pending.ThreadID,
						Status:       "failed",
						ErrorMessage: errMsg,
					}}}, nil
				}
				return Result{}, nil
			}
			t.debugf("observe server suppressed response: request=%s", requestID)
			return Result{Suppress: true}, nil
		}
		if t.pendingInternalThreadSet[requestID] {
			delete(t.pendingInternalThreadSet, requestID)
			threadID := lookupString(message, "result", "thread", "id")
			if threadID == "" {
				threadID = lookupString(message, "result", "id")
			}
			if threadID != "" {
				t.internalThreadIDs[threadID] = true
			}
			return Result{}, nil
		}
		if t.pendingInternalTurnSet[requestID] {
			delete(t.pendingInternalTurnSet, requestID)
			turnID := lookupString(message, "result", "turn", "id")
			if turnID == "" {
				turnID = lookupString(message, "result", "id")
			}
			if turnID != "" {
				t.internalTurnIDs[turnID] = true
				t.turnInitiators[turnID] = agentproto.Initiator{Kind: agentproto.InitiatorInternalHelper}
			}
			return Result{}, nil
		}
		if pending, exists := t.pendingThreadCreate[requestID]; exists {
			delete(t.pendingThreadCreate, requestID)
			if errMsg := extractJSONRPCErrorMessage(message); errMsg != "" {
				t.debugf("observe server thread/start error: request=%s error=%s", requestID, errMsg)
				return Result{Events: []agentproto.Event{{
					Kind:         agentproto.EventTurnCompleted,
					Status:       "failed",
					ErrorMessage: errMsg,
				}}}, nil
			}
			threadID := lookupString(message, "result", "thread", "id")
			if threadID == "" {
				threadID = lookupString(message, "result", "id")
			}
			t.currentThreadID = threadID
			if pending.Command.Target.CWD != "" {
				t.knownThreadCWD[threadID] = pending.Command.Target.CWD
			}
			followup, followupID, err := t.directTurnStart(threadID, pending.Command, true)
			if err != nil {
				return Result{}, err
			}
			t.debugf("observe server thread/start result: request=%s thread=%s followup=%s", requestID, threadID, followupID)
			return Result{
				Suppress:        true,
				OutboundToCodex: [][]byte{followup},
			}, nil
		}
		if pending, exists := t.pendingThreadResume[requestID]; exists {
			delete(t.pendingThreadResume, requestID)
			if errMsg := extractJSONRPCErrorMessage(message); errMsg != "" {
				t.debugf("observe server thread/resume error: request=%s thread=%s error=%s", requestID, pending.ThreadID, errMsg)
				return Result{Events: []agentproto.Event{{
					Kind:         agentproto.EventTurnCompleted,
					ThreadID:     pending.ThreadID,
					Status:       "failed",
					ErrorMessage: errMsg,
				}}}, nil
			}
			t.currentThreadID = pending.ThreadID
			if pending.Command.Target.CWD != "" {
				t.knownThreadCWD[pending.ThreadID] = pending.Command.Target.CWD
			}
			followup, followupID, err := t.directTurnStart(pending.ThreadID, pending.Command, false)
			if err != nil {
				return Result{}, err
			}
			t.debugf("observe server thread/resume result: request=%s thread=%s followup=%s", requestID, pending.ThreadID, followupID)
			return Result{
				Suppress:        true,
				OutboundToCodex: [][]byte{followup},
			}, nil
		}
		if pending, exists := t.pendingThreadNameSet[requestID]; exists {
			delete(t.pendingThreadNameSet, requestID)
			if _, hasError := message["error"]; hasError {
				return Result{}, nil
			}
			name := choose(
				pending.Name,
				lookupString(message, "result", "thread", "name"),
				lookupString(message, "result", "name"),
			)
			if pending.ThreadID == "" || name == "" {
				return Result{}, nil
			}
			return Result{
				Events: []agentproto.Event{{
					Kind:     agentproto.EventThreadDiscovered,
					ThreadID: pending.ThreadID,
					Name:     name,
				}},
			}, nil
		}
		if requestID == t.pendingThreadListRequestID {
			delete(t.threadRefreshRecords, "")
			t.pendingThreadListRequestID = ""
			t.threadRefreshOrder = nil
			threads := parseThreadList(message["result"])
			if len(threads) == 0 {
				t.threadRefreshRecords = map[string]agentproto.ThreadSnapshotRecord{}
				return Result{
					Suppress: true,
					Events: []agentproto.Event{{
						Kind:    agentproto.EventThreadsSnapshot,
						Threads: nil,
					}},
				}, nil
			}
			var outbound [][]byte
			for index, thread := range threads {
				thread.ListOrder = index + 1
				t.threadRefreshRecords[thread.ThreadID] = thread
				t.threadRefreshOrder = append(t.threadRefreshOrder, thread.ThreadID)
				readID := t.nextRequest("thread-read")
				t.pendingThreadReads[readID] = thread.ThreadID
				payload := map[string]any{
					"id":     readID,
					"method": "thread/read",
					"params": map[string]any{
						"threadId": thread.ThreadID,
					},
				}
				bytes, err := json.Marshal(payload)
				if err != nil {
					return Result{}, err
				}
				outbound = append(outbound, append(bytes, '\n'))
			}
			return Result{Suppress: true, OutboundToCodex: outbound}, nil
		}
		if threadID, exists := t.pendingThreadReads[requestID]; exists {
			record := t.threadRefreshRecords[threadID]
			patch := parseThreadRecord(message["result"])
			record.ThreadID = choose(patch.ThreadID, record.ThreadID)
			record.Name = choose(patch.Name, record.Name)
			record.Preview = choose(patch.Preview, record.Preview)
			record.CWD = choose(patch.CWD, record.CWD)
			record.Loaded = record.Loaded || patch.Loaded
			record.Archived = record.Archived || patch.Archived
			record.State = choose(patch.State, record.State)
			t.threadRefreshRecords[threadID] = record
			delete(t.pendingThreadReads, requestID)
			if len(t.pendingThreadReads) == 0 {
				records := make([]agentproto.ThreadSnapshotRecord, 0, len(t.threadRefreshRecords))
				seen := map[string]bool{}
				for _, originalThreadID := range t.threadRefreshOrder {
					current, ok := t.threadRefreshRecords[originalThreadID]
					if !ok || current.ThreadID == "" || seen[current.ThreadID] {
						continue
					}
					records = append(records, current)
					seen[current.ThreadID] = true
				}
				extras := make([]agentproto.ThreadSnapshotRecord, 0, len(t.threadRefreshRecords))
				for _, current := range t.threadRefreshRecords {
					if current.ThreadID == "" || seen[current.ThreadID] {
						continue
					}
					extras = append(extras, current)
				}
				sort.Slice(extras, func(i, j int) bool {
					if extras[i].ListOrder != extras[j].ListOrder {
						return extras[i].ListOrder < extras[j].ListOrder
					}
					return strings.Compare(extras[i].ThreadID, extras[j].ThreadID) < 0
				})
				records = append(records, extras...)
				t.threadRefreshRecords = map[string]agentproto.ThreadSnapshotRecord{}
				t.threadRefreshOrder = nil
				return Result{
					Suppress: true,
					Events: []agentproto.Event{{
						Kind:    agentproto.EventThreadsSnapshot,
						Threads: records,
					}},
				}, nil
			}
			return Result{Suppress: true}, nil
		}
	}

	method, _ := message["method"].(string)
	switch method {
	case "thread/started":
		threadID := lookupString(message, "params", "thread", "id")
		if threadID == "" {
			threadID = lookupString(message, "params", "threadId")
		}
		cwd := lookupString(message, "params", "thread", "cwd")
		if cwd == "" {
			cwd = lookupString(message, "params", "cwd")
		}
		name := lookupString(message, "params", "thread", "name")
		if name == "" {
			name = lookupString(message, "params", "thread", "title")
		}
		if t.internalThreadIDs[threadID] {
			if cwd != "" {
				t.knownThreadCWD[threadID] = cwd
			}
			return Result{Events: []agentproto.Event{{
				Kind:         agentproto.EventThreadDiscovered,
				ThreadID:     threadID,
				CWD:          cwd,
				Name:         name,
				FocusSource:  "remote_created_thread",
				TrafficClass: agentproto.TrafficClassInternalHelper,
				Initiator:    agentproto.Initiator{Kind: agentproto.InitiatorInternalHelper},
				Metadata:     map[string]any{"internalHelper": true},
			}}}, nil
		}
		t.currentThreadID = threadID
		if t.pendingLocalNewThreadTurn && threadID != "" {
			t.pendingLocalTurnByThread[threadID] = true
			t.pendingLocalNewThreadTurn = false
		}
		if cwd != "" {
			t.knownThreadCWD[threadID] = cwd
		}
		return Result{Events: []agentproto.Event{{
			Kind:        agentproto.EventThreadDiscovered,
			ThreadID:    threadID,
			CWD:         cwd,
			Name:        name,
			FocusSource: "remote_created_thread",
		}}}, nil
	case "thread/name/updated":
		threadID := lookupString(message, "params", "threadId")
		if t.internalThreadIDs[threadID] {
			name := lookupString(message, "params", "name")
			if name == "" {
				name = lookupString(message, "params", "thread", "name")
			}
			return Result{Events: []agentproto.Event{{
				Kind:         agentproto.EventThreadDiscovered,
				ThreadID:     threadID,
				Name:         name,
				TrafficClass: agentproto.TrafficClassInternalHelper,
				Initiator:    agentproto.Initiator{Kind: agentproto.InitiatorInternalHelper},
				Metadata:     map[string]any{"internalHelper": true},
			}}}, nil
		}
		name := lookupString(message, "params", "name")
		if name == "" {
			name = lookupString(message, "params", "thread", "name")
		}
		return Result{Events: []agentproto.Event{{
			Kind:     agentproto.EventThreadDiscovered,
			ThreadID: threadID,
			Name:     name,
		}}}, nil
	case "turn/started":
		threadID := lookupString(message, "params", "thread", "id")
		if threadID == "" {
			threadID = lookupString(message, "params", "threadId")
		}
		turnID := lookupString(message, "params", "turn", "id")
		if turnID == "" {
			turnID = lookupString(message, "params", "turnId")
		}
		trafficClass := t.trafficClassForTurn(threadID, turnID)
		pendingRemoteSurface := t.pendingRemoteTurnByThread[threadID]
		pendingLocal := t.pendingLocalTurnByThread[threadID]
		initiator := t.resolveTurnInitiator(threadID, turnID, trafficClass)
		if turnID != "" {
			t.turnInitiators[turnID] = initiator
		}
		t.debugf(
			"observe server turn/started: thread=%s turn=%s initiator=%s traffic=%s pendingRemoteSurface=%s pendingLocal=%t",
			threadID,
			turnID,
			initiator.Kind,
			trafficClass,
			pendingRemoteSurface,
			pendingLocal,
		)
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventTurnStarted,
			ThreadID:     threadID,
			TurnID:       turnID,
			Status:       "running",
			TrafficClass: trafficClass,
			Initiator:    initiator,
		}}}, nil
	case "turn/completed":
		threadID := lookupString(message, "params", "thread", "id")
		if threadID == "" {
			threadID = lookupString(message, "params", "threadId")
		}
		turnID := lookupString(message, "params", "turn", "id")
		if turnID == "" {
			turnID = lookupString(message, "params", "turnId")
		}
		trafficClass := t.trafficClassForTurn(threadID, turnID)
		status := lookupString(message, "params", "turn", "status")
		if status == "" {
			status = "completed"
		}
		errMsg := lookupString(message, "params", "turn", "error", "message")
		initiator := t.turnInitiators[turnID]
		if initiator.Kind == "" {
			initiator = t.resolveTurnInitiator(threadID, turnID, trafficClass)
		}
		delete(t.turnInitiators, turnID)
		delete(t.internalTurnIDs, turnID)
		t.debugf("observe server turn/completed: thread=%s turn=%s status=%s initiator=%s", threadID, turnID, status, initiator.Kind)
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventTurnCompleted,
			ThreadID:     threadID,
			TurnID:       turnID,
			Status:       status,
			ErrorMessage: errMsg,
			TrafficClass: trafficClass,
			Initiator:    initiator,
		}}}, nil
	case "serverRequest/started", "request/started":
		params := lookupMap(message, "params")
		request := extractRequestPayload(message)
		requestID := extractRequestID(message, request)
		if requestID == "" {
			return Result{}, nil
		}
		threadID := extractRequestThreadID(message, request)
		turnID := extractRequestTurnID(message, request)
		metadata := extractRequestMetadata(extractRequestType(request, params), request, params)
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventRequestStarted,
			ThreadID:     threadID,
			TurnID:       turnID,
			RequestID:    requestID,
			Status:       "pending",
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
			Metadata:     metadata,
		}}}, nil
	case "serverRequest/resolved", "request/resolved":
		params := lookupMap(message, "params")
		request := extractRequestPayload(message)
		requestID := extractRequestID(message, request)
		if requestID == "" {
			return Result{}, nil
		}
		threadID := extractRequestThreadID(message, request)
		turnID := extractRequestTurnID(message, request)
		metadata := extractResolvedRequestMetadata(extractRequestType(request, params), request, params)
		status := firstNonEmptyString(
			lookupStringFromAny(params["status"]),
			lookupStringFromAny(request["status"]),
			"resolved",
		)
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventRequestResolved,
			ThreadID:     threadID,
			TurnID:       turnID,
			RequestID:    requestID,
			Status:       status,
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
			Metadata:     metadata,
		}}}, nil
	case "item/completed":
		threadID := lookupString(message, "params", "threadId")
		turnID := lookupString(message, "params", "turnId")
		item := lookupMap(message, "params", "item")
		itemID := choose(
			lookupStringFromAny(item["id"]),
			lookupString(message, "params", "itemId"),
		)
		itemKind := normalizeItemKind(choose(
			lookupStringFromAny(item["type"]),
			lookupString(message, "params", "itemType"),
		))
		metadata := extractItemMetadata(itemKind, item)
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventItemCompleted,
			ThreadID:     threadID,
			TurnID:       turnID,
			ItemID:       itemID,
			ItemKind:     itemKind,
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
			Metadata:     metadata,
		}}}, nil
	case "item/started":
		threadID := lookupString(message, "params", "threadId")
		turnID := lookupString(message, "params", "turnId")
		item := lookupMap(message, "params", "item")
		itemID := choose(
			lookupStringFromAny(item["id"]),
			lookupString(message, "params", "itemId"),
		)
		itemKind := normalizeItemKind(choose(
			lookupStringFromAny(item["type"]),
			lookupString(message, "params", "itemType"),
		))
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventItemStarted,
			ThreadID:     threadID,
			TurnID:       turnID,
			ItemID:       itemID,
			ItemKind:     itemKind,
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
			Metadata:     extractItemMetadata(itemKind, item),
		}}}, nil
	case "item/agentMessage/delta":
		threadID := lookupString(message, "params", "threadId")
		turnID := lookupString(message, "params", "turnId")
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventItemDelta,
			ThreadID:     threadID,
			TurnID:       turnID,
			ItemID:       lookupString(message, "params", "itemId"),
			ItemKind:     "agent_message",
			Delta:        lookupString(message, "params", "delta"),
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
		}}}, nil
	case "item/plan/delta":
		threadID := lookupString(message, "params", "threadId")
		turnID := lookupString(message, "params", "turnId")
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventItemDelta,
			ThreadID:     threadID,
			TurnID:       turnID,
			ItemID:       lookupString(message, "params", "itemId"),
			ItemKind:     "plan",
			Delta:        lookupString(message, "params", "delta"),
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
		}}}, nil
	case "item/reasoning/summaryTextDelta":
		threadID := lookupString(message, "params", "threadId")
		turnID := lookupString(message, "params", "turnId")
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventItemDelta,
			ThreadID:     threadID,
			TurnID:       turnID,
			ItemID:       lookupString(message, "params", "itemId"),
			ItemKind:     "reasoning_summary",
			Delta:        lookupString(message, "params", "delta"),
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
			Metadata:     map[string]any{"summaryIndex": lookupIntFromAny(lookupAny(message, "params", "summaryIndex"))},
		}}}, nil
	case "item/reasoning/textDelta":
		threadID := lookupString(message, "params", "threadId")
		turnID := lookupString(message, "params", "turnId")
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventItemDelta,
			ThreadID:     threadID,
			TurnID:       turnID,
			ItemID:       lookupString(message, "params", "itemId"),
			ItemKind:     "reasoning_content",
			Delta:        lookupString(message, "params", "delta"),
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
			Metadata:     map[string]any{"contentIndex": lookupIntFromAny(lookupAny(message, "params", "contentIndex"))},
		}}}, nil
	case "item/commandExecution/outputDelta":
		threadID := lookupString(message, "params", "threadId")
		turnID := lookupString(message, "params", "turnId")
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventItemDelta,
			ThreadID:     threadID,
			TurnID:       turnID,
			ItemID:       lookupString(message, "params", "itemId"),
			ItemKind:     "command_execution_output",
			Delta:        lookupString(message, "params", "delta"),
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
		}}}, nil
	case "item/fileChange/outputDelta":
		threadID := lookupString(message, "params", "threadId")
		turnID := lookupString(message, "params", "turnId")
		return Result{Events: []agentproto.Event{{
			Kind:         agentproto.EventItemDelta,
			ThreadID:     threadID,
			TurnID:       turnID,
			ItemID:       lookupString(message, "params", "itemId"),
			ItemKind:     "file_change_output",
			Delta:        lookupString(message, "params", "delta"),
			TrafficClass: t.trafficClassForTurn(threadID, turnID),
			Initiator:    t.initiatorForTurn(threadID, turnID),
		}}}, nil
	default:
		return Result{}, nil
	}
}
