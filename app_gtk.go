//go:build gtk4

package main

/*
#cgo pkg-config: gtk4 gtk4-layer-shell-0

#include <gtk/gtk.h>
#include <gtk4-layer-shell.h>
#include <glib.h>
#include <stdlib.h>
#include <string.h>

extern void way_island_focus_session(char *session_id);

static GtkApplication *way_island_app = NULL;
static GtkWidget *way_island_shell = NULL;
static GtkWidget *way_island_root = NULL;
static GtkWidget *way_island_pill = NULL;
static GtkWidget *way_island_revealer = NULL;
static GtkWidget *way_island_stack = NULL;
static GtkWidget *way_island_list_page = NULL;
static GtkWidget *way_island_detail_page = NULL;
static gchar *way_island_sessions_payload = NULL;
static gchar *way_island_selected_session_id = NULL;
static gchar *way_island_css_data = NULL;
static GtkCssProvider *way_island_css_provider = NULL;
static gboolean way_island_should_quit = FALSE;
static gint way_island_panel_view = 0;
static gint way_island_stack_view = 1;
static gint way_island_pending_panel_view = -1;
static guint way_island_panel_update_source_id = 0;
static guint way_island_hover_close_source_id = 0;
static const guint WAY_ISLAND_HOVER_CLOSE_DELAY_MS = 160;

static const char *way_island_session_group(const char *state) {
	if (g_strcmp0(state, "waiting") == 0) return "waiting";
	if (g_strcmp0(state, "working") == 0) return "working";
	return "other";
}

static GtkWidget *way_island_build_count_badge(const char *group, gint count) {
	GtkWidget *badge = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
	gtk_widget_add_css_class(badge, "island-count-badge");

	GtkWidget *square = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
	gtk_widget_add_css_class(square, "island-count-square");
	gtk_widget_add_css_class(square, group);
	gtk_box_append(GTK_BOX(badge), square);

	g_autofree gchar *count_text = g_strdup_printf("%d", count);
	GtkWidget *label = gtk_label_new(count_text);
	gtk_widget_add_css_class(label, "island-count-label");
	gtk_widget_set_valign(label, GTK_ALIGN_CENTER);
	gtk_box_append(GTK_BOX(badge), label);

	return badge;
}

GtkWidget *way_island_shell_new(void);
void way_island_shell_set_child(GtkWidget *shell, GtkWidget *child);

static void way_island_rebuild_ui(const char *payload);
static void way_island_schedule_panel_view(gint panel_view);
static GtkWidget *way_island_build_shell_widget(void);
static void way_island_set_stack_view(gint panel_view);
static void way_island_cancel_hover_close(void);
static void way_island_close_panel(void);

static gboolean way_island_close_panel_timeout(gpointer user_data) {
	(void)user_data;

	way_island_hover_close_source_id = 0;
	way_island_close_panel();
	return G_SOURCE_REMOVE;
}

static void way_island_cancel_hover_close(void) {
	if (way_island_hover_close_source_id == 0) return;

	g_source_remove(way_island_hover_close_source_id);
	way_island_hover_close_source_id = 0;
}

static void on_revealer_child_revealed_changed(GObject *object, GParamSpec *pspec, gpointer user_data) {
	(void)pspec;
	(void)user_data;

	GtkRevealer *revealer = GTK_REVEALER(object);
	if (gtk_revealer_get_reveal_child(revealer)) return;
	if (gtk_revealer_get_child_revealed(revealer)) return;

	gtk_widget_set_visible(GTK_WIDGET(revealer), FALSE);
	if (way_island_stack != NULL) {
		way_island_set_stack_view(1);
	}
	if (way_island_shell != NULL) {
		gtk_widget_queue_resize(way_island_shell);
	}
}

static void way_island_set_stack_view(gint panel_view) {
	if (way_island_stack == NULL) return;

	if (panel_view == 2) {
		if (way_island_stack_view != 2) {
			gtk_stack_set_transition_type(GTK_STACK(way_island_stack), GTK_STACK_TRANSITION_TYPE_SLIDE_LEFT);
		}
		gtk_stack_set_visible_child_name(GTK_STACK(way_island_stack), "detail");
		way_island_stack_view = 2;
		return;
	}

	if (way_island_stack_view != 1) {
		gtk_stack_set_transition_type(GTK_STACK(way_island_stack), GTK_STACK_TRANSITION_TYPE_SLIDE_RIGHT);
	}
	gtk_stack_set_visible_child_name(GTK_STACK(way_island_stack), "list");
	way_island_stack_view = 1;
}

static gboolean apply_panel_view_idle(gpointer user_data) {
	gint panel_view = GPOINTER_TO_INT(user_data);
	way_island_panel_update_source_id = 0;
	way_island_pending_panel_view = -1;

	if (way_island_panel_view == panel_view) return G_SOURCE_REMOVE;

	way_island_panel_view = panel_view;
	way_island_rebuild_ui(way_island_sessions_payload);

	return G_SOURCE_REMOVE;
}

static void way_island_schedule_panel_view(gint panel_view) {
	if (way_island_pending_panel_view == panel_view) return;

	way_island_pending_panel_view = panel_view;
	if (way_island_panel_update_source_id != 0) {
		g_source_remove(way_island_panel_update_source_id);
	}

	way_island_panel_update_source_id = g_idle_add_full(
		G_PRIORITY_DEFAULT,
		apply_panel_view_idle,
		GINT_TO_POINTER(panel_view),
		NULL
	);
}

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

static gchar *way_island_primary_session_id(const char *payload) {
	if (payload == NULL || payload[0] == '\0') return NULL;

	g_auto(GStrv) lines = g_strsplit(payload, "\n", -1);
	if (lines[0] == NULL || lines[0][0] == '\0') return NULL;

	g_auto(GStrv) fields = g_strsplit(lines[0], "\t", 4);
	if (fields[0] == NULL) return NULL;

	return way_island_decode_field(fields[0]);
}

static void way_island_open_detail(const char *session_id) {
	if (session_id == NULL || session_id[0] == '\0') return;

	way_island_cancel_hover_close();
	g_free(way_island_selected_session_id);
	way_island_selected_session_id = g_strdup(session_id);
	way_island_schedule_panel_view(2);
}

static void way_island_open_primary_detail(void) {
	g_autofree gchar *session_id = way_island_primary_session_id(way_island_sessions_payload);
	if (session_id == NULL) return;

	way_island_open_detail(session_id);
}

static void way_island_open_list(void) {
	if (way_island_sessions_payload == NULL || way_island_sessions_payload[0] == '\0') return;
	way_island_cancel_hover_close();
	way_island_schedule_panel_view(1);
}

static void way_island_close_panel(void) {
	way_island_cancel_hover_close();
	way_island_schedule_panel_view(0);
}

static void on_session_row_click(GtkGestureClick *gesture, gint n_press, gdouble x, gdouble y, gpointer user_data) {
	(void)n_press; (void)x; (void)y; (void)user_data;
	GtkWidget *widget = gtk_event_controller_get_widget(GTK_EVENT_CONTROLLER(gesture));
	char *session_id = g_object_get_data(G_OBJECT(widget), "way-island-session-id");
	if (session_id == NULL || session_id[0] == '\0') return;
	way_island_open_detail(session_id);
}

static GtkWidget *way_island_build_session_row(const char *session_id, const char *name, const char *state, const char *action, const char *last_user_message) {
	GtkWidget *row_shell = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
	gtk_widget_add_css_class(row_shell, "session-row-shell");

	GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
	gtk_widget_add_css_class(row, "session-row");
	gtk_box_append(GTK_BOX(row_shell), row);
	if (last_user_message != NULL && last_user_message[0] != '\0') {
		gtk_widget_set_tooltip_text(row_shell, last_user_message);
		gtk_widget_set_tooltip_text(row, last_user_message);
	}

	GtkGesture *click = gtk_gesture_click_new();
	g_object_set_data_full(G_OBJECT(row), "way-island-session-id", g_strdup(session_id), g_free);
	g_signal_connect(click, "released", G_CALLBACK(on_session_row_click), NULL);
	gtk_widget_add_controller(row, GTK_EVENT_CONTROLLER(click));

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

	const char *detail_text = (action != NULL && action[0] != '\0') ? action : way_island_status_label(state);
	GtkWidget *status_label = gtk_label_new(detail_text);
	gtk_widget_add_css_class(status_label, "session-row-status");
	gtk_label_set_xalign(GTK_LABEL(status_label), 0.0f);
	gtk_label_set_ellipsize(GTK_LABEL(status_label), PANGO_ELLIPSIZE_END);
	gtk_label_set_max_width_chars(GTK_LABEL(status_label), 48);
	gtk_box_append(GTK_BOX(text_box), status_label);

	gtk_box_append(GTK_BOX(row), text_box);

	GtkWidget *chevron = gtk_image_new_from_icon_name("go-next-symbolic");
	gtk_widget_add_css_class(chevron, "session-row-chevron");
	gtk_widget_set_valign(chevron, GTK_ALIGN_CENTER);
	gtk_box_append(GTK_BOX(row), chevron);

	return row_shell;
}

static void on_primary_click(GtkGestureClick *gesture, gint n_press, gdouble x, gdouble y, gpointer user_data) {
	(void)gesture; (void)n_press; (void)x; (void)y; (void)user_data;
	way_island_open_primary_detail();
}

static void on_back_click(GtkGestureClick *gesture, gint n_press, gdouble x, gdouble y, gpointer user_data) {
	(void)gesture; (void)n_press; (void)x; (void)y; (void)user_data;
	way_island_schedule_panel_view(1);
}

static void on_focus_button_click(GtkButton *button, gpointer user_data) {
	(void)button; (void)user_data;

	GtkWidget *widget = GTK_WIDGET(button);
	char *session_id = g_object_get_data(G_OBJECT(widget), "way-island-session-id");
	if (session_id == NULL || session_id[0] == '\0') return;
	way_island_close_panel();
	way_island_focus_session(session_id);
}

static void on_hover_enter(GtkEventControllerMotion *controller, double x, double y, gpointer user_data) {
	(void)controller; (void)x; (void)y; (void)user_data;

	way_island_cancel_hover_close();
	if (way_island_sessions_payload == NULL || way_island_sessions_payload[0] == '\0') return;
	if (way_island_panel_view != 0) return;

	way_island_open_list();
}

static void on_hover_leave(GtkEventControllerMotion *controller, gpointer user_data) {
	(void)controller; (void)user_data;
	if (way_island_panel_view == 0) return;

	way_island_cancel_hover_close();
	way_island_hover_close_source_id = g_timeout_add(
		WAY_ISLAND_HOVER_CLOSE_DELAY_MS,
		way_island_close_panel_timeout,
		NULL
	);
}

static void way_island_rebuild_pill(const char *payload) {
	if (way_island_pill == NULL) return;

	way_island_clear_children(way_island_pill);

	g_autofree char *primary_name = NULL;
	g_autofree char *primary_state = NULL;
	gint session_count = 0;
	gint waiting_count = 0;
	gint working_count = 0;
	gint other_count = 0;

	if (payload != NULL && payload[0] != '\0') {
		g_auto(GStrv) lines = g_strsplit(payload, "\n", -1);
		session_count = way_island_count_sessions(payload);

		for (gint i = 0; lines[i] != NULL; i++) {
			if (lines[i][0] == '\0') continue;

			g_auto(GStrv) fields = g_strsplit(lines[i], "\t", 5);
			if (fields[0] == NULL || fields[1] == NULL || fields[2] == NULL) continue;

			g_autofree char *state = way_island_decode_field(fields[2]);
			const char *group = way_island_session_group(state);
			if (g_strcmp0(group, "waiting") == 0) {
				waiting_count++;
			} else if (g_strcmp0(group, "working") == 0) {
				working_count++;
			} else {
				other_count++;
			}

			if (primary_name == NULL) {
				primary_name = way_island_decode_field(fields[1]);
				primary_state = way_island_decode_field(fields[2]);
			}
		}
	}

	if (primary_name == NULL) {
		primary_name = g_strdup("No sessions");
		primary_state = g_strdup("idle");
	}

	GtkWidget *status = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
	gtk_widget_add_css_class(status, "island-status");
	gtk_widget_add_css_class(status, way_island_status_class(primary_state));
	gtk_widget_set_valign(status, GTK_ALIGN_CENTER);

	GtkWidget *summary = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
	gtk_widget_add_css_class(summary, "island-summary");
	gtk_widget_set_hexpand(summary, TRUE);
	if (session_count > 0) {
		GtkGesture *click = gtk_gesture_click_new();
		g_signal_connect(click, "released", G_CALLBACK(on_primary_click), NULL);
		gtk_widget_add_controller(summary, GTK_EVENT_CONTROLLER(click));
	}

	GtkWidget *summary_content = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
	gtk_widget_add_css_class(summary_content, "island-summary-content");
	gtk_box_append(GTK_BOX(summary), summary_content);
	gtk_box_append(GTK_BOX(summary_content), status);

	GtkWidget *title = gtk_label_new(primary_name);
	gtk_widget_add_css_class(title, "island-title");
	gtk_widget_set_hexpand(title, TRUE);
	gtk_widget_set_halign(title, GTK_ALIGN_FILL);
	gtk_widget_set_valign(title, GTK_ALIGN_CENTER);
	gtk_label_set_ellipsize(GTK_LABEL(title), PANGO_ELLIPSIZE_END);
	gtk_label_set_xalign(GTK_LABEL(title), 0.0f);
	gtk_box_append(GTK_BOX(summary_content), title);
	gtk_box_append(GTK_BOX(way_island_pill), summary);

	if (session_count > 0) {
		GtkWidget *counts = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
		gtk_widget_add_css_class(counts, "island-counts");
		gtk_widget_set_halign(counts, GTK_ALIGN_END);
		gtk_widget_set_valign(counts, GTK_ALIGN_CENTER);
		gtk_box_append(GTK_BOX(counts), way_island_build_count_badge("waiting", waiting_count));
		gtk_box_append(GTK_BOX(counts), way_island_build_count_badge("working", working_count));
		gtk_box_append(GTK_BOX(counts), way_island_build_count_badge("other", other_count));
		gtk_box_append(GTK_BOX(way_island_pill), counts);
	}
}

static void way_island_rebuild_list(const char *payload) {
	if (way_island_list_page == NULL) return;

	way_island_clear_children(way_island_list_page);

	GtkWidget *header = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
	gtk_widget_add_css_class(header, "detail-header");

	GtkWidget *title = gtk_label_new("Sessions");
	gtk_widget_add_css_class(title, "detail-title");
	gtk_label_set_xalign(GTK_LABEL(title), 0.0f);
	gtk_box_append(GTK_BOX(header), title);
	gtk_box_append(GTK_BOX(way_island_list_page), header);

	g_auto(GStrv) lines = g_strsplit(payload, "\n", -1);
	for (gint i = 0; lines[i] != NULL; i++) {
		if (lines[i][0] == '\0') continue;

		g_auto(GStrv) fields = g_strsplit(lines[i], "\t", 5);
		if (fields[0] == NULL || fields[1] == NULL || fields[2] == NULL) continue;

		g_autofree char *session_id = way_island_decode_field(fields[0]);
		g_autofree char *name = way_island_decode_field(fields[1]);
		g_autofree char *state = way_island_decode_field(fields[2]);
		g_autofree char *action = fields[3] != NULL ? way_island_decode_field(fields[3]) : g_strdup("");
		g_autofree char *last_user_message = fields[4] != NULL ? way_island_decode_field(fields[4]) : g_strdup("");

		gtk_box_append(GTK_BOX(way_island_list_page), way_island_build_session_row(session_id, name, state, action, last_user_message));
	}
}

static gboolean way_island_rebuild_selected_detail(const char *payload) {
	if (way_island_detail_page == NULL || payload == NULL || payload[0] == '\0') return FALSE;

	const char *selected_id = way_island_selected_session_id;
	if (selected_id == NULL || selected_id[0] == '\0') return FALSE;

	way_island_clear_children(way_island_detail_page);

	g_auto(GStrv) lines = g_strsplit(payload, "\n", -1);
	for (gint i = 0; lines[i] != NULL; i++) {
		if (lines[i][0] == '\0') continue;

		g_auto(GStrv) fields = g_strsplit(lines[i], "\t", 5);
		if (fields[0] == NULL || fields[1] == NULL || fields[2] == NULL) continue;

		g_autofree char *session_id = way_island_decode_field(fields[0]);
		if (g_strcmp0(session_id, selected_id) != 0) continue;

		g_autofree char *name = way_island_decode_field(fields[1]);
		g_autofree char *state = way_island_decode_field(fields[2]);
		g_autofree char *action = fields[3] != NULL ? way_island_decode_field(fields[3]) : g_strdup("");
		g_autofree char *last_user_message = fields[4] != NULL ? way_island_decode_field(fields[4]) : g_strdup("");

		GtkWidget *header = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
		gtk_widget_add_css_class(header, "detail-header");

		GtkWidget *back = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
		gtk_widget_add_css_class(back, "detail-back-button");
		gtk_widget_set_tooltip_text(back, "Back");

		GtkWidget *back_content = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
		gtk_widget_add_css_class(back_content, "detail-back-button-content");
		gtk_box_append(GTK_BOX(back), back_content);

		GtkWidget *back_icon = gtk_image_new_from_icon_name("go-previous-symbolic");
		gtk_widget_add_css_class(back_icon, "session-row-chevron");
		gtk_box_append(GTK_BOX(back_content), back_icon);
		GtkGesture *back_click = gtk_gesture_click_new();
		g_signal_connect(back_click, "released", G_CALLBACK(on_back_click), NULL);
		gtk_widget_add_controller(back, GTK_EVENT_CONTROLLER(back_click));
		gtk_box_append(GTK_BOX(header), back);

		GtkWidget *header_title = gtk_label_new("Session detail");
		gtk_widget_add_css_class(header_title, "detail-title");
		gtk_label_set_xalign(GTK_LABEL(header_title), 0.0f);
		gtk_box_append(GTK_BOX(header), header_title);
		gtk_box_append(GTK_BOX(way_island_detail_page), header);

		GtkWidget *card = gtk_box_new(GTK_ORIENTATION_VERTICAL, 10);
		gtk_widget_add_css_class(card, "detail-card");
		gtk_box_append(GTK_BOX(way_island_detail_page), card);

		GtkWidget *card_content = gtk_box_new(GTK_ORIENTATION_VERTICAL, 10);
		gtk_widget_add_css_class(card_content, "detail-card-content");
		gtk_box_append(GTK_BOX(card), card_content);

		GtkWidget *detail_name = gtk_label_new(name);
		gtk_widget_add_css_class(detail_name, "detail-session-title");
		gtk_label_set_xalign(GTK_LABEL(detail_name), 0.0f);
		gtk_label_set_wrap(GTK_LABEL(detail_name), TRUE);
		gtk_box_append(GTK_BOX(card_content), detail_name);

		GtkWidget *state_row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
		gtk_box_append(GTK_BOX(card_content), state_row);

		GtkWidget *dot = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
		gtk_widget_add_css_class(dot, "island-status");
		gtk_widget_add_css_class(dot, way_island_status_class(state));
		gtk_widget_set_valign(dot, GTK_ALIGN_CENTER);
		gtk_box_append(GTK_BOX(state_row), dot);

		GtkWidget *state_label = gtk_label_new(way_island_status_label(state));
		gtk_widget_add_css_class(state_label, "detail-state-label");
		gtk_label_set_xalign(GTK_LABEL(state_label), 0.0f);
		gtk_box_append(GTK_BOX(state_row), state_label);

		g_autofree char *detail_text = NULL;
		if (action != NULL && action[0] != '\0' && last_user_message != NULL && last_user_message[0] != '\0') {
			detail_text = g_strdup_printf("%s\n\nLast user message: %s", action, last_user_message);
		} else if (action != NULL && action[0] != '\0') {
			detail_text = g_strdup(action);
		} else if (last_user_message != NULL && last_user_message[0] != '\0') {
			detail_text = g_strdup_printf("Last user message: %s", last_user_message);
		}
		if (detail_text != NULL && detail_text[0] != '\0') {
			GtkWidget *body = gtk_label_new(detail_text);
			gtk_widget_add_css_class(body, "detail-body");
			gtk_label_set_xalign(GTK_LABEL(body), 0.0f);
			gtk_label_set_wrap(GTK_LABEL(body), TRUE);
			gtk_box_append(GTK_BOX(card_content), body);
		}

		GtkWidget *focus_button = gtk_button_new_with_label("Open session");
		gtk_widget_add_css_class(focus_button, "detail-focus-button");
		g_object_set_data_full(G_OBJECT(focus_button), "way-island-session-id", g_strdup(session_id), g_free);
		g_signal_connect(focus_button, "clicked", G_CALLBACK(on_focus_button_click), NULL);
		gtk_box_append(GTK_BOX(card_content), focus_button);

		return TRUE;
	}

	return FALSE;
}

static void way_island_rebuild_detail(const char *payload) {
	if (way_island_stack == NULL) return;

	if (payload == NULL || payload[0] == '\0') return;

	way_island_rebuild_list(payload);

	if (way_island_panel_view == 2 && way_island_rebuild_selected_detail(payload)) {
		way_island_set_stack_view(2);
		return;
	}

	if (way_island_panel_view == 2) {
		way_island_open_primary_detail();
		if (way_island_rebuild_selected_detail(payload)) {
			way_island_set_stack_view(2);
			return;
		}
	}

	way_island_set_stack_view(1);
}

static void way_island_rebuild_ui(const char *payload) {
	if (way_island_root == NULL || way_island_shell == NULL) return;

	gboolean has_sessions = (payload != NULL && payload[0] != '\0');

	gtk_widget_add_css_class(way_island_shell, "island-pill");
	if (!has_sessions) {
		gtk_widget_remove_css_class(way_island_shell, "expanded");
		way_island_panel_view = 0;
		way_island_pending_panel_view = -1;
		if (way_island_panel_update_source_id != 0) {
			g_source_remove(way_island_panel_update_source_id);
			way_island_panel_update_source_id = 0;
		}
		way_island_cancel_hover_close();
		g_clear_pointer(&way_island_selected_session_id, g_free);
	}

	way_island_rebuild_pill(payload);

	if (way_island_panel_view != 0 && has_sessions) {
		gtk_widget_add_css_class(way_island_shell, "expanded");
		if (way_island_panel_view == 2) {
			gtk_widget_add_css_class(way_island_shell, "detail-view");
		} else {
			gtk_widget_remove_css_class(way_island_shell, "detail-view");
		}
		way_island_rebuild_detail(payload);
		gtk_widget_set_visible(way_island_revealer, TRUE);
		gtk_revealer_set_reveal_child(GTK_REVEALER(way_island_revealer), TRUE);
	} else {
		gtk_widget_remove_css_class(way_island_shell, "expanded");
		gtk_widget_remove_css_class(way_island_shell, "detail-view");
		if (way_island_stack != NULL) {
			way_island_set_stack_view(1);
		}
		gtk_revealer_set_reveal_child(GTK_REVEALER(way_island_revealer), FALSE);
		gtk_widget_queue_resize(way_island_shell);
	}
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

static gboolean apply_css_idle(gpointer user_data) {
	const gchar *css = user_data;

	g_free(way_island_css_data);
	way_island_css_data = g_strdup(css != NULL ? css : "");

	if (way_island_css_provider != NULL) {
		gtk_css_provider_load_from_string(way_island_css_provider, way_island_css_data);
	}

	return G_SOURCE_REMOVE;
}

static GtkWidget *way_island_build_shell_widget(void) {
	// shell: outer visual container and clipping boundary
	way_island_shell = way_island_shell_new();

	way_island_root = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
	way_island_shell_set_child(way_island_shell, way_island_root);

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
	gtk_widget_set_visible(way_island_revealer, FALSE);
	g_signal_connect(way_island_revealer, "notify::child-revealed", G_CALLBACK(on_revealer_child_revealed_changed), NULL);
	gtk_box_append(GTK_BOX(way_island_root), way_island_revealer);

	// detail: expanded session list (inside revealer)
	way_island_stack = gtk_stack_new();
	gtk_widget_add_css_class(way_island_stack, "island-detail");
	gtk_stack_set_transition_duration(GTK_STACK(way_island_stack), 180);
	gtk_stack_set_transition_type(GTK_STACK(way_island_stack), GTK_STACK_TRANSITION_TYPE_SLIDE_LEFT_RIGHT);
	gtk_stack_set_hhomogeneous(GTK_STACK(way_island_stack), FALSE);
	gtk_stack_set_vhomogeneous(GTK_STACK(way_island_stack), FALSE);
	gtk_stack_set_interpolate_size(GTK_STACK(way_island_stack), TRUE);
	gtk_revealer_set_child(GTK_REVEALER(way_island_revealer), way_island_stack);

	way_island_list_page = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
	gtk_widget_add_css_class(way_island_list_page, "detail-page");
	gtk_stack_add_named(GTK_STACK(way_island_stack), way_island_list_page, "list");

	way_island_detail_page = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
	gtk_widget_add_css_class(way_island_detail_page, "detail-page");
	gtk_stack_add_named(GTK_STACK(way_island_stack), way_island_detail_page, "detail");
	way_island_set_stack_view(1);

	GtkEventController *motion = gtk_event_controller_motion_new();
	g_signal_connect(motion, "enter", G_CALLBACK(on_hover_enter), NULL);
	g_signal_connect(motion, "leave", G_CALLBACK(on_hover_leave), NULL);
	gtk_widget_add_controller(way_island_shell, motion);

	way_island_rebuild_ui(way_island_sessions_payload);

	return way_island_shell;
}

static void on_activate(GtkApplication *app, gpointer user_data) {
	(void)user_data;
	GtkWindow *window = GTK_WINDOW(gtk_application_window_new(app));
	GdkDisplay *display = gtk_widget_get_display(GTK_WIDGET(window));

	way_island_app = app;
	way_island_css_provider = gtk_css_provider_new();

	if (way_island_css_data != NULL) {
		gtk_css_provider_load_from_string(way_island_css_provider, way_island_css_data);
	}
	gtk_style_context_add_provider_for_display(
		display,
		GTK_STYLE_PROVIDER(way_island_css_provider),
		GTK_STYLE_PROVIDER_PRIORITY_APPLICATION
	);

	GtkWidget *shell = way_island_build_shell_widget();
	if (way_island_should_quit) {
		g_application_quit(G_APPLICATION(app));
	}

	gtk_window_set_title(window, "way-island");
	gtk_window_set_resizable(window, FALSE);
	gtk_window_set_decorated(window, FALSE);
	gtk_widget_add_css_class(GTK_WIDGET(window), "way-island-window");
	gtk_widget_remove_css_class(GTK_WIDGET(window), "background");
	gtk_widget_remove_css_class(GTK_WIDGET(window), "solid-csd");
	gtk_window_set_child(window, shell);

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

static GtkWidget *way_island_test_build_widget(const char *payload, gint panel_view, const char *selected_session_id) {
	g_clear_pointer(&way_island_sessions_payload, g_free);
	way_island_sessions_payload = g_strdup(payload != NULL ? payload : "");

	g_clear_pointer(&way_island_selected_session_id, g_free);
	if (selected_session_id != NULL && selected_session_id[0] != '\0') {
		way_island_selected_session_id = g_strdup(selected_session_id);
	}

	way_island_panel_view = panel_view;
	way_island_stack_view = 1;
	way_island_pending_panel_view = -1;

	return way_island_build_shell_widget();
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

static void schedule_css_update(const char *css) {
	g_idle_add_full(
		G_PRIORITY_DEFAULT,
		apply_css_idle,
		g_strdup(css != NULL ? css : ""),
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
	g_clear_pointer(&way_island_selected_session_id, g_free);
	g_clear_pointer(&way_island_css_data, g_free);
	g_clear_object(&way_island_css_provider);
	if (way_island_panel_update_source_id != 0) {
		g_source_remove(way_island_panel_update_source_id);
		way_island_panel_update_source_id = 0;
	}
	if (way_island_hover_close_source_id != 0) {
		g_source_remove(way_island_hover_close_source_id);
		way_island_hover_close_source_id = 0;
	}
	way_island_shell = NULL;
	way_island_stack = NULL;
	way_island_list_page = NULL;
	way_island_detail_page = NULL;
	way_island_revealer = NULL;
	way_island_pill = NULL;
	way_island_root = NULL;
	way_island_app = NULL;
	way_island_panel_view = 0;
	way_island_stack_view = 1;
	way_island_pending_panel_view = -1;
	way_island_hover_close_source_id = 0;
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
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/fsnotify/fsnotify"
	"github.com/ka2n/way-island/internal/socket"
)

//go:embed style.css
var styleCSS string

var gtkSessionFocuser *sessionFocuser

func buildGTKWidgetForTest(payload string, panelView int, selectedSessionID string) unsafe.Pointer {
	cpayload := C.CString(payload)
	defer C.free(unsafe.Pointer(cpayload))
	cselected := C.CString(selectedSessionID)
	defer C.free(unsafe.Pointer(cselected))

	return unsafe.Pointer(C.way_island_test_build_widget(cpayload, C.gint(panelView), cselected))
}

//export way_island_focus_session
func way_island_focus_session(sessionID *C.char) {
	if gtkSessionFocuser == nil || sessionID == nil {
		return
	}
	id := C.GoString(sessionID)
	if id == "" {
		return
	}
	log.Printf("focus requested for session_id=%s", id)
	triggerSessionFocus(gtkSessionFocuser, id)
}

func runUI(ctx context.Context, updates <-chan socket.SessionUpdate, store *overlayModel) int {
	gtkSessionFocuser = newSessionFocuser(store)
	defer func() {
		gtkSessionFocuser = nil
	}()

	cssPaths, pathErr := resolveUserCSSPaths()
	if pathErr != nil {
		log.Printf("resolve user CSS paths: %v", pathErr)
	}

	appCSS := styleCSS
	if pathErr == nil {
		var err error
		appCSS, err = loadAppCSSFromPaths(styleCSS, cssPaths.StylePath, cssPaths.UserStylePath)
		if err != nil {
			log.Printf("load app CSS: %v", err)
		} else if css, err := loadUserStyleCSS(cssPaths.StylePath); err == nil && css != "" {
			log.Printf("load app CSS: using config style %s", cssPaths.StylePath)
		} else if css, err := loadUserStyleCSS(cssPaths.UserStylePath); err == nil && css != "" {
			log.Printf("load app CSS: merging user style %s", cssPaths.UserStylePath)
		} else {
			log.Printf("load app CSS: using builtin style.css")
		}
	}

	cs := C.CString(appCSS)
	C.way_island_set_css(cs)
	C.free(unsafe.Pointer(cs))

	go forwardUIUpdates(ctx, updates, store, func(payload string) {
		cs := C.CString(payload)
		C.schedule_sessions_payload_update(cs)
		C.free(unsafe.Pointer(cs))
	})

	go func() {
		<-ctx.Done()
		C.schedule_application_quit()
	}()

	if pathErr == nil {
		go watchCSSChanges(ctx, cssPaths, appCSS)
	}

	return int(C.run_app())
}

func watchCSSChanges(ctx context.Context, cssPaths userCSSPaths, initialCSS string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watch app CSS: create watcher: %v", err)
		return
	}
	defer watcher.Close()

	configDir := filepath.Dir(filepath.Dir(cssPaths.StylePath))
	watchPaths := cssWatchPaths(cssPaths)
	lastCSS := initialCSS
	var reloadTimer *time.Timer
	var reloadC <-chan time.Time
	var pendingReloadEvent string

	if err := addCSSWatchPaths(watcher, configDir, watchPaths); err != nil {
		log.Printf("watch app CSS: %v", err)
	}
	defer func() {
		if reloadTimer != nil {
			reloadTimer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reloadC:
			reloadC = nil

			currentCSS, err := loadAppCSSFromPaths(styleCSS, cssPaths.StylePath, cssPaths.UserStylePath)
			if err != nil {
				log.Printf("watch app CSS: %v", err)
				continue
			}
			if currentCSS == lastCSS {
				continue
			}
			lastCSS = currentCSS

			log.Printf("watch app CSS: reloaded (event=%s)", pendingReloadEvent)
			cs := C.CString(currentCSS)
			C.schedule_css_update(cs)
			C.free(unsafe.Pointer(cs))
		case err := <-watcher.Errors:
			if err != nil {
				log.Printf("watch app CSS: %v", err)
			}
		case event := <-watcher.Events:
			if !isRelevantCSSWatchEvent(event, watchPaths) {
				continue
			}

			watchPaths = cssWatchPaths(cssPaths)
			if err := ensureCSSWatchPaths(watcher, watchPaths); err != nil {
				log.Printf("watch app CSS: %v", err)
			}
			pendingReloadEvent = event.Op.String()
			resetReloadTimer(&reloadTimer, &reloadC, 75*time.Millisecond)
		}
	}
}

func resetReloadTimer(timer **time.Timer, timerC *<-chan time.Time, delay time.Duration) {
	if *timer == nil {
		*timer = time.NewTimer(delay)
		*timerC = (*timer).C
		return
	}

	if !(*timer).Stop() {
		select {
		case <-(*timer).C:
		default:
		}
	}
	(*timer).Reset(delay)
	*timerC = (*timer).C
}

func addCSSWatchPaths(watcher *fsnotify.Watcher, configDir string, watchPaths []string) error {
	if err := watcher.Add(configDir); err != nil {
		return err
	}
	return ensureCSSWatchPaths(watcher, watchPaths)
}

func ensureCSSWatchPaths(watcher *fsnotify.Watcher, watchPaths []string) error {
	for _, watchPath := range watchPaths {
		if err := ensureCSSDirWatch(watcher, filepath.Dir(watchPath)); err != nil {
			return err
		}
	}
	return nil
}

func ensureCSSDirWatch(watcher *fsnotify.Watcher, cssDir string) error {
	_, err := os.Stat(cssDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if isPathWatched(watcher, cssDir) {
		return nil
	}
	return watcher.Add(cssDir)
}

func isRelevantCSSWatchEvent(event fsnotify.Event, watchPaths []string) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}

	for _, watchPath := range watchPaths {
		if isRelevantSingleCSSWatchEvent(event, filepath.Dir(watchPath), filepath.Base(watchPath)) {
			return true
		}
	}

	return false
}

func isRelevantSingleCSSWatchEvent(event fsnotify.Event, cssDir, cssBase string) bool {
	if event.Name == cssPathJoin(cssDir, cssBase) {
		return true
	}

	if event.Name == cssDir {
		return true
	}

	if filepath.Dir(event.Name) == cssDir && filepath.Base(event.Name) == cssBase {
		return true
	}

	if filepath.Dir(event.Name) == filepath.Dir(cssDir) && filepath.Base(event.Name) == filepath.Base(cssDir) {
		return true
	}

	return false
}

func isPathWatched(watcher *fsnotify.Watcher, path string) bool {
	for _, watchedPath := range watcher.WatchList() {
		if watchedPath == path {
			return true
		}
	}
	return false
}

func cssPathJoin(dir, base string) string {
	return filepath.Join(dir, base)
}

func cssWatchPaths(paths userCSSPaths) []string {
	candidates := []string{
		paths.StylePath,
		resolveCSSWatchPath(paths.StylePath),
		paths.UserStylePath,
		resolveCSSWatchPath(paths.UserStylePath),
	}

	watchPaths := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		watchPaths = append(watchPaths, candidate)
	}

	return watchPaths
}

func resolveCSSWatchPath(cssPath string) string {
	resolvedPath, err := filepath.EvalSymlinks(cssPath)
	if err != nil {
		return cssPath
	}
	return resolvedPath
}
