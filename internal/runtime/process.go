package relayruntime

import "time"

func TerminateProcess(pid int, grace time.Duration) error {
	return terminateProcess(pid, grace)
}
