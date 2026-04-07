//go:build gtk4

package gtkmini

/*
#cgo pkg-config: gtk4 gtk4-layer-shell-0

#include <gtk/gtk.h>
#include <gtk4-layer-shell.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

extern void gtkminiInvokeVoid(uintptr_t handle);
extern gboolean gtkminiInvokeSource(uintptr_t handle);
extern gboolean gtkminiInvokeTick(uintptr_t handle, gint64 frame_time);
extern void gtkminiHandleDelete(uintptr_t handle);

GtkWidget *way_island_shell_new(void);
void way_island_shell_set_child(GtkWidget *shell, GtkWidget *child);
GtkWidget *way_island_clip_new(void);
void way_island_clip_set_child(GtkWidget *clip, GtkWidget *child);
void way_island_clip_set_height(GtkWidget *clip, int clip_height);
GtkWidget *way_island_slide_new(void);
void way_island_slide_set_list_child(GtkWidget *slide, GtkWidget *child);
void way_island_slide_set_detail_child(GtkWidget *slide, GtkWidget *child);
void way_island_slide_set_showing_detail(GtkWidget *slide, gboolean showing_detail);
void way_island_slide_set_progress(GtkWidget *slide, double progress);

static void gtkmini_destroy_notify(gpointer data, GClosure *closure) {
	(void)closure;
	gtkminiHandleDelete((uintptr_t)data);
}

static void gtkmini_activate_cb(GtkApplication *app, gpointer data) {
	(void)app;
	gtkminiInvokeVoid((uintptr_t)data);
}

static gboolean gtkmini_source_cb(gpointer data) {
	return gtkminiInvokeSource((uintptr_t)data);
}

static gboolean gtkmini_tick_cb(GtkWidget *widget, GdkFrameClock *frame_clock, gpointer data) {
	(void)widget;
	return gtkminiInvokeTick((uintptr_t)data, gdk_frame_clock_get_frame_time(frame_clock));
}

static void gtkmini_click_cb(GtkGestureClick *gesture, gint n_press, gdouble x, gdouble y, gpointer data) {
	(void)gesture;
	(void)n_press;
	(void)x;
	(void)y;
	gtkminiInvokeVoid((uintptr_t)data);
}

static void gtkmini_long_press_cb(GtkGestureLongPress *gesture, gdouble x, gdouble y, gpointer data) {
	(void)gesture;
	(void)x;
	(void)y;
	gtkminiInvokeVoid((uintptr_t)data);
}

static void gtkmini_button_clicked_cb(GtkButton *button, gpointer data) {
	(void)button;
	gtkminiInvokeVoid((uintptr_t)data);
}

static void gtkmini_hover_enter_cb(GtkEventControllerMotion *controller, double x, double y, gpointer data) {
	(void)controller;
	(void)x;
	(void)y;
	gtkminiInvokeVoid((uintptr_t)data);
}

static void gtkmini_hover_leave_cb(GtkEventControllerMotion *controller, gpointer data) {
	(void)controller;
	gtkminiInvokeVoid((uintptr_t)data);
}

static void gtkmini_notify_child_revealed_cb(GObject *object, GParamSpec *pspec, gpointer data) {
	(void)object;
	(void)pspec;
	gtkminiInvokeVoid((uintptr_t)data);
}

static void gtkmini_notify_transition_running_cb(GObject *object, GParamSpec *pspec, gpointer data) {
	(void)object;
	(void)pspec;
	gtkminiInvokeVoid((uintptr_t)data);
}

static void gtkmini_window_notify_is_active_cb(GObject *object, GParamSpec *pspec, gpointer data) {
	(void)object;
	(void)pspec;
	gtkminiInvokeVoid((uintptr_t)data);
}

static void gtkmini_popover_closed_cb(GtkPopover *popover, gpointer data) {
	(void)popover;
	gtkminiInvokeVoid((uintptr_t)data);
}

static GtkApplication *gtkmini_application_new(const char *app_id) {
	return gtk_application_new(app_id, G_APPLICATION_DEFAULT_FLAGS);
}

static void gtkmini_application_connect_activate(GtkApplication *app, uintptr_t data) {
	g_signal_connect_data(app, "activate", G_CALLBACK(gtkmini_activate_cb), (gpointer)data, gtkmini_destroy_notify, 0);
}

static GtkWindow *gtkmini_application_window_new(GtkApplication *app) {
	return GTK_WINDOW(gtk_application_window_new(app));
}

static GtkWindow *gtkmini_window_new(void) {
	return GTK_WINDOW(gtk_window_new());
}

static GtkWidget *gtkmini_window_widget(GtkWindow *window) {
	return GTK_WIDGET(window);
}

static void gtkmini_window_set_title(GtkWindow *window, const char *title) {
	gtk_window_set_title(window, title);
}

static void gtkmini_window_set_resizable(GtkWindow *window, gboolean resizable) {
	gtk_window_set_resizable(window, resizable);
}

static void gtkmini_window_set_decorated(GtkWindow *window, gboolean decorated) {
	gtk_window_set_decorated(window, decorated);
}

static void gtkmini_window_set_default_size(GtkWindow *window, gint width, gint height) {
	gtk_window_set_default_size(window, width, height);
}

static void gtkmini_window_set_child(GtkWindow *window, GtkWidget *child) {
	gtk_window_set_child(window, child);
}

static void gtkmini_window_present(GtkWindow *window) {
	gtk_window_present(window);
}

static gboolean gtkmini_window_is_active(GtkWindow *window) {
	return gtk_window_is_active(window);
}

static void gtkmini_window_connect_notify_is_active(GtkWindow *window, uintptr_t data) {
	g_signal_connect_data(window, "notify::is-active", G_CALLBACK(gtkmini_window_notify_is_active_cb), (gpointer)data, gtkmini_destroy_notify, 0);
}

static void gtkmini_application_quit(GtkApplication *app) {
	g_application_quit(G_APPLICATION(app));
}

static int gtkmini_application_run(GtkApplication *app) {
	return g_application_run(G_APPLICATION(app), 0, NULL);
}

static void gtkmini_object_unref(gpointer object) {
	if (object != NULL) {
		g_object_unref(object);
	}
}

static GtkCssProvider *gtkmini_css_provider_new(void) {
	return gtk_css_provider_new();
}

static void gtkmini_css_provider_load_from_string(GtkCssProvider *provider, const char *css) {
	gtk_css_provider_load_from_string(provider, css != NULL ? css : "");
}

static void gtkmini_style_add_provider_for_display(GtkWidget *widget, GtkCssProvider *provider) {
	GdkDisplay *display = gtk_widget_get_display(widget);
	if (display == NULL) {
		return;
	}
	gtk_style_context_add_provider_for_display(
		display,
		GTK_STYLE_PROVIDER(provider),
		GTK_STYLE_PROVIDER_PRIORITY_APPLICATION
	);
}

static GtkWidget *gtkmini_box_new(GtkOrientation orientation, gint spacing) {
	return gtk_box_new(orientation, spacing);
}

static void gtkmini_box_append(GtkWidget *box, GtkWidget *child) {
	gtk_box_append(GTK_BOX(box), child);
}

static void gtkmini_box_remove_all(GtkWidget *box) {
	GtkWidget *child = gtk_widget_get_first_child(box);
	while (child != NULL) {
		GtkWidget *next = gtk_widget_get_next_sibling(child);
		gtk_box_remove(GTK_BOX(box), child);
		child = next;
	}
}

static GtkWidget *gtkmini_label_new(const char *text) {
	return gtk_label_new(text);
}

static void gtkmini_label_set_xalign(GtkWidget *label, gfloat xalign) {
	gtk_label_set_xalign(GTK_LABEL(label), xalign);
}

static void gtkmini_label_set_text(GtkWidget *label, const char *text) {
	gtk_label_set_text(GTK_LABEL(label), text != NULL ? text : "");
}

static void gtkmini_label_set_wrap(GtkWidget *label, gboolean wrap) {
	gtk_label_set_wrap(GTK_LABEL(label), wrap);
}

static void gtkmini_label_set_ellipsize_end(GtkWidget *label) {
	gtk_label_set_ellipsize(GTK_LABEL(label), PANGO_ELLIPSIZE_END);
}

static void gtkmini_label_set_max_width_chars(GtkWidget *label, gint value) {
	gtk_label_set_max_width_chars(GTK_LABEL(label), value);
}

static GtkWidget *gtkmini_image_new_from_icon_name(const char *icon_name) {
	return gtk_image_new_from_icon_name(icon_name);
}

static GtkWidget *gtkmini_button_new_with_label(const char *label) {
	return gtk_button_new_with_label(label);
}

static void gtkmini_button_connect_clicked(GtkWidget *button, uintptr_t data) {
	g_signal_connect_data(button, "clicked", G_CALLBACK(gtkmini_button_clicked_cb), (gpointer)data, gtkmini_destroy_notify, 0);
}

static GtkWidget *gtkmini_revealer_new(void) {
	return gtk_revealer_new();
}

static void gtkmini_revealer_set_transition_type(GtkWidget *revealer, GtkRevealerTransitionType transition) {
	gtk_revealer_set_transition_type(GTK_REVEALER(revealer), transition);
}

static void gtkmini_revealer_set_transition_duration(GtkWidget *revealer, guint duration_ms) {
	gtk_revealer_set_transition_duration(GTK_REVEALER(revealer), duration_ms);
}

static void gtkmini_revealer_set_reveal_child(GtkWidget *revealer, gboolean reveal) {
	gtk_revealer_set_reveal_child(GTK_REVEALER(revealer), reveal);
}

static gboolean gtkmini_revealer_get_reveal_child(GtkWidget *revealer) {
	return gtk_revealer_get_reveal_child(GTK_REVEALER(revealer));
}

static gboolean gtkmini_revealer_get_child_revealed(GtkWidget *revealer) {
	return gtk_revealer_get_child_revealed(GTK_REVEALER(revealer));
}

static void gtkmini_revealer_set_child(GtkWidget *revealer, GtkWidget *child) {
	gtk_revealer_set_child(GTK_REVEALER(revealer), child);
}

static void gtkmini_revealer_connect_child_revealed_notify(GtkWidget *revealer, uintptr_t data) {
	g_signal_connect_data(revealer, "notify::child-revealed", G_CALLBACK(gtkmini_notify_child_revealed_cb), (gpointer)data, gtkmini_destroy_notify, 0);
}

static GtkWidget *gtkmini_popover_new(GtkWidget *parent) {
	GtkWidget *popover = gtk_popover_new();
	if (parent != NULL) {
		gtk_widget_set_parent(popover, parent);
	}
	gtk_popover_set_autohide(GTK_POPOVER(popover), TRUE);
	gtk_popover_set_has_arrow(GTK_POPOVER(popover), FALSE);
	gtk_popover_set_position(GTK_POPOVER(popover), GTK_POS_BOTTOM);
	return popover;
}

static void gtkmini_popover_set_child(GtkWidget *popover, GtkWidget *child) {
	gtk_popover_set_child(GTK_POPOVER(popover), child);
}

static void gtkmini_popover_popup(GtkWidget *popover) {
	gtk_popover_popup(GTK_POPOVER(popover));
}

static void gtkmini_popover_popdown(GtkWidget *popover) {
	gtk_popover_popdown(GTK_POPOVER(popover));
}

static gboolean gtkmini_popover_get_visible(GtkWidget *popover) {
	return gtk_widget_get_visible(popover);
}

static void gtkmini_popover_connect_closed(GtkWidget *popover, uintptr_t data) {
	g_signal_connect_data(popover, "closed", G_CALLBACK(gtkmini_popover_closed_cb), (gpointer)data, gtkmini_destroy_notify, 0);
}

static GtkWidget *gtkmini_stack_new(void) {
	return gtk_stack_new();
}

static void gtkmini_stack_set_transition_duration(GtkWidget *stack, guint duration_ms) {
	gtk_stack_set_transition_duration(GTK_STACK(stack), duration_ms);
}

static void gtkmini_stack_set_transition_type(GtkWidget *stack, GtkStackTransitionType transition) {
	gtk_stack_set_transition_type(GTK_STACK(stack), transition);
}

static void gtkmini_stack_set_hhomogeneous(GtkWidget *stack, gboolean homogeneous) {
	gtk_stack_set_hhomogeneous(GTK_STACK(stack), homogeneous);
}

static void gtkmini_stack_set_vhomogeneous(GtkWidget *stack, gboolean homogeneous) {
	gtk_stack_set_vhomogeneous(GTK_STACK(stack), homogeneous);
}

static void gtkmini_stack_set_interpolate_size(GtkWidget *stack, gboolean interpolate) {
	gtk_stack_set_interpolate_size(GTK_STACK(stack), interpolate);
}

static void gtkmini_stack_add_named(GtkWidget *stack, GtkWidget *child, const char *name) {
	gtk_stack_add_named(GTK_STACK(stack), child, name);
}

static void gtkmini_stack_set_visible_child_name(GtkWidget *stack, const char *name) {
	gtk_stack_set_visible_child_name(GTK_STACK(stack), name);
}

static gboolean gtkmini_stack_get_transition_running(GtkWidget *stack) {
	return gtk_stack_get_transition_running(GTK_STACK(stack));
}

static void gtkmini_stack_connect_transition_running_notify(GtkWidget *stack, uintptr_t data) {
	g_signal_connect_data(stack, "notify::transition-running", G_CALLBACK(gtkmini_notify_transition_running_cb), (gpointer)data, gtkmini_destroy_notify, 0);
}

static void gtkmini_widget_add_css_class(GtkWidget *widget, const char *css_class) {
	gtk_widget_add_css_class(widget, css_class);
}

static void gtkmini_widget_remove_css_class(GtkWidget *widget, const char *css_class) {
	gtk_widget_remove_css_class(widget, css_class);
}

static void gtkmini_widget_set_hexpand(GtkWidget *widget, gboolean expand) {
	gtk_widget_set_hexpand(widget, expand);
}

static void gtkmini_widget_set_vexpand(GtkWidget *widget, gboolean expand) {
	gtk_widget_set_vexpand(widget, expand);
}

static void gtkmini_widget_set_halign(GtkWidget *widget, GtkAlign align) {
	gtk_widget_set_halign(widget, align);
}

static void gtkmini_widget_set_valign(GtkWidget *widget, GtkAlign align) {
	gtk_widget_set_valign(widget, align);
}

static void gtkmini_widget_set_visible(GtkWidget *widget, gboolean visible) {
	gtk_widget_set_visible(widget, visible);
}

static void gtkmini_widget_set_opacity(GtkWidget *widget, double opacity) {
	gtk_widget_set_opacity(widget, opacity);
}

static void gtkmini_widget_set_tooltip_text(GtkWidget *widget, const char *text) {
	gtk_widget_set_tooltip_text(widget, text);
}

static void gtkmini_widget_copy_to_clipboard(GtkWidget *widget, const char *text) {
	GdkDisplay *display = gtk_widget_get_display(widget);
	if (display == NULL) {
		return;
	}
	GdkClipboard *clipboard = gdk_display_get_clipboard(display);
	if (clipboard == NULL) {
		return;
	}
	gdk_clipboard_set_text(clipboard, text != NULL ? text : "");
}

static void gtkmini_widget_queue_resize(GtkWidget *widget) {
	gtk_widget_queue_resize(widget);
}

static gint gtkmini_widget_get_width(GtkWidget *widget) {
	return gtk_widget_get_width(widget);
}

static gint gtkmini_widget_get_height(GtkWidget *widget) {
	return gtk_widget_get_height(widget);
}

static gint gtkmini_widget_measure_natural_height(GtkWidget *widget, gint for_width) {
	gint minimum = 0;
	gint natural = 0;
	gtk_widget_measure(widget, GTK_ORIENTATION_VERTICAL, for_width, &minimum, &natural, NULL, NULL);
	return natural;
}

static gint gtkmini_widget_measure_min_width(GtkWidget *widget) {
	gint minimum = 0;
	gint natural = 0;
	gtk_widget_measure(widget, GTK_ORIENTATION_HORIZONTAL, -1, &minimum, &natural, NULL, NULL);
	return minimum;
}

static gint gtkmini_widget_measure_natural_width(GtkWidget *widget) {
	gint minimum = 0;
	gint natural = 0;
	gtk_widget_measure(widget, GTK_ORIENTATION_HORIZONTAL, -1, &minimum, &natural, NULL, NULL);
	return natural;
}

static void gtkmini_widget_set_size_request(GtkWidget *widget, gint width, gint height) {
	gtk_widget_set_size_request(widget, width, height);
}

static void gtkmini_widget_set_overflow_hidden(GtkWidget *widget) {
	gtk_widget_set_overflow(widget, GTK_OVERFLOW_HIDDEN);
}

static void gtkmini_widget_set_margin_top(GtkWidget *widget, gint margin) {
	gtk_widget_set_margin_top(widget, margin);
}

static void gtkmini_widget_add_click_controller(GtkWidget *widget, uintptr_t data) {
	GtkGesture *click = gtk_gesture_click_new();
	g_signal_connect_data(click, "released", G_CALLBACK(gtkmini_click_cb), (gpointer)data, gtkmini_destroy_notify, 0);
	gtk_widget_add_controller(widget, GTK_EVENT_CONTROLLER(click));
}

static void gtkmini_widget_add_long_press_controller(GtkWidget *widget, uintptr_t data) {
	GtkGesture *lp = gtk_gesture_long_press_new();
	g_signal_connect_data(lp, "pressed", G_CALLBACK(gtkmini_long_press_cb), (gpointer)data, gtkmini_destroy_notify, 0);
	gtk_widget_add_controller(widget, GTK_EVENT_CONTROLLER(lp));
}

static void gtkmini_widget_add_hover_controller(GtkWidget *widget, uintptr_t enter_data, uintptr_t leave_data) {
	GtkEventController *motion = gtk_event_controller_motion_new();
	g_signal_connect_data(motion, "enter", G_CALLBACK(gtkmini_hover_enter_cb), (gpointer)enter_data, gtkmini_destroy_notify, 0);
	g_signal_connect_data(motion, "leave", G_CALLBACK(gtkmini_hover_leave_cb), (gpointer)leave_data, gtkmini_destroy_notify, 0);
	gtk_widget_add_controller(widget, motion);
}

static guint gtkmini_idle_add(uintptr_t data) {
	return g_idle_add_full(G_PRIORITY_DEFAULT, gtkmini_source_cb, (gpointer)data, NULL);
}

static guint gtkmini_timeout_add(guint interval_ms, uintptr_t data) {
	return g_timeout_add(interval_ms, gtkmini_source_cb, (gpointer)data);
}

static guint gtkmini_widget_add_tick_callback(GtkWidget *widget, uintptr_t data) {
	return gtk_widget_add_tick_callback(widget, gtkmini_tick_cb, (gpointer)data, NULL);
}

static void gtkmini_widget_remove_tick_callback(GtkWidget *widget, guint id) {
	gtk_widget_remove_tick_callback(widget, id);
}

static gboolean gtkmini_source_remove(guint source_id) {
	return g_source_remove(source_id);
}

static gboolean gtkmini_init_check(void) {
	return gtk_init_check();
}

static void gtkmini_apply_css(GtkWidget *widget, const char *css) {
	GtkCssProvider *provider = gtk_css_provider_new();
	GdkDisplay *display = gtk_widget_get_display(widget);

	if (display == NULL) {
		g_object_unref(provider);
		return;
	}

	gtk_css_provider_load_from_string(provider, css != NULL ? css : "");
	gtk_style_context_add_provider_for_display(
		display,
		GTK_STYLE_PROVIDER(provider),
		GTK_STYLE_PROVIDER_PRIORITY_APPLICATION
	);
	g_object_unref(provider);
}

static void gtkmini_pump_events(gint iterations) {
	for (gint i = 0; i < iterations; i++) {
		while (g_main_context_iteration(NULL, FALSE)) {
		}
		g_usleep(10000);
	}
}

static gboolean gtkmini_save_widget_png(GtkWindow *window, GtkWidget *widget, const char *filename) {
	gtk_window_set_child(window, widget);
	gtk_window_present(window);
	gtkmini_pump_events(8);

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

static gboolean gtkmini_layer_is_supported(void) {
	return gtk_layer_is_supported();
}

static gboolean gtkmini_display_get_first_monitor_size(GtkWidget *widget, gint *width, gint *height) {
	GdkDisplay *display;
	GListModel *monitors;
	GdkMonitor *monitor;
	GdkRectangle geometry;

	if (widget == NULL || width == NULL || height == NULL) {
		return FALSE;
	}

	display = gtk_widget_get_display(widget);
	if (display == NULL) {
		return FALSE;
	}

	monitors = gdk_display_get_monitors(display);
	if (monitors == NULL || g_list_model_get_n_items(monitors) == 0) {
		return FALSE;
	}

	monitor = GDK_MONITOR(g_list_model_get_item(monitors, 0));
	if (monitor == NULL) {
		return FALSE;
	}

	gdk_monitor_get_geometry(monitor, &geometry);
	g_object_unref(monitor);

	*width = geometry.width;
	*height = geometry.height;
	return geometry.width > 0 && geometry.height > 0;
}

static void gtkmini_layer_init_for_window(GtkWindow *window) {
	gtk_layer_init_for_window(window);
}

static void gtkmini_layer_configure_top_window(GtkWindow *window) {
	gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_TOP);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_TOP, TRUE);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_LEFT, FALSE);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_RIGHT, FALSE);
	gtk_layer_set_keyboard_mode(window, GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
	gtk_layer_set_exclusive_zone(window, 0);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_TOP, 0);
}

static void gtkmini_layer_configure_overlay_window(GtkWindow *window) {
	gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_OVERLAY);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_TOP, TRUE);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_LEFT, FALSE);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_RIGHT, FALSE);
	gtk_layer_set_keyboard_mode(window, GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
	gtk_layer_set_exclusive_zone(window, 0);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_TOP, 0);
}

static void gtkmini_layer_configure_fullscreen_window(GtkWindow *window) {
	gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_TOP);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_TOP, TRUE);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_BOTTOM, TRUE);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_LEFT, TRUE);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_RIGHT, TRUE);
	gtk_layer_set_keyboard_mode(window, GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
	gtk_layer_set_exclusive_zone(window, 0);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_TOP, 0);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_BOTTOM, 0);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_LEFT, 0);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_RIGHT, 0);
}
*/
import "C"

