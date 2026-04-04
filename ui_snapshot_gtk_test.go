//go:build gtk4

package main

import (
	"image/png"
	"os"
	"testing"
)

func TestGTKSnapshotEmptyState(t *testing.T) {
	path := renderTestOutputPath(t, "gtk-empty.png")
	if err := saveGTKSnapshot(path, "", panelViewClosed, ""); err != nil {
		t.Skipf("skip GTK snapshot test: %v", err)
	}

	width, height := readSnapshotPNGSize(t, path)
	if width <= 0 || height <= 0 {
		t.Fatalf("unexpected snapshot size: %dx%d", width, height)
	}
}

func TestGTKSnapshotListState(t *testing.T) {
	emptyPath := renderTestOutputPath(t, "gtk-empty.png")
	if err := saveGTKSnapshot(emptyPath, "", panelViewClosed, ""); err != nil {
		t.Skipf("skip GTK snapshot test: %v", err)
	}
	emptyWidth, emptyHeight := readSnapshotPNGSize(t, emptyPath)

	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "waiting", Action: "Approval needed"},
		{ID: "session-2", Name: "Beta", State: "working", Action: ""},
	})
	path := renderTestOutputPath(t, "gtk-list.png")
	if err := saveGTKSnapshot(path, payload, panelViewList, ""); err != nil {
		t.Skipf("skip GTK snapshot test: %v", err)
	}

	width, height := readSnapshotPNGSize(t, path)
	if width < emptyWidth {
		t.Fatalf("list width = %d, want at least %d", width, emptyWidth)
	}
	if height <= emptyHeight {
		t.Fatalf("list height = %d, want more than %d", height, emptyHeight)
	}
}

func TestGTKSnapshotDetailState(t *testing.T) {
	emptyPath := renderTestOutputPath(t, "gtk-empty.png")
	if err := saveGTKSnapshot(emptyPath, "", panelViewClosed, ""); err != nil {
		t.Skipf("skip GTK snapshot test: %v", err)
	}
	emptyWidth, emptyHeight := readSnapshotPNGSize(t, emptyPath)

	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "tool_running", Action: "Bash: go test ./..."},
		{ID: "session-2", Name: "Beta", State: "idle", Action: ""},
	})
	path := renderTestOutputPath(t, "gtk-detail.png")
	if err := saveGTKSnapshot(path, payload, panelViewDetail, "session-1"); err != nil {
		t.Skipf("skip GTK snapshot test: %v", err)
	}

	width, height := readSnapshotPNGSize(t, path)
	if width < emptyWidth {
		t.Fatalf("detail width = %d, want at least %d", width, emptyWidth)
	}
	if height <= emptyHeight {
		t.Fatalf("detail height = %d, want more than %d", height, emptyHeight)
	}
}

func readSnapshotPNGSize(t *testing.T, path string) (int, int) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	return img.Bounds().Dx(), img.Bounds().Dy()
}
