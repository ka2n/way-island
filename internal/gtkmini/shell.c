//go:build gtk4

#include <gtk/gtk.h>
#include <stdarg.h>

typedef struct _WayIslandShell {
	GtkWidget parent_instance;
	GtkWidget *child;
} WayIslandShell;

typedef struct _WayIslandShellClass {
	GtkWidgetClass parent_class;
} WayIslandShellClass;

typedef struct _WayIslandClip {
	GtkWidget parent_instance;
	GtkWidget *child;
	int clip_height;
} WayIslandClip;

typedef struct _WayIslandClipClass {
	GtkWidgetClass parent_class;
} WayIslandClipClass;

typedef struct _WayIslandSlide {
	GtkWidget parent_instance;
	GtkWidget *list_child;
	GtkWidget *detail_child;
	double progress;
	gboolean showing_detail;
} WayIslandSlide;

typedef struct _WayIslandSlideClass {
	GtkWidgetClass parent_class;
} WayIslandSlideClass;

G_DEFINE_TYPE(WayIslandShell, way_island_shell, GTK_TYPE_WIDGET)
G_DEFINE_TYPE(WayIslandClip, way_island_clip, GTK_TYPE_WIDGET)
G_DEFINE_TYPE(WayIslandSlide, way_island_slide, GTK_TYPE_WIDGET)

static int way_island_measure_width_clamp(GtkWidget *child, int for_size) {
	int child_min_width = 0;

	if (child == NULL || !gtk_widget_should_layout(child)) {
		return for_size;
	}
	if (for_size < 0) {
		return for_size;
	}

	gtk_widget_measure(child, GTK_ORIENTATION_HORIZONTAL, -1, &child_min_width, NULL, NULL, NULL);
	if (for_size < child_min_width) {
		return child_min_width;
	}
	return for_size;
}

static gboolean way_island_geometry_debug_enabled(void) {
	static int loaded = 0;
	static gboolean enabled = FALSE;

	if (!loaded) {
		const char *value = g_getenv("WAY_ISLAND_DEBUG_GEOMETRY");
		enabled = value != NULL && *value == '1';
		loaded = 1;
	}
	return enabled;
}

static void way_island_geometry_log(const char *format, ...) {
	va_list args;
	char *message;

	if (!way_island_geometry_debug_enabled()) return;

	va_start(args, format);
	message = g_strdup_vprintf(format, args);
	va_end(args);

	g_printerr("geomdbg: %s\n", message);
	g_free(message);
}

static void way_island_slide_update_visibility(WayIslandSlide *self) {
	gboolean animating = self->progress > 0.0 && self->progress < 1.0;
	gboolean at_start = self->progress <= 0.0;
	gboolean at_end = self->progress >= 1.0;
	gboolean list_visible = FALSE;
	gboolean detail_visible = FALSE;

	if (animating) {
		list_visible = TRUE;
		detail_visible = TRUE;
	} else if (at_start) {
		if (self->showing_detail) {
			list_visible = TRUE;
			detail_visible = TRUE;
		} else {
			list_visible = TRUE;
			detail_visible = TRUE;
		}
	} else if (at_end) {
		list_visible = !self->showing_detail;
		detail_visible = self->showing_detail;
	}

	way_island_geometry_log(
		"slide.update_visibility animating=%d showing_detail=%d progress=%.3f list_visible=%d detail_visible=%d",
		animating,
		self->showing_detail,
		self->progress,
		self->list_child != NULL ? list_visible : -1,
		self->detail_child != NULL ? detail_visible : -1
	);

	if (self->list_child != NULL) {
		gtk_widget_set_visible(self->list_child, list_visible);
	}
	if (self->detail_child != NULL) {
		gtk_widget_set_visible(self->detail_child, detail_visible);
	}
}

