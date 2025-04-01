#include <GL/gl.h>
#include <GL/glx.h>
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Global flag to track if the X11 display connection is lost
int display_gone = 0;

// X11 IO error handler — if the display goes away (e.g., X server dies),
// this gets called to avoid a crash. We set a flag instead.
int handle_io_error(Display *dpy) {
    display_gone = 1;
    return 0; // 0 means "continue execution", but we've noted the error
}

// Sets the above handler as the default for X11 IO errors
void set_io_error_handler() { XSetIOErrorHandler(handle_io_error); }

// Utility to query if the display is marked dead
int is_display_dead() { return display_gone; }

// Simple wrapper for XInternAtom to get a named atom (interned string handle)
Atom get_atom(Display *dpy, const char *name) { return XInternAtom(dpy, name, False); }

// Recursively searches for a child window that matches either the full display size
// or a given size, for a maximum of 10 iterations. Often used to find desktop container windows.
Window find_subwindow(Display *dpy, Window win, int screen, int width, int height) {
    Window       root, parent, *children;
    unsigned int nchildren;

    for (int i = 0; i < 10; i++) {
        if (!XQueryTree(dpy, win, &root, &parent, &children, &nchildren)) {
            return win; // Fallback if query fails
        }
        for (unsigned int j = 0; j < nchildren; j++) {
            XWindowAttributes attrs;
            if (XGetWindowAttributes(dpy, children[j], &attrs) != 0 && attrs.map_state != 0) {
                if ((attrs.width == DisplayWidth(dpy, screen) && attrs.height == DisplayHeight(dpy, screen)) ||
                    (attrs.width == width && attrs.height == height)) {
                    win = children[j];
                    break;
                }
            }
        }
        XFree(children);
    }
    return win;
}

// Tries to find the actual desktop window (some WMs reparent windows)
// It checks for the __SWM_VROOT atom, which older WMs use to tag the virtual root.
Window find_desktop_window(Display *dpy, int screen, Window *out_root) {
    Atom   swm_vroot = get_atom(dpy, "__SWM_VROOT");
    Window root      = RootWindow(dpy, screen);
    Window desktop   = root;

    Window       root_ret, parent_ret, *children;
    unsigned int nchildren;
    if (XQueryTree(dpy, root, &root_ret, &parent_ret, &children, &nchildren)) {
        for (unsigned int i = 0; i < nchildren; i++) {
            Atom           actual_type;
            int            actual_format;
            unsigned long  nitems, bytes_after;
            unsigned char *prop = NULL;

            // Check if this child window claims to be the VROOT
            if (XGetWindowProperty(
                    dpy, children[i], swm_vroot, 0, 1, False, XA_WINDOW, &actual_type, &actual_format, &nitems,
                    &bytes_after, &prop) == Success &&
                actual_type == XA_WINDOW && prop) {
                desktop = *((Window *)prop);
                XFree(prop);
                break;
            }
            if (prop)
                XFree(prop);
        }
        XFree(children);
    }

    // Narrow down to a specific subwindow if applicable
    desktop   = find_subwindow(dpy, desktop, screen, -1, -1);
    *out_root = root;
    return desktop;
}

// Wrapper to open a connection to the X11 server
Display *open_display() { return XOpenDisplay(NULL); }

// Get display width in pixels for a given screen
int get_display_width(Display *dpy, int screen) { return DisplayWidth(dpy, screen); }

// Get display height in pixels for a given screen
int get_display_height(Display *dpy, int screen) { return DisplayHeight(dpy, screen); }

// Creates a window that is override-redirect (ignored by window manager),
// fully opaque black background, and placed at the lowest Z-order layer.
// Also marks it as _NET_WM_WINDOW_TYPE_DESKTOP to hint it's the background.
Window create_backed_window(Display *dpy, int screen, int x, int y, int width, int height) {
    Window root;
    Window desktop = find_desktop_window(dpy, screen, &root);

    XSetWindowAttributes attrs;
    attrs.override_redirect = True;   // Don’t let the WM manage this window
    attrs.backing_store     = Always; // Keep contents between exposures
    attrs.background_pixel  = BlackPixel(dpy, screen);
    attrs.event_mask        = StructureNotifyMask | ExposureMask; // We care about basic events

    unsigned long flags = CWOverrideRedirect | CWBackingStore | CWBackPixel | CWEventMask;
    Window        win =
        XCreateWindow(dpy, desktop, x, y, width, height, 0, CopyFromParent, InputOutput, CopyFromParent, flags, &attrs);

    // Tell window manager this is a desktop-style window
    Atom wm_type         = get_atom(dpy, "_NET_WM_WINDOW_TYPE");
    Atom wm_type_desktop = get_atom(dpy, "_NET_WM_WINDOW_TYPE_DESKTOP");
    XChangeProperty(dpy, win, wm_type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&wm_type_desktop, 1);

    XLowerWindow(dpy, win); // Send to back
    XMapWindow(dpy, win);   // Make visible
    XFlush(dpy);            // Ensure commands are sent to X server
    return win;
}

// Create a Pixmap from raw 32-bit image data and set it as the root background
// image should be in XImage-compatible format (32-bit RGBA or BGRA)
void set_root_pixmap(Display *dpy, int screen, const unsigned char *data, int width, int height) {
    Window root   = RootWindow(dpy, screen);
    Pixmap pixmap = XCreatePixmap(dpy, root, width, height, DefaultDepth(dpy, screen));

    GC      gc  = XCreateGC(dpy, pixmap, 0, NULL);
    XImage *img = XCreateImage(
        dpy, DefaultVisual(dpy, screen), DefaultDepth(dpy, screen), ZPixmap, 0, (char *)malloc(width * height * 4),
        width, height, 32, 0);

    memcpy(img->data, data, width * height * 4);
    XPutImage(dpy, pixmap, gc, img, 0, 0, 0, 0, width, height);

    XSetWindowBackgroundPixmap(dpy, root, pixmap);
    XClearWindow(dpy, root);

    Atom prop_root = get_atom(dpy, "_XROOTPMAP_ID");
    Atom prop_eset = get_atom(dpy, "ESETROOT_PMAP_ID");

    XChangeProperty(dpy, root, prop_root, XA_PIXMAP, 32, PropModeReplace, (unsigned char *)&pixmap, 1);
    XChangeProperty(dpy, root, prop_eset, XA_PIXMAP, 32, PropModeReplace, (unsigned char *)&pixmap, 1);

    XFreeGC(dpy, gc);
    img->data = NULL; // Prevent XDestroyImage from freeing original data
    XDestroyImage(img);
    XFlush(dpy);
}
