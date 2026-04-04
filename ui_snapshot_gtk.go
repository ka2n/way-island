//go:build gtk4

package main

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/ka2n/way-island/internal/gtkmini"
)

var gtkSnapshotInitOnce sync.Once
var gtkSnapshotInitOK bool

func initGTKSnapshotRuntime() error {
	gtkSnapshotInitOnce.Do(func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		gtkSnapshotInitOK = gtkmini.InitCheck()
	})
	if !gtkSnapshotInitOK {
		return errors.New("GTK could not initialize")
	}
	return nil
}

func withGTKTestUI(payload string, panelView int, selectedSessionID string, fn func(window *gtkmini.Window, widget *gtkmini.Widget, ui *gtkUI) error) error {
	if err := initGTKSnapshotRuntime(); err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	window := gtkmini.NewTestWindow()
	window.SetTitle("way-island-test")
	window.SetResizable(false)
	window.SetDecorated(false)

	ui := newGTKUI()
	ui.sessionsPayload = payload
	ui.selectedSessionID = selectedSessionID
	ui.panelView = panelView

	widget := ui.buildShellWidget()
	gtkmini.ApplyCSS(window.Widget(), styleCSS)
	window.SetChild(widget)
	window.Present()
	gtkmini.PumpEvents(8)
	ui.rebuildUI(payload)
	gtkmini.PumpEvents(8)

	if fn == nil {
		return nil
	}
	return fn(window, widget, ui)
}

func saveGTKSnapshot(path, payload string, panelView int, selectedSessionID string) error {
	return withGTKTestUI(payload, panelView, selectedSessionID, func(window *gtkmini.Window, widget *gtkmini.Widget, ui *gtkUI) error {
		if ok := gtkmini.SaveWidgetPNG(window, widget, path); !ok {
			return errors.New("GTK snapshot save failed")
		}
		return nil
	})
}

func (ui *gtkUI) hasPendingAnimation() bool {
	if ui == nil {
		return false
	}
	return ui.panelUpdateSource != 0 ||
		ui.hoverCloseSource != 0 ||
		ui.widthAnimSource != 0 ||
		ui.detailAnimSource != 0 ||
		ui.detailOpenSource != 0 ||
		ui.slideAnimSource != 0 ||
		ui.listTransitionMode != listTransitionNone ||
		ui.closingActive ||
		ui.listDetailAnimating
}

func pumpGTKUntilStable(ui *gtkUI, maxIterations int) (int, error) {
	if maxIterations <= 0 {
		maxIterations = 1
	}
	for i := 0; i < maxIterations; i++ {
		gtkmini.PumpEvents(1)
		if !ui.hasPendingAnimation() {
			return i + 1, nil
		}
	}
	return maxIterations, fmt.Errorf("GTK UI did not settle after %d pump iterations", maxIterations)
}
