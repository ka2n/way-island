//go:build gtk4

package main

import (
	"errors"
	"runtime"
	"sync"

	"github.com/ka2n/way-island/internal/gtkmini"
)

var gtkSnapshotInitOnce sync.Once
var gtkSnapshotInitOK bool

func saveGTKSnapshot(path, payload string, panelView int, selectedSessionID string) error {
	gtkSnapshotInitOnce.Do(func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		gtkSnapshotInitOK = gtkmini.InitCheck()
	})
	if !gtkSnapshotInitOK {
		return errors.New("GTK could not initialize")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	window := gtkmini.NewTestWindow()
	window.SetTitle("way-island-test")
	window.SetResizable(false)
	window.SetDecorated(false)

	widget := buildGTKWidgetForTest(payload, panelView, selectedSessionID)
	gtkmini.ApplyCSS(window.Widget(), styleCSS)
	if ok := gtkmini.SaveWidgetPNG(window, widget, path); !ok {
		return errors.New("GTK snapshot save failed")
	}

	return nil
}
