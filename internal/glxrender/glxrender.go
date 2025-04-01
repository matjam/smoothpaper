package glxrender

/*
#cgo LDFLAGS: -lGL -lX11 -lXrender -lva-glx
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <GL/gl.h>
#include <GL/glx.h>
#include <stdlib.h>
#include <string.h>

Atom get_atom(Display* dpy, const char* name) {
    return XInternAtom(dpy, name, False);
}

Window find_subwindow(Display* dpy, Window win, int screen, int width, int height) {
    Window root, parent, *children;
    unsigned int nchildren;

    for (int i = 0; i < 10; i++) {
        if (!XQueryTree(dpy, win, &root, &parent, &children, &nchildren)) {
            return win;
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

Window find_desktop_window(Display* dpy, int screen, Window* out_root) {
    Atom swm_vroot = get_atom(dpy, "__SWM_VROOT");
    Window root = RootWindow(dpy, screen);
    Window desktop = root;

    Window root_ret, parent_ret, *children;
    unsigned int nchildren;
    if (XQueryTree(dpy, root, &root_ret, &parent_ret, &children, &nchildren)) {
        for (unsigned int i = 0; i < nchildren; i++) {
            Atom actual_type;
            int actual_format;
            unsigned long nitems, bytes_after;
            unsigned char *prop = NULL;

            if (XGetWindowProperty(dpy, children[i], swm_vroot, 0, 1, False, XA_WINDOW,
                                   &actual_type, &actual_format, &nitems, &bytes_after, &prop) == Success &&
                actual_type == XA_WINDOW && prop) {
                desktop = *((Window*)prop);
                XFree(prop);
                break;
            }
            if (prop) XFree(prop);
        }
        XFree(children);
    }

    desktop = find_subwindow(dpy, desktop, screen, -1, -1);
    *out_root = root;
    return desktop;
}

Display* open_display() {
    return XOpenDisplay(NULL);
}

int get_display_width(Display* dpy, int screen) {
    return DisplayWidth(dpy, screen);
}

int get_display_height(Display* dpy, int screen) {
    return DisplayHeight(dpy, screen);
}

Window create_backed_window(Display* dpy, int screen, int x, int y, int width, int height) {
    Window root;
    Window desktop = find_desktop_window(dpy, screen, &root);

    XSetWindowAttributes attrs;
    attrs.override_redirect = True;
    attrs.backing_store = Always;
    attrs.background_pixel = BlackPixel(dpy, screen);
    attrs.event_mask = StructureNotifyMask | ExposureMask;

    unsigned long flags = CWOverrideRedirect | CWBackingStore | CWBackPixel | CWEventMask;
    Window win = XCreateWindow(
        dpy, desktop, x, y, width, height, 0,
        CopyFromParent, InputOutput,
        CopyFromParent,
        flags, &attrs);

    Atom wm_type = get_atom(dpy, "_NET_WM_WINDOW_TYPE");
    Atom wm_type_desktop = get_atom(dpy, "_NET_WM_WINDOW_TYPE_DESKTOP");
    XChangeProperty(dpy, win, wm_type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&wm_type_desktop, 1);

    XLowerWindow(dpy, win);
    XMapWindow(dpy, win);
    XFlush(dpy);
    return win;
}



*/
import "C"

import (
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/matjam/smoothpaper/internal/render"
)

type glxRenderer struct {
	display *C.Display
	window  C.Window
	context C.GLXContext
	width   int
	height  int

	texA texture
	texB texture

	start    time.Time
	duration time.Duration
	fading   bool

	scaleMode  render.ScalingMode
	easingMode render.EasingMode
	framerate  int
}

