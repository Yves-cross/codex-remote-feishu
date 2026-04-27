package state

import "time"

type AssistantStreamRecord struct {
	InstanceID           string
	ThreadID             string
	TurnID               string
	ItemID               string
	Phase                string
	SourceMessageID      string
	SourceMessagePreview string
	MessageID            string
	StreamCardID         string
	CompletedText        string
	Text                 string
	VisibleText          string
	Loading              bool
	LastEmittedText      string
	LastEmittedAt        time.Time
}
