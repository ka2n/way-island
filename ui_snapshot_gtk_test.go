//go:build gtk4

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGTKSnapshotEmptyState(t *testing.T) {
	assertGTKSnapshotMatches(t, "gtk-empty.png", "", panelViewClosed, "")
}

func TestGTKSnapshotListState(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "waiting", Action: "Approval needed"},
		{ID: "session-2", Name: "Beta", State: "working", Action: ""},
	})
	assertGTKSnapshotMatches(t, "gtk-list.png", payload, panelViewList, "")
}

func TestGTKSnapshotDetailState(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "tool_running", Action: "Bash: go test ./..."},
		{ID: "session-2", Name: "Beta", State: "idle", Action: ""},
	})
	assertGTKSnapshotMatches(t, "gtk-detail.png", payload, panelViewDetail, "session-1")
}

func assertGTKSnapshotMatches(t *testing.T, snapshotName, payload string, panelView int, selectedSessionID string) {
	t.Helper()

	actualPath := renderTestOutputPath(t, snapshotName)
	if err := saveGTKSnapshot(actualPath, payload, panelView, selectedSessionID); err != nil {
		t.Skipf("skip GTK snapshot test: %v", err)
	}

	expectedPath := filepath.Join("testdata", "snapshots", snapshotName)
	if os.Getenv("WAY_ISLAND_ACCEPT_SNAPSHOTS") == "1" {
		if err := updateSnapshotBaseline(expectedPath, actualPath); err != nil {
			t.Fatalf("update snapshot baseline: %v", err)
		}
	}
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("missing expected snapshot %s; run with WAY_ISLAND_ACCEPT_SNAPSHOTS=1", expectedPath)
	}

	diffPath := renderTestOutputPath(t, snapshotName+".diff.png")
	result, err := compareSnapshotImages(expectedPath, actualPath, diffPath, 0.1)
	if err != nil {
		t.Fatalf("compare snapshot: %v", err)
	}
	if result.DiffRatio > defaultSnapshotDiffRatio {
		t.Fatalf("snapshot diff too large: ratio=%.4f diff_pixels=%d diff=%s actual=%s expected=%s",
			result.DiffRatio, result.DiffPixels, result.DiffPath, actualPath, expectedPath)
	}
}

func updateSnapshotBaseline(expectedPath, actualPath string) error {
	data, err := os.ReadFile(actualPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(expectedPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(expectedPath, data, 0o644)
}