import (
	"runtime/cgo"
	"sync"
	"time"
	"unsafe"
)

type SourceID uint
type TickCallbackID uint

type Application struct {
	ptr *C.GtkApplication
}

type Window struct {
	ptr *C.GtkWindow
}

type CSSProvider struct {
	ptr *C.GtkCssProvider
}

type Widget struct {
	ptr *C.GtkWidget
}

type Orientation int

const (
	OrientationHorizontal Orientation = iota
	OrientationVertical
)

type Align int

const (
	AlignFill Align = iota
	AlignStart
	AlignEnd
	AlignCenter
)

type StackTransition int

const (
	StackTransitionNone           StackTransition = 0
	StackTransitionCrossfade      StackTransition = 1
	StackTransitionSlideRight     StackTransition = 4
	StackTransitionSlideLeft      StackTransition = 5
	StackTransitionSlideLeftRight StackTransition = 6
)

type RevealerTransition int

const (
	RevealerTransitionNone      RevealerTransition = 0
	RevealerTransitionCrossfade RevealerTransition = 1
	RevealerTransitionSlideDown RevealerTransition = 2
)

var sourceHandles sync.Map
var tickCallbackHandles sync.Map

func cString(value string) (*C.char, func()) {
	cvalue := C.CString(value)
	return cvalue, func() {
		C.free(unsafe.Pointer(cvalue))
	}
}

