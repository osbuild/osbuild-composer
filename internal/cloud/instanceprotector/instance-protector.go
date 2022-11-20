package instanceprotector

import (
	"context"
	"time"
)

type InstanceProtectorImpl interface {
	SetProtection(bool)
}

type InstanceProtector struct {
	protImpl         InstanceProtectorImpl
	protectTimeout   time.Duration
	unprotectTimeout time.Duration
	protect          chan struct{}
	unprotect        chan struct{}
}

// NewInstanceProtector returns an instance protector that can
// protect up to @size jobs in parallel. Beyond that limit
// calls to Protect() starts to block.
func NewInstanceProtector(protectTimeout, unprotectTimeout time.Duration, size uint64, protImpl InstanceProtectorImpl) *InstanceProtector {
	ip := &InstanceProtector{
		protImpl:         protImpl,
		protectTimeout:   protectTimeout,
		unprotectTimeout: unprotectTimeout,
		protect:          make(chan struct{}, size),
		unprotect:        make(chan struct{}, size),
	}
	return ip
}

// Protect requests the instance to be protected from shutdown.
//
// The request is serviced asynchronously and is best effort only.
func (ip *InstanceProtector) Protect() {
	if ip != nil {
		ip.protect <- struct{}{}
	}
}

// Unprotect indicates that the instance no longer needs to be protected.
//
// The request is serviced asynchronosuly, so the instance may still
// remain protected even though all Protect requests have been undone.
func (ip *InstanceProtector) Unprotect() {
	if ip != nil {
		ip.unprotect <- struct{}{}
	}
}

// protectOnce asynchronously performs one protect followed by one
// unprotect call. Return early in case the context is cancelled.
//
// If the context is cancelled, we may not actually call
// protect/unprotect at all. But if protect is called, then
// unprotect is always called too.
//
// Collate matching protect/unprotect tokens when possible
// to avoid uneccesary calls (which can be slow), this should
// minimize the risk of queues of tokens building up which in
// the worst case could block new work being scheduled, or at
// least cause us to unprotect when we should still be protected
// and vice versa.
//
// The main reason for doing this async, is that the calls can
// be very slow and cause unneccesary blocking. There is already
// a race between a job being dequeued and the instance being
// protected, and moreover the protection is not always respected
// by the cloud provider, so we do not lose much by being async.
func (ip *InstanceProtector) protectOnce(ctx context.Context) {
protect:
	for {
		// Pop off alternating protect / unprotect tokens until we receive
		// a protect token without a matching unprotect one for ten seconds.
		// At that point, protect the instance and break from this loop.
		//
		// We wait for ten seconds to avoid a busy loop, and allow more
		// collation. At worst this means we have to redo ten seconds of
		// work in case the instance is killed early (but that can anyway
		// happen for other reasons).

		var timeout <-chan time.Time
		select {
		case <-ip.protect:
			// received the initial protect request, do not act on it
			// for at least ten seconds
			timeout = time.After(ip.protectTimeout)
		case <-ctx.Done():
			return
		}
		for {
			// Drain the channels of already scheduled protect / unprotect pairs
			select {
			case <-ip.unprotect:
			case <-timeout:
				// There has been more protect than unprotect requests for
				// the duration of the timeout. Ready to set protection,
				// break out of the loop.
				break protect
			case <-ctx.Done():
				return
			}
			select {
			case <-ip.protect:
			default:
				// All protect and unprotect calls have been matched up before
				// the timeout expired and no protect requests are pending. Discard
				// the request to protect the instance and start over.
				continue protect
			}
		}
	}
	ip.protImpl.SetProtection(true)
	defer ip.protImpl.SetProtection(false)
unprotect:
	for {
		// Pop off alternating unprotect / protect tokens until we receive
		// an unprotect token without a matching protect one for one minute.
		// At that point, unprotect the instance and break from this loop and
		// start waiting for a protect token again.
		//
		// We wait for one minute to avoid a busy loop, and allow more
		// collation. At worst this means we keep an instance running for
		// one more minute than we need to.
		select {
		case <-ip.unprotect:
		case <-ctx.Done():
			return
		}
		select {
		case <-ip.protect:
		case <-time.After(ip.unprotectTimeout):
			break unprotect
		case <-ctx.Done():
			return
		}
	}
}

// Start performs alternating calls to protect / unprotect in
// response to asynchronous Protect()/Unprotect() calls on
// ip.
//
// The function returns when ctx is cancelled.
func (ip *InstanceProtector) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ip.protectOnce(ctx)
	}
}