static void way_island_shell_snapshot(GtkWidget *widget, GtkSnapshot *snapshot) {
	WayIslandShell *self = (WayIslandShell *)widget;
	const float radius = 16.0f;
	const float width = (float)gtk_widget_get_width(widget);
	const float height = (float)gtk_widget_get_height(widget);
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
	GtkStyleContext *context = gtk_widget_get_style_context(widget);
#pragma GCC diagnostic pop

	if (width <= 0.0f || height <= 0.0f) return;

	graphene_rect_t bounds = GRAPHENE_RECT_INIT(0.0f, 0.0f, width, height);
	graphene_size_t zero = GRAPHENE_SIZE_INIT(0.0f, 0.0f);
	graphene_size_t rounded = GRAPHENE_SIZE_INIT(radius, radius);
	GskRoundedRect outline;
	gsk_rounded_rect_init(&outline, &bounds, &zero, &zero, &rounded, &rounded);
	way_island_geometry_log("shell.snapshot width=%.1f height=%.1f child=%p", width, height, self->child);
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
	gtk_snapshot_render_background(snapshot, context, 0.0, 0.0, width, height);

	gtk_snapshot_push_rounded_clip(snapshot, &outline);
	if (self->child != NULL) {
		gtk_widget_snapshot_child(widget, self->child, snapshot);
	}
	gtk_snapshot_pop(snapshot);
	gtk_snapshot_render_frame(snapshot, context, 0.0, 0.0, width, height);
#pragma GCC diagnostic pop
}

static void way_island_shell_measure(GtkWidget *widget, GtkOrientation orientation, int for_size, int *minimum, int *natural, int *minimum_baseline, int *natural_baseline) {
	WayIslandShell *self = (WayIslandShell *)widget;
	int child_min = 0;
	int child_nat = 0;
	int request_width = -1;
	int request_height = -1;
	int child_for_size = for_size;

	if (self->child != NULL && gtk_widget_should_layout(self->child)) {
		if (orientation == GTK_ORIENTATION_VERTICAL) {
			child_for_size = way_island_measure_width_clamp(self->child, for_size);
		}
		gtk_widget_measure(self->child, orientation, child_for_size, &child_min, &child_nat, NULL, NULL);
	}

	gtk_widget_get_size_request(widget, &request_width, &request_height);
	if (orientation == GTK_ORIENTATION_HORIZONTAL && request_width >= 0) {
		*minimum = request_width;
		*natural = request_width;
	} else if (orientation == GTK_ORIENTATION_VERTICAL && request_height >= 0) {
		*minimum = request_height;
		*natural = request_height;
	} else {
		*minimum = child_min;
		*natural = child_nat;
	}
	way_island_geometry_log(
		"shell.measure orientation=%s for_size=%d request_width=%d request_height=%d child_min=%d child_nat=%d result_min=%d result_nat=%d",
		orientation == GTK_ORIENTATION_HORIZONTAL ? "h" : "v",
		for_size,
		request_width,
		request_height,
		child_min,
		child_nat,
		*minimum,
		*natural
	);

	if (minimum_baseline != NULL) *minimum_baseline = -1;
	if (natural_baseline != NULL) *natural_baseline = -1;
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
	widget_class->measure = way_island_shell_measure;
	widget_class->snapshot = way_island_shell_snapshot;
	gtk_widget_class_set_layout_manager_type(widget_class, GTK_TYPE_BIN_LAYOUT);
	gtk_widget_class_set_css_name(widget_class, "way-island-shell");
}

static void way_island_shell_init(WayIslandShell *self) {
	self->child = NULL;
}

static void way_island_clip_measure(GtkWidget *widget, GtkOrientation orientation, int for_size, int *minimum, int *natural, int *minimum_baseline, int *natural_baseline) {
	WayIslandClip *self = (WayIslandClip *)widget;
	int child_min = 0;
	int child_nat = 0;
	int request_width = -1;
	int request_height = -1;

	if (self->child != NULL && gtk_widget_should_layout(self->child)) {
		gtk_widget_measure(self->child, orientation, for_size, &child_min, &child_nat, NULL, NULL);
	}

	gtk_widget_get_size_request(widget, &request_width, &request_height);
	if (orientation == GTK_ORIENTATION_HORIZONTAL && request_width >= 0) {
		*minimum = request_width;
		*natural = request_width;
	} else if (orientation == GTK_ORIENTATION_VERTICAL && request_height >= 0) {
		*minimum = request_height;
		*natural = request_height;
	} else if (orientation == GTK_ORIENTATION_VERTICAL && self->clip_height >= 0) {
		*minimum = self->clip_height;
		*natural = self->clip_height;
	} else {
		*minimum = child_min;
		*natural = child_nat;
	}
	way_island_geometry_log(
		"clip.measure orientation=%s for_size=%d clip_height=%d child_min=%d child_nat=%d result_min=%d result_nat=%d",
		orientation == GTK_ORIENTATION_HORIZONTAL ? "h" : "v",
		for_size,
		self->clip_height,
		request_width,
		request_height,
		child_min,
		child_nat,
		*minimum,
		*natural
	);
	if (minimum_baseline != NULL) *minimum_baseline = -1;
	if (natural_baseline != NULL) *natural_baseline = -1;
}