func NewRenderer(scale render.ScalingMode, easing render.EasingMode, framerate int) (render.Renderer, error) {
	runtime.LockOSThread()
	dpy := C.open_display()
	if dpy == nil {
		return nil, fmt.Errorf("unable to open X11 display")
	}
	screen := C.XDefaultScreen(dpy)
	width := int(C.get_display_width(dpy, screen))
	height := int(C.get_display_height(dpy, screen))
	win := C.create_backed_window(dpy, screen, 0, 0, C.int(width), C.int(height))

	attribs := []C.int{C.GLX_RGBA, C.GLX_DEPTH_SIZE, 24, C.GLX_DOUBLEBUFFER, 0}
	vi := C.glXChooseVisual(dpy, screen, &attribs[0])
	if vi == nil {
		return nil, fmt.Errorf("no suitable visual")
	}

	ctx := C.glXCreateContext(dpy, vi, nil, C.True)
	C.glXMakeCurrent(dpy, C.GLXDrawable(win), ctx)

	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("opengl init failed: %w", err)
	}
	gl.Viewport(0, 0, int32(width), int32(height))
	gl.ClearColor(0.0, 0.0, 0.0, 1.0)

	return &glxRenderer{
		display:    dpy,
		window:     win,
		context:    ctx,
		width:      width,
		height:     height,
		scaleMode:  scale,
		easingMode: easing,
		framerate:  framerate,
	}, nil
}

func (r *glxRenderer) GetSize() (int, int) {
	return r.width, r.height
}

func (r *glxRenderer) SetImage(img image.Image) error {
	if r.texA.id != 0 {
		gl.DeleteTextures(1, &r.texA.id)
	}
	t, err := r.createTexture(img)
	if err != nil {
		return err
	}
	r.texA = t
	r.fading = false
	return nil
}

// Transition to the next image with a fade effect. Will block until the transition is complete.
func (r *glxRenderer) Transition(next image.Image, duration time.Duration) error {
	if r.texA.id == 0 {
		log.Info("texA is nil, using a black texture")
		ta, err := r.createColorTexture(0, 0, 0)
		if err != nil {
			log.Errorf("failed to create texture: %v", err)
			return err
		}
		r.texA = ta
	}

	if r.texB.id != 0 {
		gl.DeleteTextures(1, &r.texB.id)
	}
	t, err := r.createTexture(next)
	if err != nil {
		return err
	}
	r.texB = t
	r.start = time.Now()
	r.duration = duration
	r.fading = true

	for r.fading {
		err = r.Render()
		if err != nil {
			return err
		}
	}

	return nil
}

// Render the current image; this blocks for the given frame rate. Ideally, you do not
// need to call this directly, as it is called in a loop by the renderer during Transition.
func (r *glxRenderer) Render() error {
	alpha := float32(1.0)
	if r.fading {
		t := float32(time.Since(r.start).Seconds() / r.duration.Seconds())
		if t >= 1.0 {
			t = 1.0
			deleteTexture(&r.texA)
			r.texA = r.texB

			r.texB.id = 0
			r.texB.width = 0
			r.texB.height = 0

			r.fading = false
		}
		alpha = applyEasing(r.easingMode, t)
	}

	if !r.fading {
		r.renderStatic(r.texA)
	} else {
		r.renderFade(alpha, r.texA, r.texB)
	}
	C.glXSwapBuffers(r.display, C.GLXDrawable(r.window))
	time.Sleep(time.Second / time.Duration(r.framerate))
	return nil
}

type texture struct {
	id     uint32
	width  int
	height int
}

func (r *glxRenderer) createTexture(img image.Image) (texture, error) {
	var tex texture

	bounds := img.Bounds()
	tex.width = bounds.Dx()
	tex.height = bounds.Dy()

	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	gl.GenTextures(1, &tex.id)
	gl.BindTexture(gl.TEXTURE_2D, tex.id)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		int32(tex.width), int32(tex.height), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	gl.GenerateMipmap(gl.TEXTURE_2D)

	return tex, nil
}

func (r *glxRenderer) Cleanup() {
	if r.texA.id != 0 {
		gl.DeleteTextures(1, &r.texA.id)
	}
	if r.texB.id != 0 {
		gl.DeleteTextures(1, &r.texB.id)
	}
	C.glXMakeCurrent(r.display, 0, nil)
	C.glXDestroyContext(r.display, r.context)
	C.XDestroyWindow(r.display, r.window)
	C.XCloseDisplay(r.display)
}