func boolToGBoolean(value bool) C.gboolean {
	if value {
		return 1
	}
	return 0
}

func newHandle(fn any) C.uintptr_t {
	return C.uintptr_t(cgo.NewHandle(fn))
}

//export gtkminiInvokeVoid
func gtkminiInvokeVoid(handle C.uintptr_t) {
	cgo.Handle(handle).Value().(func())()
}

//export gtkminiInvokeSource
func gtkminiInvokeSource(handle C.uintptr_t) C.gboolean {
	h := cgo.Handle(handle)
	keep := h.Value().(func() bool)()
	if keep {
		return 1
	}

	h.Delete()
	var matchedKey any
	sourceHandles.Range(func(key, value any) bool {
		if value == handle {
			matchedKey = key
			return false
		}
		return true
	})
	if matchedKey != nil {
		sourceHandles.Delete(matchedKey)
	}

	return 0
}

//export gtkminiInvokeTick
func gtkminiInvokeTick(handle C.uintptr_t, frameTime C.gint64) C.gboolean {
	h := cgo.Handle(handle)
	keep := h.Value().(func(int64) bool)(int64(frameTime))
	if keep {
		return 1
	}

	h.Delete()
	var matchedKey any
	tickCallbackHandles.Range(func(key, value any) bool {
		if value == handle {
			matchedKey = key
			return false
		}
		return true
	})
	if matchedKey != nil {
		tickCallbackHandles.Delete(matchedKey)
	}

	return 0
}