static void way_island_clip_size_allocate(GtkWidget *widget, int width, int height, int baseline) {
	WayIslandClip *self = (WayIslandClip *)widget;
	(void)height;
	(void)baseline;

	if (self->child == NULL) return;

	GtkAllocation allocation = {
		.x = 0,
		.y = 0,
		.width = width,
		.height = 0,
	};
	int child_min_width = 0;
	gtk_widget_measure(self->child, GTK_ORIENTATION_VERTICAL, width, NULL, &allocation.height, NULL, NULL);
	gtk_widget_measure(self->child, GTK_ORIENTATION_HORIZONTAL, -1, &child_min_width, NULL, NULL, NULL);
	if (allocation.width < child_min_width) {
		allocation.width = child_min_width;
	}
	if (allocation.height < self->clip_height) {
		allocation.height = self->clip_height;
	}
	way_island_geometry_log(
		"clip.size_allocate width=%d height=%d clip_height=%d child_width=%d child_height=%d",
		width,
		height,
		self->clip_height,
		allocation.width,
		allocation.height
	);
	gtk_widget_size_allocate(self->child, &allocation, -1);
}

static void way_island_clip_snapshot(GtkWidget *widget, GtkSnapshot *snapshot) {
	WayIslandClip *self = (WayIslandClip *)widget;
	float width = (float)gtk_widget_get_width(widget);
	float height = (float)gtk_widget_get_height(widget);
	graphene_rect_t bounds;

	if (self->child == NULL || width <= 0.0f || height <= 0.0f) return;

	bounds = GRAPHENE_RECT_INIT(0.0f, 0.0f, width, height);
	way_island_geometry_log("clip.snapshot width=%.1f height=%.1f clip_height=%d", width, height, self->clip_height);
	gtk_snapshot_push_clip(snapshot, &bounds);
	gtk_widget_snapshot_child(widget, self->child, snapshot);
	gtk_snapshot_pop(snapshot);
}

static void way_island_clip_dispose(GObject *object) {
	WayIslandClip *self = (WayIslandClip *)object;

	if (self->child != NULL) {
		gtk_widget_unparent(self->child);
		self->child = NULL;
	}

	G_OBJECT_CLASS(way_island_clip_parent_class)->dispose(object);
}

static void way_island_clip_class_init(WayIslandClipClass *klass) {
	GObjectClass *object_class = G_OBJECT_CLASS(klass);
	GtkWidgetClass *widget_class = GTK_WIDGET_CLASS(klass);

	object_class->dispose = way_island_clip_dispose;
	widget_class->measure = way_island_clip_measure;
	widget_class->size_allocate = way_island_clip_size_allocate;
	widget_class->snapshot = way_island_clip_snapshot;
	gtk_widget_class_set_css_name(widget_class, "way-island-clip");
}

static void way_island_clip_init(WayIslandClip *self) {
	self->child = NULL;
	self->clip_height = -1;
}

static void way_island_slide_measure(GtkWidget *widget, GtkOrientation orientation, int for_size, int *minimum, int *natural, int *minimum_baseline, int *natural_baseline) {
	WayIslandSlide *self = (WayIslandSlide *)widget;
	int list_min = 0, list_nat = 0;
	int detail_min = 0, detail_nat = 0;
	double progress;
	int start_min = 0;
	int start_nat = 0;
	int end_min = 0;
	int end_nat = 0;
	int list_for_size = for_size;
	int detail_for_size = for_size;

	if (self->list_child != NULL && gtk_widget_should_layout(self->list_child)) {
		if (orientation == GTK_ORIENTATION_VERTICAL) {
			list_for_size = way_island_measure_width_clamp(self->list_child, for_size);
		}
		gtk_widget_measure(self->list_child, orientation, list_for_size, &list_min, &list_nat, NULL, NULL);
	}
	if (self->detail_child != NULL && gtk_widget_should_layout(self->detail_child)) {
		if (orientation == GTK_ORIENTATION_VERTICAL) {
			detail_for_size = way_island_measure_width_clamp(self->detail_child, for_size);
		}
		gtk_widget_measure(self->detail_child, orientation, detail_for_size, &detail_min, &detail_nat, NULL, NULL);
	}

	if (orientation == GTK_ORIENTATION_HORIZONTAL) {
		progress = self->progress;
		if (progress < 0.0) progress = 0.0;
		if (progress > 1.0) progress = 1.0;

		if (self->showing_detail) {
			start_min = list_min;
			start_nat = list_nat;
			end_min = detail_min;
			end_nat = detail_nat;
		} else {
			start_min = detail_min;
			start_nat = detail_nat;
			end_min = list_min;
			end_nat = list_nat;
		}

		*minimum = 0;
		*natural = (int)((double)start_nat + ((double)(end_nat-start_nat) * progress) + 0.5);
	} else {
		*minimum = MAX(list_min, detail_min);
		*natural = MAX(list_nat, detail_nat);
	}
	way_island_geometry_log(
		"slide.measure orientation=%s for_size=%d list_min=%d list_nat=%d detail_min=%d detail_nat=%d result_min=%d result_nat=%d progress=%.3f showing_detail=%d",
		orientation == GTK_ORIENTATION_HORIZONTAL ? "h" : "v",
		for_size,
		list_min,
		list_nat,
		detail_min,
		detail_nat,
		*minimum,
		*natural,
		self->progress,
		self->showing_detail
	);
	if (minimum_baseline != NULL) *minimum_baseline = -1;
	if (natural_baseline != NULL) *natural_baseline = -1;
}

