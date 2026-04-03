package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

func TestForwardUIUpdatesThrottlesFlushes(t *testing.T) {
	store := newOverlayModel()
	updates := make(chan socket.SessionUpdate, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu      sync.Mutex
		flushes []time.Time
		done    = make(chan struct{})
	)

	go func() {
		defer close(done)
		forwardUIUpdates(ctx, updates, store, func(string) {
			mu.Lock()
			flushes = append(flushes, time.Now())
			mu.Unlock()
		})
	}()

	now := time.Now()
	for i := range 3 {
		updates <- socket.SessionUpdate{
			Type: socket.SessionUpdateUpsert,
			Session: socket.Session{
				ID:          "session-1",
				State:       socket.SessionStateWorking,
				LastEventAt: now.Add(time.Duration(i) * time.Millisecond),
			},
			Reason: "test",
		}
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		mu.Lock()
		count := len(flushes)
		mu.Unlock()
		if count >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for throttled flushes")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(flushes) != 2 {
		t.Fatalf("flush count = %d, want 2", len(flushes))
	}

	if delta := flushes[1].Sub(flushes[0]); delta < uiUpdateThrottleInterval-20*time.Millisecond {
		t.Fatalf("flush interval = %v, want at least %v", delta, uiUpdateThrottleInterval-20*time.Millisecond)
	}
}

func TestForwardUIUpdatesFlushesLatestStateAfterThrottle(t *testing.T) {
	store := newOverlayModel()
	updates := make(chan socket.SessionUpdate, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan string, 4)
	done := make(chan struct{})

	go func() {
		defer close(done)
		forwardUIUpdates(ctx, updates, store, func(payload string) {
			payloads <- payload
		})
	}()

	base := time.Now()
	updates <- socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateWorking,
			LastEventAt: base,
		},
		Reason: "test",
	}
	updates <- socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateWaiting,
			LastEventAt: base.Add(time.Millisecond),
		},
		Reason: "test",
	}

	first := waitForPayload(t, payloads)
	second := waitForPayload(t, payloads)

	cancel()
	<-done

	if first == second {
		t.Fatalf("expected throttled flush to publish newer payload")
	}
	if second == "" {
		t.Fatalf("expected latest payload to be non-empty")
	}
}

func TestForwardUIUpdatesFlushesPendingPayloadWhenUpdatesClose(t *testing.T) {
	store := newOverlayModel()
	updates := make(chan socket.SessionUpdate, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan string, 4)
	done := make(chan struct{})

	go func() {
		defer close(done)
		forwardUIUpdates(ctx, updates, store, func(payload string) {
			payloads <- payload
		})
	}()

	base := time.Now()
	updates <- socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateWorking,
			LastEventAt: base,
		},
		Reason: "test",
	}
	updates <- socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateWaiting,
			LastEventAt: base.Add(time.Millisecond),
		},
		Reason: "test",
	}
	close(updates)

	first := waitForPayload(t, payloads)
	second := waitForPayload(t, payloads)
	<-done

	if first == second {
		t.Fatalf("expected closed channel to flush pending payload")
	}
}

func TestForwardUIUpdatesDoesNotStarveDuringContinuousUpdates(t *testing.T) {
	store := newOverlayModel()
	updates := make(chan socket.SessionUpdate, 32)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan string, 8)
	done := make(chan struct{})

	go func() {
		defer close(done)
		forwardUIUpdates(ctx, updates, store, func(payload string) {
			payloads <- payload
		})
	}()

	base := time.Now()
	updates <- socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateWorking,
			LastEventAt: base,
		},
		Reason: "test",
	}

	time.Sleep(uiUpdateThrottleInterval / 2)

	updates <- socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateToolRunning,
			LastEventAt: base.Add(time.Millisecond),
		},
		Reason: "test",
	}

	time.Sleep(uiUpdateThrottleInterval / 2)

	updates <- socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateWaiting,
			LastEventAt: base.Add(2 * time.Millisecond),
		},
		Reason: "test",
	}

	first := waitForPayload(t, payloads)
	second := waitForPayload(t, payloads)

	cancel()
	<-done

	if first == second {
		t.Fatalf("expected a second flush during continuous updates")
	}
}

func waitForPayload(t *testing.T, payloads <-chan string) string {
	t.Helper()

	select {
	case payload := <-payloads:
		return payload
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for payload")
		return ""
	}
}