//export gtkminiHandleDelete
func gtkminiHandleDelete(handle C.uintptr_t) {
	cgo.Handle(handle).Delete()
}

func NewApplication(appID string) *Application {
	cappID, free := cString(appID)
	defer free()
	return &Application{ptr: C.gtkmini_application_new(cappID)}
}

func (a *Application) ConnectActivate(fn func()) {
	C.gtkmini_application_connect_activate(a.ptr, newHandle(fn))
}

func (a *Application) Run() int {
	return int(C.gtkmini_application_run(a.ptr))
}

func (a *Application) Quit() {
	C.gtkmini_application_quit(a.ptr)
}

func (a *Application) Unref() {
	C.gtkmini_object_unref(C.gpointer(unsafe.Pointer(a.ptr)))
}

func NewApplicationWindow(app *Application) *Window {
	return &Window{ptr: C.gtkmini_application_window_new(app.ptr)}
}

func NewTestWindow() *Window {
	return &Window{ptr: C.gtkmini_window_new()}
}

func (w *Window) Widget() *Widget {
	return &Widget{ptr: C.gtkmini_window_widget(w.ptr)}
}

func (w *Window) SetTitle(title string) {
	ctitle, free := cString(title)
	defer free()
	C.gtkmini_window_set_title(w.ptr, ctitle)
}

