package control

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestCanceledRecoveryLoopsClosePublisherBeforeReturn(t *testing.T) {
	var closed atomic.Int32
	m := &Module{close: func() error { closed.Add(1); return nil }}
	ctx, cancel := context.WithCancel(t.Context())
	m.Start(ctx)
	cancel()
	waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
	defer stop()
	if err := m.Wait(waitCtx); err != nil || closed.Load() != 1 {
		t.Fatalf("loops no terminaron/cerraron publisher: %v %d", err, closed.Load())
	}
}

func TestRecoveryShutdownHasDeadline(t *testing.T) {
	m := &Module{}
	m.wg.Add(1)
	defer m.wg.Done()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if err := m.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
