package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

const debugLogPath = "/tmp/wayisland-hook.log"

var debugLogger *log.Logger

func init() {
	if os.Getenv("WAYISLAND_DEBUG") != "1" {
		return
	}

	f, err := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}

	debugLogger = log.New(f, "", 0)

	// Redirect the standard log package output to the same file so daemon-side
	// log.Printf calls also land in the debug log.
	log.SetOutput(f)
	log.SetFlags(0)
}

func debugf(format string, args ...any) {
	if debugLogger == nil {
		return
	}
	prefix := time.Now().Format("2006-01-02T15:04:05.000")
	debugLogger.Printf("[%s] "+format, append([]any{prefix}, args...)...)
}

func debugJSON(label string, v any) {
	if debugLogger == nil {
		return
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		debugf("%s: (marshal error: %v)", label, err)
		return
	}
	debugf("%s:\n%s", label, fmt.Sprintf("%s", b))
}