func (w *Window) SetResizable(value bool) {
	C.gtkmini_window_set_resizable(w.ptr, boolToGBoolean(value))
}

func (w *Window) SetDecorated(value bool) {
	C.gtkmini_window_set_decorated(w.ptr, boolToGBoolean(value))
}

func (w *Window) SetDefaultSize(width, height int) {
	C.gtkmini_window_set_default_size(w.ptr, C.gint(width), C.gint(height))
}

func (w *Window) SetChild(child *Widget) {
	C.gtkmini_window_set_child(w.ptr, child.ptr)
}

func (w *Window) Present() {
	C.gtkmini_window_present(w.ptr)
}

func (w *Window) IsActive() bool {
	return C.gtkmini_window_is_active(w.ptr) != 0
}

func (w *Window) ConnectNotifyIsActive(fn func()) {
	C.gtkmini_window_connect_notify_is_active(w.ptr, newHandle(fn))
}

func NewCSSProvider() *CSSProvider {
	return &CSSProvider{ptr: C.gtkmini_css_provider_new()}
}

func (p *CSSProvider) LoadFromString(css string) {
	ccss, free := cString(css)
	defer free()
	C.gtkmini_css_provider_load_from_string(p.ptr, ccss)
}

func (p *CSSProvider) Unref() {
	C.gtkmini_object_unref(C.gpointer(unsafe.Pointer(p.ptr)))
}