func applyEasing(mode render.EasingMode, t float32) float32 {
	switch mode {
	case render.EasingLinear:
		return t
	case render.EasingEaseIn:
		return t * t
	case render.EasingEaseOut:
		return t * (2 - t)
	case render.EasingEaseInOut:
		if t < 0.5 {
			return 2 * t * t
		} else {
			return -1 + (4-2*t)*t
		}
	default:
		return t
	}
}

func (r *glxRenderer) renderFade(alpha float32, texA, texB texture) {
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Enable(gl.TEXTURE_2D)

	if texA.id != 0 && alpha < 1.0 {
		gl.Color4f(1, 1, 1, 1.0-alpha)
		gl.BindTexture(gl.TEXTURE_2D, texA.id)
		r.drawCenteredQuad(texA)
	}
	if texB.id != 0 {
		gl.Color4f(1, 1, 1, alpha)
		gl.BindTexture(gl.TEXTURE_2D, texB.id)
		r.drawCenteredQuad(texB)
	}

	gl.Disable(gl.BLEND)
}
func (r *glxRenderer) renderStatic(tex texture) {
	if tex.id == 0 {
		return
	}
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.Enable(gl.TEXTURE_2D)
	gl.BindTexture(gl.TEXTURE_2D, tex.id)
	gl.Color4f(1, 1, 1, 1)
	r.drawCenteredQuad(tex)
}

func (r *glxRenderer) drawCenteredQuad(tex texture) {
	tw := float32(tex.width)
	th := float32(tex.height)
	sw := float32(r.width)
	sh := float32(r.height)

	tAspect := tw / th
	sAspect := sw / sh

	var hx, hy float32

	switch r.scaleMode {
	case render.ScalingModeStretch:
		hx, hy = 1.0, 1.0

	case render.ScalingModeFitHorizontal: // "horizontal"
		hx = 1.0
		// height scaled proportionally to width
		hy = (th / tw) * (sw / sh)

	case render.ScalingModeFitVertical: // "vertical"
		hy = 1.0
		// width scaled proportionally to height
		hx = (tw / th) * (sh / sw)

	case render.ScalingModeCenter:
		fallthrough
	default:
		// Fill as much as possible without cropping or stretching
		if tAspect > sAspect {
			hx = 1.0
			hy = (sh / sw) * (tw / th)
		} else {
			hy = 1.0
			hx = (sw / sh) * (th / tw)
		}

		// Clamp to avoid overflow (optional, depending on design)
		if hx > 1.0 {
			hx = 1.0
		}
		if hy > 1.0 {
			hy = 1.0
		}
	}

	gl.Begin(gl.QUADS)
	gl.TexCoord2f(0.0, 1.0)
	gl.Vertex2f(-hx, -hy)
	gl.TexCoord2f(1.0, 1.0)
	gl.Vertex2f(hx, -hy)
	gl.TexCoord2f(1.0, 0.0)
	gl.Vertex2f(hx, hy)
	gl.TexCoord2f(0.0, 0.0)
	gl.Vertex2f(-hx, hy)
	gl.End()
}

func deleteTexture(tex *texture) {
	if tex.id != 0 {
		gl.DeleteTextures(1, &tex.id)
		tex.id = 0
		tex.width = 0
		tex.height = 0
	}
}

func (r *glxRenderer) createColorTexture(rVal, gVal, bVal uint8) (texture, error) {
	const w, h = 2, 2           // Small but safe texture size
	pix := make([]uint8, w*h*4) // RGBA

	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = rVal
		pix[i+1] = gVal
		pix[i+2] = bVal
		pix[i+3] = 255
	}

	var tex texture
	tex.width = w
	tex.height = h

	gl.GenTextures(1, &tex.id)
	gl.BindTexture(gl.TEXTURE_2D, tex.id)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pix))

	return tex, nil
}
