package osbuildexecutor

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
)

// ErrSecureInstanceGone is returned when the AWS executor VM disappeared
// (Spot BidEvicted, process crash, etc.) before osbuild output was fetched.
var ErrSecureInstanceGone = errors.New("secure instance died")

func isSecureInstanceUnreachable(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ENETUNREACH, syscall.EHOSTUNREACH, syscall.EPIPE:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"connection refused",
		"connection reset by peer",
		"connection reset",
		"no route to host",
		"network is unreachable",
		"broken pipe",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func isTruncatedMonitor(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "unexpected end of JSON input") ||
		strings.Contains(msg, "unexpected EOF")
}

// isSecureInstanceGone reports whether the executor VM is gone rather than
// osbuild failing for real. fetchLog timeout alone is not SI-gone.
func isSecureInstanceGone(buildErr, fetchErr error) bool {
	if isSecureInstanceUnreachable(buildErr) {
		return true
	}
	if isTruncatedMonitor(buildErr) && isSecureInstanceUnreachable(fetchErr) {
		return true
	}
	if buildErr == nil && isSecureInstanceUnreachable(fetchErr) {
		return true
	}
	return false
}

func wrapSecureInstanceGone(opErr, fetchErr error) error {
	switch {
	case opErr != nil && fetchErr != nil:
		return fmt.Errorf("%w: %w, unable to fetch log: %w", ErrSecureInstanceGone, opErr, fetchErr)
	case opErr != nil:
		return fmt.Errorf("%w: %w", ErrSecureInstanceGone, opErr)
	case fetchErr != nil:
		return fmt.Errorf("%w: unable to fetch output: %w", ErrSecureInstanceGone, fetchErr)
	default:
		return ErrSecureInstanceGone
	}
}
