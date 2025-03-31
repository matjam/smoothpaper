#ifndef SMOOTHPAPER_HELPERS_H
#define SMOOTHPAPER_HELPERS_H

#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <stdio.h>

int xErrorHandler(Display *dpy, XErrorEvent *err) {
    char buffer[1024];
    XGetErrorText(dpy, err->error_code, buffer, sizeof(buffer));
    printf("X Error: %s (request %d, minor %d)\n", buffer, err->request_code, err->minor_code);
    return 0;
}

void setXErrorHandler() {
    printf("Setting X error handler\n");
    XSetErrorHandler(xErrorHandler);
}

static inline int getEventType(XEvent *e) { return e->xany.type; }

static inline void destroyXImage(XImage *image) {
    if (image) {
        XDestroyImage(image);
    }
}

int screen_width(Display *dpy) {
    Screen *screen = DefaultScreenOfDisplay(dpy);
    return screen->width;
}

int screen_height(Display *dpy) {
    Screen *screen = DefaultScreenOfDisplay(dpy);
    return screen->height;
}

typedef struct {
    Display *display;
    Window   root;
    Window   desktop;
    Window   window;
    int      width;
    int      height;
} XDesktopWindow;

XDesktopWindow create_desktop_window() {
    XDesktopWindow out = {0};

    Display *display = XOpenDisplay(NULL);
    if (!display) {
        fprintf(stderr, "Failed to open X display\n");
        return out;
    }

    int    screen = DefaultScreen(display);
    Window root   = RootWindow(display, screen);

    int width  = DisplayWidth(display, screen);
    int height = DisplayHeight(display, screen);

    // Create an override-redirect window
    XSetWindowAttributes attrs = {
        .override_redirect = True,
        .background_pixel  = BlackPixel(display, screen),
        .event_mask        = ExposureMask | StructureNotifyMask};

    Window win = XCreateWindow(
        display, root, 0, 0, width, height, 0, CopyFromParent, InputOutput, CopyFromParent,
        CWOverrideRedirect | CWBackPixel | CWEventMask, &attrs);

    // Set _NET_WM_WINDOW_TYPE = _NET_WM_WINDOW_TYPE_DESKTOP
    Atom xa_type    = XInternAtom(display, "_NET_WM_WINDOW_TYPE", False);
    Atom xa_desktop = XInternAtom(display, "_NET_WM_WINDOW_TYPE_DESKTOP", False);
    XChangeProperty(display, win, xa_type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&xa_desktop, 1);

    // Lower the window so it stays below everything
    XLowerWindow(display, win);
    XMapWindow(display, win);
    XFlush(display);

    out.display = display;
    out.root    = root;
    out.desktop = win;
    out.window  = win;
    out.width   = width;
    out.height  = height;

    return out;
}

#endif // SMOOTHPAPER_HELPERS_H
