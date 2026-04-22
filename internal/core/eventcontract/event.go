package eventcontract

import "strings"

type EventMeta struct {
	Target               TargetRef
	SourceMessageID      string
	SourceMessagePreview string
	DaemonLifecycleID    string
	InlineReplaceMode    InlineReplaceMode
	Semantics            DeliverySemantics
}

func (meta EventMeta) Normalized() EventMeta {
	meta.Target = meta.Target.Normalized()
	meta.SourceMessageID = strings.TrimSpace(meta.SourceMessageID)
	meta.SourceMessagePreview = strings.TrimSpace(meta.SourceMessagePreview)
	meta.DaemonLifecycleID = strings.TrimSpace(meta.DaemonLifecycleID)
	switch meta.InlineReplaceMode {
	case InlineReplaceCurrentCard:
	default:
		meta.InlineReplaceMode = InlineReplaceNone
	}
	meta.Semantics = meta.Semantics.Normalized()
	return meta
}

type Event struct {
	Meta    EventMeta
	Payload Payload
}

func (event Event) Kind() Kind {
	return PayloadKind(event.Payload)
}

func (event Event) GatewayID() string {
	return event.Meta.Target.GatewayID
}

func (event Event) SurfaceSessionID() string {
	return event.Meta.Target.SurfaceSessionID
}

func (event Event) Normalized() Event {
	event.Meta = event.Meta.Normalized()
	return event
}
