package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

const uiUpdateThrottleInterval = 100 * time.Millisecond

// activePaneTTYCache caches the result of activePaneTTY for a short TTL to avoid
// spawning a tmux subprocess on every session update.
type activePaneTTYCache struct {
	mu        sync.Mutex
	tty       string
	fetchedAt time.Time
	ttl       time.Duration
}

var sharedActivePaneTTYCache = &activePaneTTYCache{ttl: 500 * time.Millisecond}

func (c *activePaneTTYCache) get() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.fetchedAt) < c.ttl {
		return c.tty
	}
	tty, err := activePaneTTY(runFocusCommand)
	if err != nil {
		// On error (e.g. tmux not running), return empty string without caching.
		return ""
	}
	c.tty = tty
	c.fetchedAt = time.Now()
	return tty
}

// updateSessionSuppression checks whether the updated session's AgentTTY matches
// the currently active tmux pane. If it does, the session is marked as suppressed
// so its state changes don't contribute to the attention-badge counts.
// The active pane TTY is cached for 500ms to avoid spawning a subprocess on every
// session update.
func updateSessionSuppression(store *overlayModel, update socket.SessionUpdate) {
	// Timeout cleanup is handled in Apply(), but for belt-and-suspenders
	// and for callers that skip Apply(), we also clear here.
	if update.Type == socket.SessionUpdateTimeout {
		return
	}
	agentTTY := strings.TrimSpace(update.Session.AgentTTY)
	if agentTTY == "" {
		return
	}
	activeTTY := sharedActivePaneTTYCache.get()
	store.SetSuppressed(update.Session.ID, ttyMatches(agentTTY, activeTTY))
}

func forwardUIUpdates(ctx context.Context, updates <-chan socket.SessionUpdate, store *overlayModel, flush func(string)) {
	lastFlush := time.Time{}
	var timer *time.Timer
	var timerC <-chan time.Time
	detectedUpdates := make(chan socket.SessionUpdate, 8)
	detector := newApprovalPromptDetector(store, detectedUpdates)

	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerC = nil
	}

	flushNow := func() {
		flush(store.Payload())
		lastFlush = time.Now()
	}

	scheduleFlush := func(now time.Time) {
		if lastFlush.IsZero() || now.Sub(lastFlush) >= uiUpdateThrottleInterval {
			stopTimer()
			flushNow()
			return
		}

		if timerC != nil {
			return
		}

		wait := uiUpdateThrottleInterval - now.Sub(lastFlush)
		if timer == nil {
			timer = time.NewTimer(wait)
		} else {
			stopTimer()
			timer.Reset(wait)
		}
		timerC = timer.C
	}

	defer stopTimer()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timerC:
			timerC = nil
			flushNow()
		case update, ok := <-updates:
			if !ok {
				if timerC != nil {
					stopTimer()
					flushNow()
				}
				return
			}
			log.Printf("session update type=%s session_id=%s state=%s reason=%s", update.Type, update.Session.ID, update.Session.State, update.Reason)
			store.Apply(update)
			updateSessionSuppression(store, update)
			detector.Observe(update)
			scheduleFlush(time.Now())
		case update := <-detectedUpdates:
			log.Printf("session update type=%s session_id=%s state=%s reason=%s", update.Type, update.Session.ID, update.Session.State, update.Reason)
			store.Apply(update)
			scheduleFlush(time.Now())
		}
	}
}
