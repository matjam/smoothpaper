#ifndef SMOOTHPAPER_HELPERS_H
#define SMOOTHPAPER_HELPERS_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <stdio.h>

int xErrorHandler(Display *dpy, XErrorEvent *err) {
  char buffer[1024];
  XGetErrorText(dpy, err->error_code, buffer, sizeof(buffer));
  printf("X Error: %s (request %d, minor %d)\n", buffer, err->request_code,
         err->minor_code);
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
#endif // SMOOTHPAPER_HELPERS_H
