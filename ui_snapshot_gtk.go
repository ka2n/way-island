//go:build gtk4

package main

/*
#cgo pkg-config: gtk4 gtk4-layer-shell-0

#include <gtk/gtk.h>
#include <gtk4-layer-shell.h>
#include <stdbool.h>
#include <stdlib.h>

static gboolean way_island_test_init_gtk(void) {
	return gtk_init_check();
}

static GtkWindow *way_island_test_new_window(void) {
	GtkWindow *window = GTK_WINDOW(gtk_window_new());
	gtk_window_set_title(window, "way-island-test");
	gtk_window_set_resizable(window, FALSE);
	gtk_window_set_decorated(window, FALSE);
	return window;
}

static void way_island_test_apply_css(GtkWidget *widget, const char *css) {
	GtkCssProvider *provider = gtk_css_provider_new();
	GdkDisplay *display = gtk_widget_get_display(widget);

	gtk_css_provider_load_from_string(provider, css != NULL ? css : "");
	gtk_style_context_add_provider_for_display(
		display,
		GTK_STYLE_PROVIDER(provider),
		GTK_STYLE_PROVIDER_PRIORITY_APPLICATION
	);
	g_object_unref(provider);
}

static void way_island_test_pump_events(gint iterations) {
	for (gint i = 0; i < iterations; i++) {
		while (g_main_context_iteration(NULL, FALSE)) {
		}
		g_usleep(10000);
	}
}

static gboolean way_island_test_save_widget_png(GtkWindow *window, GtkWidget *widget, const char *filename) {
	gtk_window_set_child(window, widget);
	gtk_window_present(window);
	way_island_test_pump_events(8);

	const int width = gtk_widget_get_width(widget);
	const int height = gtk_widget_get_height(widget);
	if (width <= 0 || height <= 0) {
		return FALSE;
	}

	GdkPaintable *paintable = gtk_widget_paintable_new(widget);
	GtkSnapshot *snapshot = gtk_snapshot_new();
	gdk_paintable_snapshot(paintable, GDK_SNAPSHOT(snapshot), width, height);
	GskRenderNode *node = gtk_snapshot_free_to_node(snapshot);
	if (node == NULL) {
		g_object_unref(paintable);
		return FALSE;
	}

	GskRenderer *renderer = gtk_native_get_renderer(GTK_NATIVE(window));
	if (renderer == NULL) {
		gsk_render_node_unref(node);
		g_object_unref(paintable);
		return FALSE;
	}

	GdkTexture *texture = gsk_renderer_render_texture(renderer, node, NULL);
	gsk_render_node_unref(node);
	g_object_unref(paintable);
	if (texture == NULL) {
		return FALSE;
	}

	gdk_texture_save_to_png(texture, filename);
	g_object_unref(texture);
	return TRUE;
}
*/
import "C"

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"
)

var gtkSnapshotInitOnce sync.Once
var gtkSnapshotInitOK bool

func saveGTKSnapshot(path, payload string, panelView int, selectedSessionID string) error {
	gtkSnapshotInitOnce.Do(func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		gtkSnapshotInitOK = C.way_island_test_init_gtk() != 0
	})
	if !gtkSnapshotInitOK {
		return errors.New("GTK could not initialize")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	css := C.CString(styleCSS)
	defer C.free(unsafe.Pointer(css))
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	window := C.way_island_test_new_window()
	widget := (*C.GtkWidget)(buildGTKWidgetForTest(payload, panelView, selectedSessionID))
	C.way_island_test_apply_css((*C.GtkWidget)(unsafe.Pointer(window)), css)
	if ok := C.way_island_test_save_widget_png(window, widget, cpath); ok == 0 {
		return errors.New("GTK snapshot save failed")
	}

	return nil
}
