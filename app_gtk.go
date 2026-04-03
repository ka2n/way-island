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
static gchar *way_island_sessions_payload = NULL;
static gboolean way_island_should_quit = FALSE;

static const char *WAY_ISLAND_CSS =
	"window {"
	"  background: transparent;"
	"}"
	".overlay-root {"
	"  transition: 120ms ease-out;"
	"}"
	".overlay-root.active {"
	"  background: rgba(12, 15, 22, 0.94);"
	"  border: 1px solid rgba(110, 125, 148, 0.38);"
	"  border-radius: 18px;"
	"  padding: 8px 12px;"
	"  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.42);"
	"}"
	".overlay-root.empty {"
	"  background: transparent;"
	"  border: none;"
	"  padding: 0;"
	"  box-shadow: none;"
	"}"
	".sessions-row {"
	"  border-spacing: 8px;"
	"}"
	".session-chip {"
	"  background: rgba(34, 41, 54, 0.92);"
	"  border: 1px solid rgba(125, 139, 161, 0.22);"
	"  border-radius: 999px;"
	"  padding: 6px 10px;"
	"}"
	".session-icon {"
	"  min-width: 10px;"
	"  min-height: 10px;"
	"  border-radius: 999px;"
	"}"
	".session-icon.working {"
	"  background: #4ade80;"
	"}"
	".session-icon.tool-running {"
	"  background: #60a5fa;"
	"}"
	".session-icon.waiting {"
	"  background: #fbbf24;"
	"}"
	".session-icon.idle {"
	"  background: #94a3b8;"
	"}"
	".session-label {"
	"  color: #f8fafc;"
	"  font-weight: 600;"
	"  font-size: 12px;"
	"}"
	".empty-dot {"
	"  background: rgba(148, 163, 184, 0.7);"
	"  min-width: 8px;"
	"  min-height: 8px;"
	"  border-radius: 999px;"
	"}";

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

static GtkWidget *way_island_build_empty_view(void) {
	GtkWidget *dot = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);

	gtk_widget_add_css_class(dot, "empty-dot");

	return dot;
}

static const char *way_island_status_class(const char *state) {
	if (g_strcmp0(state, "working") == 0) {
		return "working";
	}
	if (g_strcmp0(state, "tool_running") == 0) {
		return "tool-running";
	}
	if (g_strcmp0(state, "waiting") == 0) {
		return "waiting";
	}
	return "idle";
}

static GtkWidget *way_island_build_session_chip(const char *session_id, const char *state) {
	GtkWidget *chip = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
	GtkWidget *icon = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
	GtkWidget *label = gtk_label_new(NULL);
	g_autofree gchar *status_text = g_strdup(state);
	g_autofree gchar *label_text = NULL;

	status_text = g_strdelimit(status_text, "_", ' ');
	label_text = g_strdup_printf("%s %s", session_id, status_text);

	gtk_widget_add_css_class(chip, "session-chip");
	gtk_widget_add_css_class(icon, "session-icon");
	gtk_widget_add_css_class(icon, way_island_status_class(state));
	gtk_widget_add_css_class(label, "session-label");
	gtk_label_set_text(GTK_LABEL(label), label_text);
	gtk_label_set_xalign(GTK_LABEL(label), 0.0f);
	gtk_widget_set_valign(icon, GTK_ALIGN_CENTER);
	gtk_widget_set_valign(label, GTK_ALIGN_CENTER);
	gtk_box_append(GTK_BOX(chip), icon);
	gtk_box_append(GTK_BOX(chip), label);

	return chip;
}

static void way_island_rebuild_ui(const char *payload) {
	GtkWidget *content = NULL;

	if (way_island_root == NULL) {
		return;
	}

	way_island_clear_children(way_island_root);

	if (payload == NULL || payload[0] == '\0') {
		gtk_widget_remove_css_class(way_island_root, "active");
		gtk_widget_add_css_class(way_island_root, "empty");
		content = way_island_build_empty_view();
	} else {
		GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
		g_auto(GStrv) lines = g_strsplit(payload, "\n", -1);

		gtk_widget_remove_css_class(way_island_root, "empty");
		gtk_widget_add_css_class(way_island_root, "active");
		gtk_widget_add_css_class(row, "sessions-row");
		for (gint i = 0; lines[i] != NULL; i++) {
			g_auto(GStrv) fields = NULL;
			g_autofree char *session_id = NULL;
			g_autofree char *state = NULL;

			if (lines[i][0] == '\0') {
				continue;
			}

			fields = g_strsplit(lines[i], "\t", 2);
			if (fields[0] == NULL || fields[1] == NULL) {
				continue;
			}

			session_id = way_island_decode_field(fields[0]);
			state = way_island_decode_field(fields[1]);
			gtk_box_append(GTK_BOX(row), way_island_build_session_chip(session_id, state));
		}

		content = row;
	}

	gtk_box_append(GTK_BOX(way_island_root), content);
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
	GtkWindow *window = GTK_WINDOW(gtk_application_window_new(app));
	GtkWidget *root = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
	GtkCssProvider *provider = gtk_css_provider_new();
	GdkDisplay *display = gtk_widget_get_display(GTK_WIDGET(window));

	way_island_app = app;
	way_island_root = root;
	gtk_css_provider_load_from_string(provider, WAY_ISLAND_CSS);
	gtk_style_context_add_provider_for_display(
		display,
		GTK_STYLE_PROVIDER(provider),
		GTK_STYLE_PROVIDER_PRIORITY_APPLICATION
	);
	g_object_unref(provider);
	gtk_widget_add_css_class(root, "overlay-root");
	gtk_widget_add_css_class(root, "empty");
	way_island_rebuild_ui(way_island_sessions_payload);
	if (way_island_should_quit) {
		g_application_quit(G_APPLICATION(app));
	}

	gtk_window_set_title(window, "way-island");
	gtk_window_set_default_size(window, 1, 1);
	gtk_window_set_resizable(window, FALSE);
	gtk_window_set_decorated(window, FALSE);
	gtk_window_set_child(window, root);

	if (gtk_layer_is_supported()) {
		gtk_layer_init_for_window(window);
		gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_TOP);
		gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_TOP, TRUE);
		gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_LEFT, FALSE);
		gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_RIGHT, FALSE);
		gtk_layer_set_keyboard_mode(window, GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
		gtk_layer_set_exclusive_zone(window, 0);
		gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_TOP, 8);
	}

	gtk_window_present(window);
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
	way_island_root = NULL;
	way_island_app = NULL;
	way_island_should_quit = FALSE;
	g_object_unref(app);

	return status;
}
*/
import "C"

import (
	"context"
	"log"
	"unsafe"

	"github.com/ka2n/way-island/internal/socket"
)

func runUI(ctx context.Context, updates <-chan socket.SessionUpdate) int {
	model := newOverlayModel()

	go func() {
		for update := range updates {
			log.Printf("session update type=%s session_id=%s state=%s", update.Type, update.Session.ID, update.Session.State)
			model.Apply(update)
			cs := C.CString(model.Payload())
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
