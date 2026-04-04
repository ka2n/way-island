package main

import (
	"strings"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

const approvalPromptDetectionDelay = 2 * time.Second

type toolApprovalPipeline interface {
	toolName() string
	detect(output string) bool
}

type bashApprovalPipeline struct{}

func (bashApprovalPipeline) toolName() string {
	return "bash"
}

func (bashApprovalPipeline) detect(output string) bool {
	return containsMarkers(output, []string{
		"would you like to run the following command?",
		"yes, proceed",
		"yes, and don't ask again",
		"press enter to confirm or esc to cancel",
	}, 2)
}

type approvalPromptDetector struct {
	store       *overlayModel
	capturePane func(sessionID string) (string, error)
	sleep       func(time.Duration)
	now         func() time.Time
	updates     chan<- socket.SessionUpdate
	pipelines   map[string]toolApprovalPipeline
}

func newApprovalPromptDetector(store *overlayModel, updates chan<- socket.SessionUpdate) *approvalPromptDetector {
	focuser := newSessionFocuser(store)
	detector := &approvalPromptDetector{
		store:       store,
		capturePane: focuser.capturePaneText,
		sleep:       time.Sleep,
		now:         time.Now,
		updates:     updates,
		pipelines:   map[string]toolApprovalPipeline{},
	}
	for _, pipeline := range []toolApprovalPipeline{bashApprovalPipeline{}} {
		detector.pipelines[pipeline.toolName()] = pipeline
	}
	return detector
}

func (d *approvalPromptDetector) Observe(update socket.SessionUpdate) {
	if update.Type != socket.SessionUpdateUpsert {
		return
	}
	if update.Session.HookSource != "codex" {
		return
	}
	if update.Session.State != socket.SessionStateToolRunning {
		return
	}
	if strings.TrimSpace(update.Session.CurrentAction) == "" {
		return
	}
	if _, ok := d.pipelines[update.Session.CurrentTool]; !ok {
		return
	}

	session := update.Session
	go d.detect(session)
}

func (d *approvalPromptDetector) detect(session socket.Session) {
	d.sleep(approvalPromptDetectionDelay)

	current, ok := d.store.Session(session.ID)
	if !ok {
		return
	}
	if current.State != socket.SessionStateToolRunning {
		return
	}
	if current.HookSource != "codex" {
		return
	}
	if !current.LastEventAt.Equal(session.LastEventAt) {
		return
	}
	if strings.TrimSpace(current.CurrentAction) == "" {
		return
	}

	pipeline, ok := d.pipelines[current.CurrentTool]
	if !ok {
		return
	}

	output, err := d.capturePane(session.ID)
	if err != nil || !pipeline.detect(output) {
		return
	}

	current.State = socket.SessionStateWaiting
	current.LastEventAt = d.now()
	d.updates <- socket.SessionUpdate{
		Type:    socket.SessionUpdateUpsert,
		Session: current,
		Reason:  "tmux:approval_prompt:" + current.CurrentTool,
	}
}

func containsMarkers(output string, markers []string, minHits int) bool {
	text := strings.ToLower(output)
	hits := 0
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			hits++
		}
	}
	return hits >= minHits
}
