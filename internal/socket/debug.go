package socket

import (
	"log"
	"os"
	"time"
)

var socketDebugEnabled = os.Getenv("WAYISLAND_DEBUG") == "1"

func debugf(format string, args ...any) {
	if !socketDebugEnabled {
		return
	}
	prefix := time.Now().Format("2006-01-02T15:04:05.000")
	log.Printf("[%s] [socket] "+format, append([]any{prefix}, args...)...)
}
