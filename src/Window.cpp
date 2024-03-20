/*
 * Copyright © 2005 Novell, Inc.
 *
 * Permission to use, copy, modify, distribute, and sell this software
 * and its documentation for any purpose is hereby granted without
 * fee, provided that the above copyright notice appear in all copies
 * and that both that copyright notice and this permission notice
 * appear in supporting documentation, and that the name of
 * Novell, Inc. not be used in advertising or publicity pertaining to
 * distribution of the software without specific, written prior permission.
 * Novell, Inc. makes no representations about the suitability of this
 * software for any purpose. It is provided "as is" without express or
 * implied warranty.
 *
 * NOVELL, INC. DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE,
 * INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS, IN
 * NO EVENT SHALL NOVELL, INC. BE LIABLE FOR ANY SPECIAL, INDIRECT OR
 * CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS
 * OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT,
 * NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION
 * WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 *
 * Author: David Reveman <davidr@novell.com>
 */

/*
 * Modified by: Shantanu Goel
 * Tech Blog: http://tech.shantanugoel.com
 * Blog: http://blog.shantanugoel.com
 * Home Page: http://tech.shantanugoel.com/projects/linux/shantz-xwinwrap
 *
 * Changelog:
 * 15-Jan-09:   1. Fixed the bug where XFetchName returning a NULL for "name"
 *                 resulted in a crash.
 *              2. Provided an option to specify the desktop window name.
 *              3. Added debug messages
 *
 * 24-Aug-08:   1. Fixed the geometry option (-g) so that it works
 *              2. Added override option (-ov), for seamless integration with
 *                 desktop like a background in non-fullscreen modes
 *              3. Added shape option (-sh), to create non-rectangular windows.
 *                 Currently supporting circlular and triangular windows
 */

/*
 * Picked up by Aaahh, https://github.com/Aaahh/xwinwrap
 */

/*
 * Shamelessly stolen xwinwrap.c by David Reveman and modified to work with
 * SFML by Nathan Ollerenshaw. Parts of this code are from xwinwrap.c, and
 * are therefore (c) Novell, Inc.
 */

#include <SFML/Graphics/RenderWindow.hpp>
#include <X11/X.h>
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <cstdint>
#include <spdlog/common.h>
#include <spdlog/spdlog.h>

#define ATOM(a) XInternAtom(display, #a, False)

// Generally not a fan of global symbols like this, but there's only one display and screen.
Display *display = nullptr;
int      display_width;
int      display_height;
int      screen;

struct window {
    Window   root, window, desktop;
    Drawable drawable;
    Visual  *visual;
    Colormap colourmap;

    unsigned int width;
    unsigned int height;
    int          x;
    int          y;
} window;

static void init_x11() {
    display = XOpenDisplay(nullptr);
    if (display == nullptr) {
        spdlog::error("Error: couldn't open display");
        return;
    }
    screen         = DefaultScreen(display);
    display_width  = DisplayWidth(display, screen);
    display_height = DisplayHeight(display, screen);
}

static Window find_subwindow(Window win, int w, int h) {
    unsigned int i = 0, j = 0;
    Window       troot = {}, parent = {}, *children = nullptr;
    unsigned int n = 0;

    /* search subwindows with same size as display or work area */

    for (i = 0; i < 10; i++) {
        XQueryTree(display, win, &troot, &parent, &children, &n);

        for (j = 0; j < n; j++) {
            XWindowAttributes attrs;

            if (XGetWindowAttributes(display, children[j], &attrs) != 0) {
                /* Window must be mapped and same size as display or
                 * work space */
                if (attrs.map_state != 0 && ((attrs.width == display_width && attrs.height == display_height) ||
                                             (attrs.width == w && attrs.height == h))) {
                    win = children[j];
                    break;
                }
            }
        }

        XFree(children);
        if (j == n) {
            break;
        }
    }

    return win;
}

static Window find_desktop_window(Window *p_root, Window *p_desktop) {
    Atom           type   = 0;
    int            format = 0, i = 0;
    uint64_t       nitems = 0, bytes = 0;
    unsigned int   n     = 0;
    Window         root  = RootWindow(display, screen);
    Window         win   = 0;
    Window         troot = {}, parent = {}, *children = nullptr;
    unsigned char *buf = nullptr;

    if (p_root == nullptr || p_desktop == nullptr) {
        return 0;
    }

    /* some window managers set __SWM_VROOT to some child of root window */

    XQueryTree(display, root, &troot, &parent, &children, &n);
    for (i = 0; i < (int)n; i++) {
        if (XGetWindowProperty(
                display, children[i], ATOM(__SWM_VROOT), 0, 1, False, XA_WINDOW, &type, &format, &nitems, &bytes,
                &buf) == Success &&
            type == XA_WINDOW) {
            win = *reinterpret_cast<Window *>(buf);
            XFree(buf);
            XFree(children);
            spdlog::debug(": desktop window ({}) found from __SWM_VROOT property", win);

            *p_root    = win;
            *p_desktop = win;
            return win;
        }

        if (buf) {
            XFree(buf);
            buf = 0;
        }
    }
    XFree(children);

    /* get subwindows from root */
    win = find_subwindow(root, -1, -1);

    display_width  = DisplayWidth(display, screen);
    display_height = DisplayHeight(display, screen);

    win = find_subwindow(win, display_width, display_height);

    if (buf) {
        XFree(buf);
        buf = 0;
    }

    if (win != root) {
        spdlog::debug("desktop window ({}) is subwindow of root window ({})", win, root);
    } else {
        spdlog::debug("desktop window ({}) is root window", win);
    }

    *p_root    = root;
    *p_desktop = win;

    return win;
}

sf::RenderWindow *getRenderWindow() {
    spdlog::info("finding desktop window");

    init_x11();
    if (!display) {
        return nullptr;
    }

    window.x      = 0;
    window.y      = 0;
    window.width  = static_cast<unsigned int>(DisplayWidth(display, screen));
    window.height = static_cast<unsigned int>(DisplayHeight(display, screen));

    if (!find_desktop_window(&window.root, &window.desktop)) {
        spdlog::error("Error: couldn't find desktop window");
        return nullptr;
    }

    spdlog::info("desktop window found width={} height={}", window.width, window.height);

    // Create an override_redirect True window.
    window.visual    = DefaultVisual(display, screen);
    window.colourmap = DefaultColormap(display, screen);

    int     depth = 0, flags = CWOverrideRedirect | CWBackingStore | CWBackPixel;
    Visual *visual = nullptr;
    Atom    xa;

    depth  = CopyFromParent;
    visual = CopyFromParent;

    XSetWindowAttributes attrs = {
        ParentRelative, 0L, 0, 0L, 0, 0, Always, 0L, 0L, False, StructureNotifyMask | ExposureMask, 0L, True, 0, 0};
    window.window = XCreateWindow(
        display, window.root, window.x, window.y, window.width, window.height, 0, depth, InputOutput, visual,
        static_cast<unsigned long>(flags), &attrs);
    XLowerWindow(display, window.window);
    xa = ATOM(_NET_WM_WINDOW_TYPE);
    Atom prop;
    prop = ATOM(_NET_WM_WINDOW_TYPE_DESKTOP);

    XChangeProperty(display, window.window, xa, XA_ATOM, 32, PropModeReplace, (unsigned char *)&prop, 1);
    spdlog::info("creating SFML render window");

    // this is the window we want to draw to with SFML.

    sf::RenderWindow *render_window = new sf::RenderWindow(window.window);

    return render_window;
}
