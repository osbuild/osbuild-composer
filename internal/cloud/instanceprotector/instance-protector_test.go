package instanceprotector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type DummyInstanceProtector struct {
	protected int
	barrier   chan bool
}

func NewDummyInstanceProtector() *DummyInstanceProtector {
	return &DummyInstanceProtector{
		barrier: make(chan bool),
	}
}

func (p *DummyInstanceProtector) SetProtection(protected bool) {
	if protected {
		p.protected += 1
	} else {
		p.protected -= 1
	}
	p.barrier <- protected
}

func TestInstanceProtector_Start(t *testing.T) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	p := NewDummyInstanceProtector()
	ip := NewInstanceProtector(10*time.Millisecond, 10*time.Millisecond, 10, p)
	require.Equal(t, 0, p.protected)

	go ip.Start(ctx)

	// double protect + double unprotect, no collation
	ip.Protect()
	require.True(t, <-p.barrier) // wait for the protection to kick in
	require.Equal(t, 1, p.protected)
	ip.Protect()   // already protected - no effect
	ip.Unprotect() // undoes one protection - no effect
	ip.Unprotect()
	require.False(t, <-p.barrier) // wait for the protection te be lifted
	require.Equal(t, 0, p.protected)

	// double protect + double unprotect, partial collation
	ip.Protect()
	ip.Protect()
	ip.Unprotect()
	require.True(t, <-p.barrier) // wait for the protection to kick in
	require.Equal(t, 1, p.protected)
	ip.Unprotect()
	require.False(t, <-p.barrier) // wait for the protection to be lifted
	require.Equal(t, 0, p.protected)
}

func TestInstanceProtector_Cancel1(t *testing.T) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	p := NewDummyInstanceProtector()
	ip := NewInstanceProtector(10*time.Millisecond, 10*time.Millisecond, 10, p)
	require.Equal(t, 0, p.protected)

	go ip.Start(ctx)

	// protect, no unprotect
	ip.Protect()
	require.True(t, <-p.barrier) // wait for the protection to kick in
	require.Equal(t, 1, p.protected)

	// cancel the context
	ctxCancel()
	require.False(t, <-p.barrier) // wait for the protection to be lifted
	require.Equal(t, 0, p.protected)
}

func TestInstanceProtector_Cancel2(t *testing.T) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	p := NewDummyInstanceProtector()
	ip := NewInstanceProtector(10*time.Millisecond, 20*time.Millisecond, 10, p)
	require.Equal(t, 0, p.protected)

	go ip.Start(ctx)

	// protect, unprotect pending
	ip.Protect()
	require.True(t, <-p.barrier) // wait for the protection to kick in
	require.Equal(t, 1, p.protected)
	ip.Unprotect()

	// cancel the context
	ctxCancel()
	require.False(t, <-p.barrier) // wait for the protection to be lifted
	require.Equal(t, 0, p.protected)
}