static void way_island_slide_size_allocate(GtkWidget *widget, int width, int height, int baseline) {
	WayIslandSlide *self = (WayIslandSlide *)widget;
	GtkAllocation allocation = {.x = 0, .y = 0, .width = width, .height = height};
	int child_nat_height = 0;
	int child_min_width = 0;
	(void)baseline;

	if (self->list_child != NULL) {
		gtk_widget_measure(self->list_child, GTK_ORIENTATION_HORIZONTAL, -1, &child_min_width, NULL, NULL, NULL);
		allocation.width = MAX(width, child_min_width);
		gtk_widget_measure(self->list_child, GTK_ORIENTATION_VERTICAL, allocation.width, NULL, &child_nat_height, NULL, NULL);
		allocation.height = MAX(height, child_nat_height);
		gtk_widget_size_allocate(self->list_child, &allocation, -1);
	}
	if (self->detail_child != NULL) {
		gtk_widget_measure(self->detail_child, GTK_ORIENTATION_HORIZONTAL, -1, &child_min_width, NULL, NULL, NULL);
		allocation.width = MAX(width, child_min_width);
		gtk_widget_measure(self->detail_child, GTK_ORIENTATION_VERTICAL, allocation.width, NULL, &child_nat_height, NULL, NULL);
		allocation.height = MAX(height, child_nat_height);
		gtk_widget_size_allocate(self->detail_child, &allocation, -1);
	}
	way_island_geometry_log(
		"slide.size_allocate width=%d height=%d progress=%.3f showing_detail=%d",
		width,
		height,
		self->progress,
		self->showing_detail
	);
}

static void way_island_slide_snapshot(GtkWidget *widget, GtkSnapshot *snapshot) {
	WayIslandSlide *self = (WayIslandSlide *)widget;
	float width = (float)gtk_widget_get_width(widget);
	float height = (float)gtk_widget_get_height(widget);
	graphene_rect_t bounds;
	double progress;

	if (width <= 0.0f || height <= 0.0f) return;

	bounds = GRAPHENE_RECT_INIT(0.0f, 0.0f, width, height);
	way_island_geometry_log(
		"slide.snapshot width=%.1f height=%.1f progress=%.3f showing_detail=%d",
		width,
		height,
		self->progress,
		self->showing_detail
	);
	gtk_snapshot_push_clip(snapshot, &bounds);

	progress = self->progress;
	if (progress < 0.0) progress = 0.0;
	if (progress > 1.0) progress = 1.0;

	if (self->showing_detail) {
		if (self->list_child != NULL) {
			gtk_snapshot_save(snapshot);
			gtk_snapshot_translate(snapshot, &GRAPHENE_POINT_INIT((float)(-progress * width), 0.0f));
			gtk_widget_snapshot_child(widget, self->list_child, snapshot);
			gtk_snapshot_restore(snapshot);
		}
		if (self->detail_child != NULL) {
			gtk_snapshot_save(snapshot);
			gtk_snapshot_translate(snapshot, &GRAPHENE_POINT_INIT((float)((1.0 - progress) * width), 0.0f));
			gtk_widget_snapshot_child(widget, self->detail_child, snapshot);
			gtk_snapshot_restore(snapshot);
		}
	} else {
		if (self->detail_child != NULL) {
			gtk_snapshot_save(snapshot);
			gtk_snapshot_translate(snapshot, &GRAPHENE_POINT_INIT((float)(progress * width), 0.0f));
			gtk_widget_snapshot_child(widget, self->detail_child, snapshot);
			gtk_snapshot_restore(snapshot);
		}
		if (self->list_child != NULL) {
			gtk_snapshot_save(snapshot);
			gtk_snapshot_translate(snapshot, &GRAPHENE_POINT_INIT((float)((progress - 1.0) * width), 0.0f));
			gtk_widget_snapshot_child(widget, self->list_child, snapshot);
			gtk_snapshot_restore(snapshot);
		}
	}

	gtk_snapshot_pop(snapshot);
}

