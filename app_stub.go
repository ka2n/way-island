//go:build !gtk4

package main

import (
	"context"
	"log"

	"github.com/ka2n/way-island/internal/socket"
)

func runUI(ctx context.Context, updates <-chan socket.SessionUpdate, store *overlayModel) int {
	go func() {
		for update := range updates {
			log.Printf("session update type=%s session_id=%s state=%s reason=%s", update.Type, update.Session.ID, update.Session.State, update.Reason)
			store.Apply(update)
		}
	}()

	<-ctx.Done()
	return 0
}
