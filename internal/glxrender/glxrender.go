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

Display* open_display() {
    return XOpenDisplay(NULL);
}

Window get_root_window(Display* dpy) {
    return DefaultRootWindow(dpy);
}

int get_display_width(Display* dpy, int screen) {
    return DisplayWidth(dpy, screen);
}

int get_display_height(Display* dpy, int screen) {
    return DisplayHeight(dpy, screen);
}

Window create_backed_window(Display* dpy, Window parent, int screen, int x, int y, int width, int height) {
    XSetWindowAttributes attrs;
    attrs.backing_store = Always;
    attrs.background_pixel = BlackPixel(dpy, screen);

    Window win = XCreateWindow(
        dpy, parent, x, y, width, height, 0,
        DefaultDepth(dpy, screen), InputOutput,
        DefaultVisual(dpy, screen),
        CWBackingStore | CWBackPixel, &attrs
    );

    Atom wm_type = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE", False);
    Atom wm_type_desktop = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE_DESKTOP", False);
    XChangeProperty(dpy, win, wm_type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&wm_type_desktop, 1);

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

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/matjam/smoothpaper/internal/render"
)

type glxRenderer struct {
	display *C.Display
	window  C.Window
	context C.GLXContext
	width   int
	height  int

	texA       uint32
	texB       uint32
	start      time.Time
	duration   time.Duration
	fading     bool
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
	root := C.get_root_window(dpy)
	win := C.create_backed_window(dpy, root, screen, 0, 0, C.int(width), C.int(height))

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
	if r.texA != 0 {
		gl.DeleteTextures(1, &r.texA)
	}
	t, err := r.createTexture(img)
	if err != nil {
		return err
	}
	r.texA = t
	r.fading = false
	return nil
}

func (r *glxRenderer) Transition(next image.Image, duration time.Duration) error {
	if r.texB != 0 {
		gl.DeleteTextures(1, &r.texB)
	}
	t, err := r.createTexture(next)
	if err != nil {
		return err
	}
	r.texB = t
	r.start = time.Now()
	r.duration = duration
	r.fading = true
	return nil
}

func (r *glxRenderer) Render() error {
	alpha := 1.0
	if r.fading {
		t := time.Since(r.start).Seconds() / r.duration.Seconds()
		if t >= 1.0 {
			r.fading = false
			gl.DeleteTextures(1, &r.texA)
			r.texA = r.texB
			r.texB = 0
			t = 1.0
		}
		alpha = applyEasing(r.easingMode, t)
	}

	r.renderFade(float32(alpha), r.texA, r.texB)
	C.glXSwapBuffers(r.display, C.GLXDrawable(r.window))
	time.Sleep(time.Second / time.Duration(r.framerate))
	return nil
}

func (r *glxRenderer) renderFade(alpha float32, texA, texB uint32) {
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Enable(gl.TEXTURE_2D)

	if texA != 0 && alpha < 1.0 {
		gl.Color4f(1, 1, 1, 1.0-alpha)
		gl.BindTexture(gl.TEXTURE_2D, texA)
		drawQuad()
	}
	if texB != 0 {
		gl.Color4f(1, 1, 1, alpha)
		gl.BindTexture(gl.TEXTURE_2D, texB)
		drawQuad()
	}
	gl.Disable(gl.BLEND)
}

func (r *glxRenderer) createTexture(img image.Image) (uint32, error) {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		int32(rgba.Rect.Dx()), int32(rgba.Rect.Dy()), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))
	return tex, nil
}

func (r *glxRenderer) Cleanup() {
	if r.texA != 0 {
		gl.DeleteTextures(1, &r.texA)
	}
	if r.texB != 0 {
		gl.DeleteTextures(1, &r.texB)
	}
	C.glXMakeCurrent(r.display, 0, nil)
	C.glXDestroyContext(r.display, r.context)
	C.XDestroyWindow(r.display, r.window)
	C.XCloseDisplay(r.display)
}

func applyEasing(mode render.EasingMode, t float64) float64 {
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

func drawQuad() {
	gl.Begin(gl.QUADS)
	gl.TexCoord2f(0.0, 1.0)
	gl.Vertex2f(-1.0, -1.0)
	gl.TexCoord2f(1.0, 1.0)
	gl.Vertex2f(1.0, -1.0)
	gl.TexCoord2f(1.0, 0.0)
	gl.Vertex2f(1.0, 1.0)
	gl.TexCoord2f(0.0, 0.0)
	gl.Vertex2f(-1.0, 1.0)
	gl.End()
}
