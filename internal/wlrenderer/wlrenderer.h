#include <stdlib.h>
#include <string.h>

#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GLES2/gl2.h>
#include <wayland-client.h>
#include <wayland-egl.h>

#include "wlr-layer-shell-unstable-v1.h"
#include "xdg-shell-protocol.h"

// Forward-declare the Go handler
extern void
goHandleGlobal(uintptr_t handle, struct wl_registry *registry, uint32_t name, char *interface, uint32_t version);

// Shim to bridge to the Go handler
static void
shimHandleGlobal(void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
    goHandleGlobal((uintptr_t)data, registry, name, (char *)interface, version);
}

// After the goHandleGlobal forward declaration, add:
extern void goHandleGlobalRemove(uintptr_t handle, struct wl_registry *registry, uint32_t name);

// Add this C shim function:
static void shimHandleGlobalRemove(void *data, struct wl_registry *registry, uint32_t name) {
    goHandleGlobalRemove((uintptr_t)data, registry, name);
}

static const struct wl_registry_listener registry_listener = {
    .global        = shimHandleGlobal,
    .global_remove = shimHandleGlobalRemove,
};

static inline const struct wl_registry_listener *get_registry_listener() { return &registry_listener; }

static inline struct wl_display *connect_wayland_display() { return wl_display_connect(NULL); }

// Forward declaration for the Go handler
extern void goHandleLayerSurfaceConfigure(
    uintptr_t handle, struct zwlr_layer_surface_v1 *surface, uint32_t serial, uint32_t width, uint32_t height);

extern void goHandleLayerSurfaceClosed(uintptr_t handle, struct zwlr_layer_surface_v1 *surface);

// C shim to bridge to Go
static void shimHandleLayerSurfaceConfigure(
    void *data, struct zwlr_layer_surface_v1 *surface, uint32_t serial, uint32_t width, uint32_t height) {
    goHandleLayerSurfaceConfigure((uintptr_t)data, surface, serial, width, height);
}

static void shimHandleLayerSurfaceClosed(void *data, struct zwlr_layer_surface_v1 *surface) {
    goHandleLayerSurfaceClosed((uintptr_t)data, surface);
}

static const struct zwlr_layer_surface_v1_listener layer_surface_listener = {
    .configure = shimHandleLayerSurfaceConfigure, .closed = shimHandleLayerSurfaceClosed};

static inline const struct zwlr_layer_surface_v1_listener *get_layer_surface_listener() {
    return &layer_surface_listener;
}

// Core protocol interface needed for binding wl_output from Go
extern const struct wl_interface wl_output_interface;

// ===== wl_output listener for scale/geometry =====
extern void goHandleOutputScale(uintptr_t handle, struct wl_output *output, int32_t factor);

static void shimHandleOutputGeometry(void *data, struct wl_output *output, int32_t x, int32_t y,
                                    int32_t phys_width, int32_t phys_height, int32_t subpixel,
                                    const char *make, const char *model, int32_t transform) {
    (void)data; (void)output; (void)x; (void)y; (void)phys_width; (void)phys_height;
    (void)subpixel; (void)make; (void)model; (void)transform;
}

static void shimHandleOutputMode(void *data, struct wl_output *output, uint32_t flags,
                                 int32_t width, int32_t height, int32_t refresh) {
    (void)data; (void)output; (void)flags; (void)width; (void)height; (void)refresh;
}

static void shimHandleOutputDone(void *data, struct wl_output *output) {
    (void)data; (void)output;
}

static void shimHandleOutputScale(void *data, struct wl_output *output, int32_t factor) {
    goHandleOutputScale((uintptr_t)data, output, factor);
}

static const struct wl_output_listener output_listener = {
    .geometry = shimHandleOutputGeometry,
    .mode     = shimHandleOutputMode,
    .done     = shimHandleOutputDone,
    .scale    = shimHandleOutputScale,
};

static inline const struct wl_output_listener *get_output_listener() { return &output_listener; }

