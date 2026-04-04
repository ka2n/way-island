package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteOverlayPNGCollapsedEmptyState(t *testing.T) {
	vm := buildOverlayViewModel("", panelViewClosed, "")
	path := renderTestOutputPath(t, "empty.png")

	if err := writeOverlayPNG(path, vm); err != nil {
		t.Fatalf("write png: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat png: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty png")
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open png: %v", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if got := img.Bounds().Dy(); got != renderCollapsedHeight {
		t.Fatalf("height = %d, want %d", got, renderCollapsedHeight)
	}
}

func TestWriteOverlayPNGListState(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "waiting", Action: "Approval needed"},
		{ID: "session-2", Name: "Beta", State: "working", Action: ""},
	})
	vm := buildOverlayViewModel(payload, panelViewList, "")
	path := renderTestOutputPath(t, "list.png")

	if err := writeOverlayPNG(path, vm); err != nil {
		t.Fatalf("write png: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open png: %v", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	wantHeight := renderCollapsedHeight + renderListHeaderGap + len(vm.ListRows)*renderListRowHeight + 12
	if got := img.Bounds().Dy(); got != wantHeight {
		t.Fatalf("height = %d, want %d", got, wantHeight)
	}
}

func TestWriteOverlayPNGDetailState(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "working", Action: "Running tests"},
		{ID: "session-2", Name: "Beta", State: "idle", Action: ""},
	})
	vm := buildOverlayViewModel(payload, panelViewDetail, "session-1")
	path := renderTestOutputPath(t, "detail.png")

	if err := writeOverlayPNG(path, vm); err != nil {
		t.Fatalf("write png: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open png: %v", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	wantHeight := renderCollapsedHeight + renderDetailHeight
	if got := img.Bounds().Dy(); got != wantHeight {
		t.Fatalf("height = %d, want %d", got, wantHeight)
	}
}

func renderTestOutputPath(t *testing.T, name string) string {
	t.Helper()

	if dir := os.Getenv("WAY_ISLAND_RENDER_TEST_OUTPUT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir render output dir: %v", err)
		}
		path := filepath.Join(dir, name)
		t.Logf("writing render output to %s", path)
		return path
	}

	return filepath.Join(t.TempDir(), name)
}
