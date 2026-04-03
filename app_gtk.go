//go:build gtk4

package main

/*
#cgo pkg-config: gtk4 gtk4-layer-shell-0

#include <gtk/gtk.h>
#include <gtk4-layer-shell.h>
#include <glib.h>
#include <stdlib.h>
#include <string.h>

static GtkApplication *way_island_app = NULL;
static GtkWidget *way_island_root = NULL;
static GtkWidget *way_island_pill = NULL;
static GtkWidget *way_island_revealer = NULL;
static GtkWidget *way_island_detail = NULL;
static gchar *way_island_sessions_payload = NULL;
static gchar *way_island_css_data = NULL;
static gboolean way_island_should_quit = FALSE;
static gboolean way_island_expanded = FALSE;

static char *way_island_decode_field(const char *encoded) {
	gsize decoded_len = 0;
	guchar *decoded = g_base64_decode(encoded, &decoded_len);
	char *text = g_malloc(decoded_len + 1);

	memcpy(text, decoded, decoded_len);
	text[decoded_len] = '\0';
	g_free(decoded);

	return text;
}

static void way_island_clear_children(GtkWidget *widget) {
	GtkWidget *child = gtk_widget_get_first_child(widget);
	while (child != NULL) {
		GtkWidget *next = gtk_widget_get_next_sibling(child);
		gtk_box_remove(GTK_BOX(widget), child);
		child = next;
	}
}

static const char *way_island_status_class(const char *state) {
	if (g_strcmp0(state, "working") == 0) return "working";
	if (g_strcmp0(state, "tool_running") == 0) return "tool-running";
	if (g_strcmp0(state, "waiting") == 0) return "waiting";
	return "idle";
}

static const char *way_island_status_label(const char *state) {
	if (g_strcmp0(state, "working") == 0) return "Working";
	if (g_strcmp0(state, "tool_running") == 0) return "Running tool";
	if (g_strcmp0(state, "waiting") == 0) return "Waiting";
	return "Idle";
}

static gint way_island_count_sessions(const char *payload) {
	gint count = 0;
	if (payload == NULL || payload[0] == '\0') return 0;
	for (const char *p = payload; *p; p++) {
		if (*p == '\n') count++;
	}
	return count;
}

static GtkWidget *way_island_build_session_row(const char *name, const char *state) {
	GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
	gtk_widget_add_css_class(row, "session-row");

	GtkWidget *dot = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
	gtk_widget_add_css_class(dot, "island-status");
	gtk_widget_add_css_class(dot, way_island_status_class(state));
	gtk_widget_set_valign(dot, GTK_ALIGN_CENTER);
	gtk_box_append(GTK_BOX(row), dot);

	GtkWidget *text_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
	gtk_widget_set_hexpand(text_box, TRUE);

	GtkWidget *title = gtk_label_new(name);
	gtk_widget_add_css_class(title, "session-row-title");
	gtk_label_set_xalign(GTK_LABEL(title), 0.0f);
	gtk_label_set_ellipsize(GTK_LABEL(title), PANGO_ELLIPSIZE_END);
	gtk_label_set_max_width_chars(GTK_LABEL(title), 30);
	gtk_box_append(GTK_BOX(text_box), title);

	GtkWidget *status_label = gtk_label_new(way_island_status_label(state));
	gtk_widget_add_css_class(status_label, "session-row-status");
	gtk_label_set_xalign(GTK_LABEL(status_label), 0.0f);
	gtk_box_append(GTK_BOX(text_box), status_label);

	gtk_box_append(GTK_BOX(row), text_box);

	return row;
}

static void way_island_rebuild_pill(const char *payload) {
	if (way_island_pill == NULL) return;

	way_island_clear_children(way_island_pill);

	if (payload == NULL || payload[0] == '\0') {
		GtkWidget *dot = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
		gtk_widget_add_css_class(dot, "empty-dot");
		gtk_box_append(GTK_BOX(way_island_pill), dot);
		return;
	}

	g_auto(GStrv) lines = g_strsplit(payload, "\n", -1);
	gint session_count = way_island_count_sessions(payload);

	g_autofree char *primary_name = NULL;
	g_autofree char *primary_state = NULL;

	if (lines[0] != NULL && lines[0][0] != '\0') {
		g_auto(GStrv) fields = g_strsplit(lines[0], "\t", 2);
		if (fields[0] != NULL && fields[1] != NULL) {
			primary_name = way_island_decode_field(fields[0]);
			primary_state = way_island_decode_field(fields[1]);
		}
	}

	if (primary_name == NULL) return;

	GtkWidget *status = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
	gtk_widget_add_css_class(status, "island-status");
	gtk_widget_add_css_class(status, way_island_status_class(primary_state));
	gtk_widget_set_valign(status, GTK_ALIGN_CENTER);
	gtk_box_append(GTK_BOX(way_island_pill), status);

	GtkWidget *title = gtk_label_new(primary_name);
	gtk_widget_add_css_class(title, "island-title");
	gtk_widget_set_valign(title, GTK_ALIGN_CENTER);
	gtk_label_set_ellipsize(GTK_LABEL(title), PANGO_ELLIPSIZE_END);
	gtk_label_set_max_width_chars(GTK_LABEL(title), 24);
	gtk_box_append(GTK_BOX(way_island_pill), title);

	if (session_count > 1) {
		g_autofree gchar *badge_text = g_strdup_printf("%d", session_count);
		GtkWidget *badge = gtk_label_new(badge_text);
		gtk_widget_add_css_class(badge, "island-badge");
		gtk_widget_set_valign(badge, GTK_ALIGN_CENTER);
		gtk_box_append(GTK_BOX(way_island_pill), badge);
	}
}

static void way_island_rebuild_detail(const char *payload) {
	if (way_island_detail == NULL) return;

	way_island_clear_children(way_island_detail);

	if (payload == NULL || payload[0] == '\0') return;

	g_auto(GStrv) lines = g_strsplit(payload, "\n", -1);
	for (gint i = 0; lines[i] != NULL; i++) {
		if (lines[i][0] == '\0') continue;

		g_auto(GStrv) fields = g_strsplit(lines[i], "\t", 2);
		if (fields[0] == NULL || fields[1] == NULL) continue;

		g_autofree char *name = way_island_decode_field(fields[0]);
		g_autofree char *state = way_island_decode_field(fields[1]);

		gtk_box_append(GTK_BOX(way_island_detail), way_island_build_session_row(name, state));
	}
}

static void way_island_rebuild_ui(const char *payload) {
	if (way_island_root == NULL) return;

	gboolean has_sessions = (payload != NULL && payload[0] != '\0');

	if (has_sessions) {
		gtk_widget_remove_css_class(way_island_root, "empty");
		gtk_widget_add_css_class(way_island_root, "island-pill");
	} else {
		gtk_widget_add_css_class(way_island_root, "empty");
		gtk_widget_remove_css_class(way_island_root, "island-pill");
		gtk_widget_remove_css_class(way_island_root, "expanded");
		way_island_expanded = FALSE;
	}

	way_island_rebuild_pill(payload);

	if (way_island_expanded && has_sessions) {
		way_island_rebuild_detail(payload);
		gtk_revealer_set_reveal_child(GTK_REVEALER(way_island_revealer), TRUE);
	} else {
		gtk_revealer_set_reveal_child(GTK_REVEALER(way_island_revealer), FALSE);
	}
}

static void on_hover_enter(GtkEventControllerMotion *controller, double x, double y, gpointer user_data) {
	(void)controller; (void)x; (void)y; (void)user_data;

	if (way_island_sessions_payload == NULL || way_island_sessions_payload[0] == '\0') return;
	if (way_island_expanded) return;

	way_island_expanded = TRUE;
	gtk_widget_add_css_class(way_island_root, "expanded");
	way_island_rebuild_detail(way_island_sessions_payload);
	gtk_revealer_set_reveal_child(GTK_REVEALER(way_island_revealer), TRUE);
}

static void on_hover_leave(GtkEventControllerMotion *controller, gpointer user_data) {
	(void)controller; (void)user_data;

	if (!way_island_expanded) return;

	way_island_expanded = FALSE;
	gtk_widget_remove_css_class(way_island_root, "expanded");
	gtk_revealer_set_reveal_child(GTK_REVEALER(way_island_revealer), FALSE);
}

static gboolean apply_sessions_payload_idle(gpointer user_data) {
	const gchar *payload = user_data;

	g_free(way_island_sessions_payload);
	way_island_sessions_payload = g_strdup(payload);
	way_island_rebuild_ui(way_island_sessions_payload);

	return G_SOURCE_REMOVE;
}

static gboolean quit_application_idle(gpointer user_data) {
	(void)user_data;

	way_island_should_quit = TRUE;
	if (way_island_app != NULL) {
		g_application_quit(G_APPLICATION(way_island_app));
	}

	return G_SOURCE_REMOVE;
}

static void on_activate(GtkApplication *app, gpointer user_data) {
	(void)user_data;
	GtkWindow *window = GTK_WINDOW(gtk_application_window_new(app));
	GtkCssProvider *provider = gtk_css_provider_new();
	GdkDisplay *display = gtk_widget_get_display(GTK_WIDGET(window));

	way_island_app = app;

	if (way_island_css_data != NULL) {
		gtk_css_provider_load_from_string(provider, way_island_css_data);
	}
	gtk_style_context_add_provider_for_display(
		display,
		GTK_STYLE_PROVIDER(provider),
		GTK_STYLE_PROVIDER_PRIORITY_APPLICATION
	);
	g_object_unref(provider);

	// root: vertical container (pill on top, detail below)
	way_island_root = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
	gtk_widget_add_css_class(way_island_root, "empty");

	// pill: collapsed summary bar
	way_island_pill = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
	gtk_widget_add_css_class(way_island_pill, "island-content");
	gtk_widget_set_hexpand(way_island_pill, TRUE);
	gtk_box_append(GTK_BOX(way_island_root), way_island_pill);

	// revealer: animates expand/collapse
	way_island_revealer = gtk_revealer_new();
	gtk_revealer_set_transition_type(GTK_REVEALER(way_island_revealer), GTK_REVEALER_TRANSITION_TYPE_SLIDE_DOWN);
	gtk_revealer_set_transition_duration(GTK_REVEALER(way_island_revealer), 200);
	gtk_revealer_set_reveal_child(GTK_REVEALER(way_island_revealer), FALSE);
	gtk_box_append(GTK_BOX(way_island_root), way_island_revealer);

	// detail: expanded session list (inside revealer)
	way_island_detail = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
	gtk_widget_add_css_class(way_island_detail, "island-detail");
	gtk_revealer_set_child(GTK_REVEALER(way_island_revealer), way_island_detail);

	// Hover events
	GtkEventController *motion = gtk_event_controller_motion_new();
	g_signal_connect(motion, "enter", G_CALLBACK(on_hover_enter), NULL);
	g_signal_connect(motion, "leave", G_CALLBACK(on_hover_leave), NULL);
	gtk_widget_add_controller(way_island_root, motion);

	way_island_rebuild_ui(way_island_sessions_payload);
	if (way_island_should_quit) {
		g_application_quit(G_APPLICATION(app));
	}

	gtk_window_set_title(window, "way-island");
	gtk_window_set_resizable(window, FALSE);
	gtk_window_set_decorated(window, FALSE);
	gtk_window_set_child(window, way_island_root);

	if (gtk_layer_is_supported()) {
		gtk_layer_init_for_window(window);
		gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_TOP);
		gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_TOP, TRUE);
		gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_LEFT, FALSE);
		gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_RIGHT, FALSE);
		gtk_layer_set_keyboard_mode(window, GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
		gtk_layer_set_exclusive_zone(window, 0);
		gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_TOP, 0);
	}

	gtk_window_present(window);
}

static void way_island_set_css(const char *css) {
	g_free(way_island_css_data);
	way_island_css_data = g_strdup(css);
}

static void schedule_sessions_payload_update(const char *payload) {
	g_idle_add_full(
		G_PRIORITY_DEFAULT,
		apply_sessions_payload_idle,
		g_strdup(payload != NULL ? payload : ""),
		g_free
	);
}

static void schedule_application_quit(void) {
	g_idle_add_full(
		G_PRIORITY_DEFAULT,
		quit_application_idle,
		NULL,
		NULL
	);
}

static int run_app(void) {
	GtkApplication *app = gtk_application_new("com.github.ka2n.way-island", G_APPLICATION_DEFAULT_FLAGS);
	int status;

	g_signal_connect(app, "activate", G_CALLBACK(on_activate), NULL);
	status = g_application_run(G_APPLICATION(app), 0, NULL);
	g_clear_pointer(&way_island_sessions_payload, g_free);
	g_clear_pointer(&way_island_css_data, g_free);
	way_island_detail = NULL;
	way_island_revealer = NULL;
	way_island_pill = NULL;
	way_island_root = NULL;
	way_island_app = NULL;
	way_island_expanded = FALSE;
	way_island_should_quit = FALSE;
	g_object_unref(app);

	return status;
}
*/
import "C"

import (
	"context"
	_ "embed"
	"log"
	"unsafe"

	"github.com/ka2n/way-island/internal/socket"
)

//go:embed style.css
var styleCSS string

func runUI(ctx context.Context, updates <-chan socket.SessionUpdate, store *overlayModel) int {
	cs := C.CString(styleCSS)
	C.way_island_set_css(cs)
	C.free(unsafe.Pointer(cs))

	go func() {
		for update := range updates {
			log.Printf("session update type=%s session_id=%s state=%s reason=%s", update.Type, update.Session.ID, update.Session.State, update.Reason)
			store.Apply(update)
			cs := C.CString(store.Payload())
			C.schedule_sessions_payload_update(cs)
			C.free(unsafe.Pointer(cs))
		}
	}()

	go func() {
		<-ctx.Done()
		C.schedule_application_quit()
	}()

	return int(C.run_app())
}
