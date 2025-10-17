package worker

import "os/exec"

// MockExecCommand replaces the exec.Command() wrapper and returns a function
// that can be called to restore the original.
func MockExecCommand(mock func(name string, arg ...string) *exec.Cmd) (restore func()) {
	original := execCommand
	execCommand = mock
	return func() {
		execCommand = original
	}
}