func AddProviderForDisplay(widget *Widget, provider *CSSProvider) {
	C.gtkmini_style_add_provider_for_display(widget.ptr, provider.ptr)
}

func NewShell() *Widget {
	return &Widget{ptr: C.way_island_shell_new()}
}

func ShellSetChild(shell, child *Widget) {
	C.way_island_shell_set_child(shell.ptr, child.ptr)
}

func NewClip() *Widget {
	return &Widget{ptr: C.way_island_clip_new()}
}

func ClipSetChild(clip, child *Widget) {
	C.way_island_clip_set_child(clip.ptr, child.ptr)
}

func (w *Widget) ClipSetHeight(height int) {
	C.way_island_clip_set_height(w.ptr, C.int(height))
}

func NewSlide() *Widget {
	return &Widget{ptr: C.way_island_slide_new()}
}

func SlideSetListChild(slide, child *Widget) {
	C.way_island_slide_set_list_child(slide.ptr, child.ptr)
}

func SlideSetDetailChild(slide, child *Widget) {
	C.way_island_slide_set_detail_child(slide.ptr, child.ptr)
}

func (w *Widget) SlideSetShowingDetail(showing bool) {
	C.way_island_slide_set_showing_detail(w.ptr, boolToGBoolean(showing))
}

func (w *Widget) SlideSetProgress(progress float64) {
	C.way_island_slide_set_progress(w.ptr, C.double(progress))
}

func NewBox(orientation Orientation, spacing int) *Widget {
	return &Widget{ptr: C.gtkmini_box_new(C.GtkOrientation(orientation), C.gint(spacing))}
}

func (w *Widget) Append(child *Widget) {
	C.gtkmini_box_append(w.ptr, child.ptr)
}

func (w *Widget) ClearBoxChildren() {
	C.gtkmini_box_remove_all(w.ptr)
}

func NewLabel(text string) *Widget {
	ctext, free := cString(text)
	defer free()
	return &Widget{ptr: C.gtkmini_label_new(ctext)}
}

func (w *Widget) LabelSetXAlign(value float32) {
	C.gtkmini_label_set_xalign(w.ptr, C.gfloat(value))
}

func (w *Widget) LabelSetText(value string) {
	cvalue, free := cString(value)
	defer free()
	C.gtkmini_label_set_text(w.ptr, cvalue)
}

func (w *Widget) LabelSetWrap(value bool) {
	C.gtkmini_label_set_wrap(w.ptr, boolToGBoolean(value))
}

func (w *Widget) LabelSetEllipsizeEnd() {
	C.gtkmini_label_set_ellipsize_end(w.ptr)
}

func (w *Widget) LabelSetMaxWidthChars(value int) {
	C.gtkmini_label_set_max_width_chars(w.ptr, C.gint(value))
}

func NewImageFromIconName(iconName string) *Widget {
	cicon, free := cString(iconName)
	defer free()
	return &Widget{ptr: C.gtkmini_image_new_from_icon_name(cicon)}
}

func NewButtonWithLabel(label string) *Widget {
	clabel, free := cString(label)
	defer free()
	return &Widget{ptr: C.gtkmini_button_new_with_label(clabel)}
}

func (w *Widget) ConnectButtonClicked(fn func()) {
	C.gtkmini_button_connect_clicked(w.ptr, newHandle(fn))
}

func NewRevealer() *Widget {
	return &Widget{ptr: C.gtkmini_revealer_new()}
}

func (w *Widget) RevealerSetTransitionType(transition RevealerTransition) {
	C.gtkmini_revealer_set_transition_type(w.ptr, C.GtkRevealerTransitionType(transition))
}

func (w *Widget) RevealerSetTransitionDuration(durationMS uint) {
	C.gtkmini_revealer_set_transition_duration(w.ptr, C.guint(durationMS))
}

