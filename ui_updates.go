package main

import (
	"context"
	"log"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

const uiUpdateThrottleInterval = 100 * time.Millisecond

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
			detector.Observe(update)
			scheduleFlush(time.Now())
		case update := <-detectedUpdates:
			log.Printf("session update type=%s session_id=%s state=%s reason=%s", update.Type, update.Session.ID, update.Session.State, update.Reason)
			store.Apply(update)
			scheduleFlush(time.Now())
		}
	}
}
