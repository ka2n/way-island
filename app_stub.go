//go:build !gtk4

package main

import (
	"context"

	"github.com/ka2n/way-island/internal/socket"
)

func runUI(ctx context.Context, updates <-chan socket.SessionUpdate, store *overlayModel) int {
	go forwardUIUpdates(ctx, updates, store, func(string) {})

	<-ctx.Done()
	return 0
}