static void way_island_slide_dispose(GObject *object) {
	WayIslandSlide *self = (WayIslandSlide *)object;

	if (self->list_child != NULL) {
		gtk_widget_unparent(self->list_child);
		self->list_child = NULL;
	}
	if (self->detail_child != NULL) {
		gtk_widget_unparent(self->detail_child);
		self->detail_child = NULL;
	}

	G_OBJECT_CLASS(way_island_slide_parent_class)->dispose(object);
}

static void way_island_slide_class_init(WayIslandSlideClass *klass) {
	GObjectClass *object_class = G_OBJECT_CLASS(klass);
	GtkWidgetClass *widget_class = GTK_WIDGET_CLASS(klass);

	object_class->dispose = way_island_slide_dispose;
	widget_class->measure = way_island_slide_measure;
	widget_class->size_allocate = way_island_slide_size_allocate;
	widget_class->snapshot = way_island_slide_snapshot;
	gtk_widget_class_set_css_name(widget_class, "way-island-slide");
}

static void way_island_slide_init(WayIslandSlide *self) {
	self->list_child = NULL;
	self->detail_child = NULL;
	self->progress = 1.0;
	self->showing_detail = FALSE;
	way_island_slide_update_visibility(self);
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

GtkWidget *way_island_clip_new(void) {
	return g_object_new(way_island_clip_get_type(), NULL);
}

void way_island_clip_set_child(GtkWidget *clip, GtkWidget *child) {
	WayIslandClip *self = (WayIslandClip *)clip;
	if (self->child == child) return;

	if (self->child != NULL) {
		gtk_widget_unparent(self->child);
	}

	self->child = child;

	if (child != NULL) {
		gtk_widget_set_parent(child, clip);
	}
}

void way_island_clip_set_height(GtkWidget *clip, int clip_height) {
	WayIslandClip *self = (WayIslandClip *)clip;
	if (self->clip_height == clip_height) return;
	self->clip_height = clip_height;
	gtk_widget_queue_resize(clip);
	gtk_widget_queue_draw(clip);
}

GtkWidget *way_island_slide_new(void) {
	return g_object_new(way_island_slide_get_type(), NULL);
}

static void way_island_slide_set_child(GtkWidget *slide, GtkWidget **slot, GtkWidget *child) {
	WayIslandSlide *self = (WayIslandSlide *)slide;
	if (*slot == child) return;

	if (*slot != NULL) {
		gtk_widget_unparent(*slot);
	}
	*slot = child;
	if (child != NULL) {
		gtk_widget_set_parent(child, slide);
	}
	way_island_slide_update_visibility(self);
	gtk_widget_queue_resize(slide);
	gtk_widget_queue_draw(slide);
}

void way_island_slide_set_list_child(GtkWidget *slide, GtkWidget *child) {
	WayIslandSlide *self = (WayIslandSlide *)slide;
	way_island_slide_set_child(slide, &self->list_child, child);
}

void way_island_slide_set_detail_child(GtkWidget *slide, GtkWidget *child) {
	WayIslandSlide *self = (WayIslandSlide *)slide;
	way_island_slide_set_child(slide, &self->detail_child, child);
}

void way_island_slide_set_showing_detail(GtkWidget *slide, gboolean showing_detail) {
	WayIslandSlide *self = (WayIslandSlide *)slide;
	if (self->showing_detail == showing_detail) return;
	self->showing_detail = showing_detail;
	way_island_slide_update_visibility(self);
	gtk_widget_queue_draw(slide);
}

void way_island_slide_set_progress(GtkWidget *slide, double progress) {
	WayIslandSlide *self = (WayIslandSlide *)slide;
	if (progress < 0.0) progress = 0.0;
	if (progress > 1.0) progress = 1.0;
	if (self->progress == progress) return;
	self->progress = progress;
	way_island_slide_update_visibility(self);
	gtk_widget_queue_draw(slide);
}