func (w *Widget) RevealerSetRevealChild(value bool) {
	C.gtkmini_revealer_set_reveal_child(w.ptr, boolToGBoolean(value))
}

func (w *Widget) RevealerGetRevealChild() bool {
	return C.gtkmini_revealer_get_reveal_child(w.ptr) != 0
}

func (w *Widget) RevealerGetChildRevealed() bool {
	return C.gtkmini_revealer_get_child_revealed(w.ptr) != 0
}

func (w *Widget) RevealerSetChild(child *Widget) {
	C.gtkmini_revealer_set_child(w.ptr, child.ptr)
}

func (w *Widget) ConnectNotifyChildRevealed(fn func()) {
	C.gtkmini_revealer_connect_child_revealed_notify(w.ptr, newHandle(fn))
}

func NewPopover(parent *Widget) *Widget {
	var parentPtr *C.GtkWidget
	if parent != nil {
		parentPtr = parent.ptr
	}
	return &Widget{ptr: C.gtkmini_popover_new(parentPtr)}
}

func (w *Widget) PopoverSetChild(child *Widget) {
	C.gtkmini_popover_set_child(w.ptr, child.ptr)
}

func (w *Widget) PopoverPopup() {
	C.gtkmini_popover_popup(w.ptr)
}

func (w *Widget) PopoverPopdown() {
	C.gtkmini_popover_popdown(w.ptr)
}

func (w *Widget) PopoverVisible() bool {
	return C.gtkmini_popover_get_visible(w.ptr) != 0
}

func (w *Widget) ConnectPopoverClosed(fn func()) {
	C.gtkmini_popover_connect_closed(w.ptr, newHandle(fn))
}

func NewStack() *Widget {
	return &Widget{ptr: C.gtkmini_stack_new()}
}

func (w *Widget) StackSetTransitionDuration(durationMS uint) {
	C.gtkmini_stack_set_transition_duration(w.ptr, C.guint(durationMS))
}

func (w *Widget) StackSetTransitionType(transition StackTransition) {
	C.gtkmini_stack_set_transition_type(w.ptr, C.GtkStackTransitionType(transition))
}

func (w *Widget) StackSetHHomogeneous(value bool) {
	C.gtkmini_stack_set_hhomogeneous(w.ptr, boolToGBoolean(value))
}

func (w *Widget) StackSetVHomogeneous(value bool) {
	C.gtkmini_stack_set_vhomogeneous(w.ptr, boolToGBoolean(value))
}

func (w *Widget) StackSetInterpolateSize(value bool) {
	C.gtkmini_stack_set_interpolate_size(w.ptr, boolToGBoolean(value))
}

func (w *Widget) StackAddNamed(child *Widget, name string) {
	cname, free := cString(name)
	defer free()
	C.gtkmini_stack_add_named(w.ptr, child.ptr, cname)
}

func (w *Widget) StackSetVisibleChildName(name string) {
	cname, free := cString(name)
	defer free()
	C.gtkmini_stack_set_visible_child_name(w.ptr, cname)
}

func (w *Widget) StackGetTransitionRunning() bool {
	return C.gtkmini_stack_get_transition_running(w.ptr) != 0
}

func (w *Widget) ConnectNotifyStackTransitionRunning(fn func()) {
	C.gtkmini_stack_connect_transition_running_notify(w.ptr, newHandle(fn))
}

func (w *Widget) AddCSSClass(cssClass string) {
	ccss, free := cString(cssClass)
	defer free()
	C.gtkmini_widget_add_css_class(w.ptr, ccss)
}

func (w *Widget) RemoveCSSClass(cssClass string) {
	ccss, free := cString(cssClass)
	defer free()
	C.gtkmini_widget_remove_css_class(w.ptr, ccss)
}

func (w *Widget) SetHexpand(value bool) {
	C.gtkmini_widget_set_hexpand(w.ptr, boolToGBoolean(value))
}

func (w *Widget) SetVexpand(value bool) {
	C.gtkmini_widget_set_vexpand(w.ptr, boolToGBoolean(value))
}

func (w *Widget) SetHAlign(value Align) {
	C.gtkmini_widget_set_halign(w.ptr, C.GtkAlign(value))
}

func (w *Widget) SetVAlign(value Align) {
	C.gtkmini_widget_set_valign(w.ptr, C.GtkAlign(value))
}

func (w *Widget) SetVisible(value bool) {
	C.gtkmini_widget_set_visible(w.ptr, boolToGBoolean(value))
}

func (w *Widget) SetOpacity(value float64) {
	C.gtkmini_widget_set_opacity(w.ptr, C.double(value))
}

func (w *Widget) SetTooltipText(text string) {
	ctext, free := cString(text)
	defer free()
	C.gtkmini_widget_set_tooltip_text(w.ptr, ctext)
}

func (w *Widget) CopyToClipboard(text string) {
	ctext, free := cString(text)
	defer free()
	C.gtkmini_widget_copy_to_clipboard(w.ptr, ctext)
}

func (w *Widget) QueueResize() {
	C.gtkmini_widget_queue_resize(w.ptr)
}

func (w *Widget) Width() int {
	return int(C.gtkmini_widget_get_width(w.ptr))
}

func (w *Widget) Height() int {
	return int(C.gtkmini_widget_get_height(w.ptr))
}

func (w *Widget) MeasureNaturalHeight(forWidth int) int {
	return int(C.gtkmini_widget_measure_natural_height(w.ptr, C.gint(forWidth)))
}

