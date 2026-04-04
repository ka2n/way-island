//go:build gtk4

#include <gtk/gtk.h>

typedef struct _WayIslandShell {
	GtkWidget parent_instance;
	GtkWidget *child;
} WayIslandShell;

typedef struct _WayIslandShellClass {
	GtkWidgetClass parent_class;
} WayIslandShellClass;

G_DEFINE_TYPE(WayIslandShell, way_island_shell, GTK_TYPE_WIDGET)

static void way_island_shell_snapshot(GtkWidget *widget, GtkSnapshot *snapshot) {
	WayIslandShell *self = (WayIslandShell *)widget;
	const float radius = 16.0f;
	const float width = (float)gtk_widget_get_width(widget);
	const float height = (float)gtk_widget_get_height(widget);
	GtkStyleContext *context = gtk_widget_get_style_context(widget);

	if (width <= 0.0f || height <= 0.0f) return;

	graphene_rect_t bounds = GRAPHENE_RECT_INIT(0.0f, 0.0f, width, height);
	graphene_size_t zero = GRAPHENE_SIZE_INIT(0.0f, 0.0f);
	graphene_size_t rounded = GRAPHENE_SIZE_INIT(radius, radius);
	GskRoundedRect outline;
	gsk_rounded_rect_init(&outline, &bounds, &zero, &zero, &rounded, &rounded);
	gtk_snapshot_render_background(snapshot, context, 0.0, 0.0, width, height);

	gtk_snapshot_push_rounded_clip(snapshot, &outline);
	if (self->child != NULL) {
		gtk_widget_snapshot_child(widget, self->child, snapshot);
	}
	gtk_snapshot_pop(snapshot);
	gtk_snapshot_render_frame(snapshot, context, 0.0, 0.0, width, height);
}

static void way_island_shell_dispose(GObject *object) {
	WayIslandShell *self = (WayIslandShell *)object;

	if (self->child != NULL) {
		gtk_widget_unparent(self->child);
		self->child = NULL;
	}

	G_OBJECT_CLASS(way_island_shell_parent_class)->dispose(object);
}

static void way_island_shell_class_init(WayIslandShellClass *klass) {
	GObjectClass *object_class = G_OBJECT_CLASS(klass);
	GtkWidgetClass *widget_class = GTK_WIDGET_CLASS(klass);

	object_class->dispose = way_island_shell_dispose;
	widget_class->snapshot = way_island_shell_snapshot;
	gtk_widget_class_set_layout_manager_type(widget_class, GTK_TYPE_BIN_LAYOUT);
	gtk_widget_class_set_css_name(widget_class, "way-island-shell");
}

static void way_island_shell_init(WayIslandShell *self) {
	self->child = NULL;
}

GtkWidget *way_island_shell_new(void) {
	return g_object_new(way_island_shell_get_type(), NULL);
}

void way_island_shell_set_child(GtkWidget *shell, GtkWidget *child) {
	WayIslandShell *self = (WayIslandShell *)shell;
	if (self->child == child) return;

	if (self->child != NULL) {
		gtk_widget_unparent(self->child);
	}

	self->child = child;

	if (child != NULL) {
		gtk_widget_set_parent(child, shell);
	}
}
