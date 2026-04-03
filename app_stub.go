//go:build !gtk4

package main

import (
	"context"
	"log"

	"github.com/ka2n/way-island/internal/socket"
)

func runUI(ctx context.Context, updates <-chan socket.SessionUpdate) int {
	go func() {
		for update := range updates {
			log.Printf("session update type=%s session_id=%s state=%s", update.Type, update.Session.ID, update.Session.State)
		}
	}()

	<-ctx.Done()
	return 0
}