func (w *Widget) MeasureMinWidth() int {
	return int(C.gtkmini_widget_measure_min_width(w.ptr))
}

func (w *Widget) MeasureNaturalWidth() int {
	return int(C.gtkmini_widget_measure_natural_width(w.ptr))
}

func (w *Widget) SetSizeRequest(width, height int) {
	C.gtkmini_widget_set_size_request(w.ptr, C.gint(width), C.gint(height))
}

func (w *Widget) SetOverflowHidden() {
	C.gtkmini_widget_set_overflow_hidden(w.ptr)
}

func (w *Widget) SetMarginTop(value int) {
	C.gtkmini_widget_set_margin_top(w.ptr, C.gint(value))
}

func (w *Widget) ConnectClick(fn func()) {
	C.gtkmini_widget_add_click_controller(w.ptr, newHandle(fn))
}

func (w *Widget) ConnectLongPress(fn func()) {
	C.gtkmini_widget_add_long_press_controller(w.ptr, newHandle(fn))
}

func (w *Widget) ConnectHover(enter, leave func()) {
	C.gtkmini_widget_add_hover_controller(w.ptr, newHandle(enter), newHandle(leave))
}

func (w *Widget) AddTickCallback(fn func(frameTimeMicros int64) bool) TickCallbackID {
	handle := cgo.NewHandle(fn)
	callbackID := TickCallbackID(C.gtkmini_widget_add_tick_callback(w.ptr, C.uintptr_t(handle)))
	tickCallbackHandles.Store(callbackID, C.uintptr_t(handle))
	return callbackID
}

func (w *Widget) AddTimedAnimation(duration time.Duration, fn func(progress float64) bool) TickCallbackID {
	durationMicros := int64(duration / time.Microsecond)
	if durationMicros <= 0 {
		durationMicros = 1
	}

	var startMicros int64
	return w.AddTickCallback(func(frameTimeMicros int64) bool {
		if startMicros == 0 {
			startMicros = frameTimeMicros
		}

		elapsedMicros := frameTimeMicros - startMicros
		if elapsedMicros < 0 {
			elapsedMicros = 0
		}

		progress := float64(elapsedMicros) / float64(durationMicros)
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}

		keep := fn(progress)
		if progress >= 1 {
			return false
		}
		return keep
	})
}

func (w *Widget) RemoveTickCallback(callbackID TickCallbackID) {
	if callbackID == 0 {
		return
	}
	C.gtkmini_widget_remove_tick_callback(w.ptr, C.guint(callbackID))
	if handle, ok := tickCallbackHandles.LoadAndDelete(callbackID); ok {
		cgo.Handle(handle.(C.uintptr_t)).Delete()
	}
}

func IdleAdd(fn func()) SourceID {
	handle := cgo.NewHandle(func() bool {
		fn()
		return false
	})
	sourceID := SourceID(C.gtkmini_idle_add(C.uintptr_t(handle)))
	sourceHandles.Store(sourceID, C.uintptr_t(handle))
	return sourceID
}

func TimeoutAdd(delayMS uint, fn func() bool) SourceID {
	handle := cgo.NewHandle(fn)
	sourceID := SourceID(C.gtkmini_timeout_add(C.guint(delayMS), C.uintptr_t(handle)))
	sourceHandles.Store(sourceID, C.uintptr_t(handle))
	return sourceID
}

func SourceRemove(sourceID SourceID) {
	if sourceID == 0 {
		return
	}
	C.gtkmini_source_remove(C.guint(sourceID))
	if handle, ok := sourceHandles.LoadAndDelete(sourceID); ok {
		cgo.Handle(handle.(C.uintptr_t)).Delete()
	}
}

func InitCheck() bool {
	return C.gtkmini_init_check() != 0
}

func ApplyCSS(widget *Widget, css string) {
	ccss, free := cString(css)
	defer free()
	C.gtkmini_apply_css(widget.ptr, ccss)
}

func PumpEvents(iterations int) {
	if iterations < 0 {
		iterations = 0
	}
	C.gtkmini_pump_events(C.gint(iterations))
}

func SaveWidgetPNG(window *Window, widget *Widget, path string) bool {
	cpath, free := cString(path)
	defer free()
	return C.gtkmini_save_widget_png(window.ptr, widget.ptr, cpath) != 0
}

func LayerShellConfigureTop(window *Window) {
	if C.gtkmini_layer_is_supported() == 0 {
		return
	}

	C.gtkmini_layer_init_for_window(window.ptr)
	C.gtkmini_layer_configure_top_window(window.ptr)
}

func LayerShellConfigureOverlay(window *Window) {
	if C.gtkmini_layer_is_supported() == 0 {
		return
	}

	C.gtkmini_layer_init_for_window(window.ptr)
	C.gtkmini_layer_configure_overlay_window(window.ptr)
}

func LayerShellConfigureFullscreen(window *Window) {
	if C.gtkmini_layer_is_supported() == 0 {
		return
	}

	C.gtkmini_layer_init_for_window(window.ptr)
	C.gtkmini_layer_configure_fullscreen_window(window.ptr)
}

func DisplayFirstMonitorSize(widget *Widget) (int, int, bool) {
	var width C.gint
	var height C.gint
	ok := C.gtkmini_display_get_first_monitor_size(widget.ptr, &width, &height) != 0
	return int(width), int(height), ok
}
