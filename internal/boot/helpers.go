//go:build integration

package boot

import (
	"log"
	"os"
	"syscall"
	"time"
)

// durationMin returns the smaller of two given durations
func durationMin(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// killProcessCleanly firstly sends SIGTERM to the process. If it still exists
// after the specified timeout, it sends SIGKILL
func killProcessCleanly(process *os.Process, timeout time.Duration) error {
	err := process.Signal(syscall.SIGTERM)
	if err != nil {
		log.Printf("cannot send SIGTERM to process, sending SIGKILL instead: %v", err)
		return process.Kill()
	}

	const pollInterval = 10 * time.Millisecond

	for {
		p, err := os.FindProcess(process.Pid)
		if err != nil {
			return nil
		}

		err = p.Signal(syscall.Signal(0))
		if err != nil {
			return nil
		}

		sleep := durationMin(pollInterval, timeout)
		if sleep == 0 {
			break
		}

		timeout -= sleep
		time.Sleep(sleep)
	}

	return process.Kill()
}
