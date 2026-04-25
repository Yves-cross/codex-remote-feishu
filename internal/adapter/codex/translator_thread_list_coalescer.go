package codex

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type threadListQuery struct {
	Limit          int
	Cursor         string
	SortKey        string
	Archived       bool
	ModelProviders []string
	SourceKinds    []string
}

type threadListInflight struct {
	Key             string
	OwnerRequestID  string
	OwnerVisible    bool
	AliasRequestIDs []string
}

func defaultThreadListQuery() threadListQuery {
	return threadListQuery{
		Limit:   50,
		SortKey: "created_at",
	}
}

func normalizeThreadListQuery(params map[string]any) threadListQuery {
	query := defaultThreadListQuery()
	if params == nil {
		return query
	}
	if limit := lookupIntFromAny(params["limit"]); limit > 0 {
		query.Limit = limit
	}
	if cursor := strings.TrimSpace(lookupStringFromAny(firstNonNil(params["cursor"], params["pageToken"], params["page_token"]))); cursor != "" {
		query.Cursor = cursor
	}
	if sortKey := firstNonEmptyString(
		lookupStringFromAny(params["sortKey"]),
		lookupStringFromAny(params["sort_key"]),
	); sortKey != "" {
		query.SortKey = sortKey
	}
	if archived, ok := params["archived"]; ok {
		query.Archived = lookupBoolFromAny(archived)
	}
	query.ModelProviders = normalizeThreadListStringSlice(firstNonNil(params["modelProviders"], params["model_providers"]))
	query.SourceKinds = normalizeThreadListStringSlice(firstNonNil(params["sourceKinds"], params["source_kinds"]))
	return query
}

func (q threadListQuery) key() string {
	return fmt.Sprintf(
		"limit=%d|cursor=%s|sort=%s|archived=%t|models=%s|sources=%s",
		q.Limit,
		q.Cursor,
		q.SortKey,
		q.Archived,
		strings.Join(q.ModelProviders, ","),
		strings.Join(q.SourceKinds, ","),
	)
}

func normalizeThreadListStringSlice(source any) []string {
	raw := contentArrayValues(source)
	if len(raw) == 0 {
		return nil
	}
	values := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, current := range raw {
		value := strings.TrimSpace(lookupStringFromAny(current))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	return values
}

func (t *Translator) observeClientThreadList(requestID string, params map[string]any) Result {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return Result{}
	}
	query := normalizeThreadListQuery(params)
	key := query.key()
	if inflight := t.threadListInflight[key]; inflight != nil {
		if requestID == inflight.OwnerRequestID {
			return Result{}
		}
		inflight.AliasRequestIDs = append(inflight.AliasRequestIDs, requestID)
		t.debugf(
			"observe client thread/list joined inflight owner=%s alias=%s key=%s visible=%t aliases=%d",
			inflight.OwnerRequestID,
			requestID,
			key,
			inflight.OwnerVisible,
			len(inflight.AliasRequestIDs),
		)
		return Result{Suppress: true}
	}
	t.threadListInflight[key] = &threadListInflight{
		Key:            key,
		OwnerRequestID: requestID,
		OwnerVisible:   true,
	}
	t.threadListOwnerKeys[requestID] = key
	t.debugf("observe client thread/list owner=%s key=%s", requestID, key)
	return Result{}
}

func (t *Translator) joinThreadListInflight(query threadListQuery) (string, bool, bool) {
	inflight := t.threadListInflight[query.key()]
	if inflight == nil {
		return "", false, false
	}
	return inflight.OwnerRequestID, inflight.OwnerVisible, true
}

func (t *Translator) beginNativeThreadListInflight(requestID string, query threadListQuery) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return
	}
	key := query.key()
	t.threadListInflight[key] = &threadListInflight{
		Key:            key,
		OwnerRequestID: requestID,
		OwnerVisible:   false,
	}
	t.threadListOwnerKeys[requestID] = key
}

func (t *Translator) takeThreadListInflightByOwner(requestID string) (*threadListInflight, bool) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, false
	}
	key := t.threadListOwnerKeys[requestID]
	if strings.TrimSpace(key) == "" {
		return nil, false
	}
	delete(t.threadListOwnerKeys, requestID)
	inflight := t.threadListInflight[key]
	if inflight == nil || inflight.OwnerRequestID != requestID {
		return nil, false
	}
	delete(t.threadListInflight, key)
	return inflight, true
}

func buildAliasedJSONRPCResponses(message map[string]any, aliasRequestIDs []string) ([][]byte, error) {
	if len(aliasRequestIDs) == 0 {
		return nil, nil
	}
	out := make([][]byte, 0, len(aliasRequestIDs))
	for _, requestID := range aliasRequestIDs {
		requestID = strings.TrimSpace(requestID)
		if requestID == "" {
			continue
		}
		response := map[string]any{
			"id": requestID,
		}
		if version, ok := message["jsonrpc"]; ok {
			response["jsonrpc"] = version
		}
		if result, ok := message["result"]; ok {
			response["result"] = result
		}
		if errPayload, ok := message["error"]; ok {
			response["error"] = errPayload
		}
		bytes, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		out = append(out, append(bytes, '\n'))
	}
	return out, nil
}
