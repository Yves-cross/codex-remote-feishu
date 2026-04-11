package orchestrator

type threadKickStatus string

const (
	threadKickIdle    threadKickStatus = "idle"
	threadKickQueued  threadKickStatus = "queued"
	threadKickRunning threadKickStatus = "running"
)
