package glrender

/*
#cgo LDFLAGS: -lGL -lX11
#include <GL/gl.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <stdlib.h>
#include <string.h>

void set_window_override_redirect(Display* display, Window win) {
    XSetWindowAttributes attrs;
    attrs.override_redirect = True;
    XChangeWindowAttributes(display, win, CWOverrideRedirect, &attrs);
}

void set_net_wm_window_type_desktop(Display* display, Window win) {
    Atom net_wm_window_type = XInternAtom(display, "_NET_WM_WINDOW_TYPE", False);
    Atom net_wm_window_type_desktop = XInternAtom(display, "_NET_WM_WINDOW_TYPE_DESKTOP", False);
    XChangeProperty(display, win, net_wm_window_type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&net_wm_window_type_desktop, 1);
}
*/
import "C"

import (
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"sync"
	"time"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type ScalingMode string

const (
	ScalingModeCenter        ScalingMode = "center"
	ScalingModeStretch       ScalingMode = "stretched"
	ScalingModeFitHorizontal ScalingMode = "horizontal"
	ScalingModeFitVertical   ScalingMode = "vertical"
)

type EasingMode string

const (
	EasingLinear    EasingMode = "linear"
	EasingEaseIn    EasingMode = "ease-in"
	EasingEaseOut   EasingMode = "ease-out"
	EasingEaseInOut EasingMode = "ease-in-out"
)

type Renderer interface {
	SetImage(image image.Image) error                          // Set the current image
	Transition(next image.Image, duration time.Duration) error // Transition to the next image
	Render() error                                             // Render the current image, called in a loop and will block for each frame
	Cleanup()                                                  // Cleanup resources
	GetSize() (int, int)                                       // Get the dimensions of the window
}

// NewRenderer creates a new Renderer instance.
func NewRenderer(scaleMode ScalingMode, easingMode EasingMode, framerate int) (Renderer, error) {
	return newGLFWRenderer(scaleMode, easingMode, framerate)
}

type glfwRenderer struct {
	win        *glfw.Window
	scaleMode  ScalingMode
	easingMode EasingMode
	framerate  int

	mu        sync.Mutex
	texA      uint32
	texB      uint32
	fadeStart time.Time
	fadeDur   time.Duration
	isFading  bool
}

func newGLFWRenderer(scale ScalingMode, easing EasingMode, framerate int) (*glfwRenderer, error) {
	runtime.LockOSThread()
	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("glfw init failed: %w", err)
	}
	glfw.WindowHint(glfw.Decorated, glfw.False)
	glfw.WindowHint(glfw.Focused, glfw.False)
	glfw.WindowHint(glfw.Floating, glfw.False)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.Visible, glfw.False) // prevent auto-map

	vidMode := glfw.GetPrimaryMonitor().GetVideoMode()
	win, err := glfw.CreateWindow(vidMode.Width, vidMode.Height, "smoothpaper", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create window failed: %w", err)
	}

	// --- Begin compositor-safe setup ---
	display := C.XOpenDisplay(nil)
	if display != nil {
		displayWindow := C.Window(win.GetX11Window())
		C.set_window_override_redirect(display, displayWindow)
		C.set_net_wm_window_type_desktop(display, displayWindow)
		C.XMapWindow(display, displayWindow)
		C.XLowerWindow(display, displayWindow)
		C.XFlush(display)
	}
	// --- End compositor-safe setup ---

	win.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("gl init failed: %w", err)
	}
	gl.Viewport(0, 0, int32(vidMode.Width), int32(vidMode.Height))
	gl.ClearColor(0.0, 0.0, 0.0, 1.0)

	if framerate <= 0 {
		framerate = 60
	} else if framerate > 240 {
		framerate = 240
	}

	return &glfwRenderer{
		win:        win,
		scaleMode:  scale,
		easingMode: easing,
		framerate:  framerate,
	}, nil
}

func (r *glfwRenderer) GetSize() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.win.GetSize()
}

func (r *glfwRenderer) SetImage(img image.Image) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.texA != 0 {
		gl.DeleteTextures(1, &r.texA)
	}
	t, err := r.createTexture(img)
	if err != nil {
		return err
	}
	r.texA = t
	r.isFading = false
	return nil
}

func (r *glfwRenderer) Transition(next image.Image, duration time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.texB != 0 {
		gl.DeleteTextures(1, &r.texB)
	}
	t, err := r.createTexture(next)
	if err != nil {
		return err
	}
	r.texB = t
	r.fadeStart = time.Now()
	r.fadeDur = duration
	r.isFading = true
	return nil
}

func (r *glfwRenderer) Render() error {
	r.mu.Lock()
	texA := r.texA
	texB := r.texB
	fadeStart := r.fadeStart
	fadeDur := r.fadeDur
	isFading := r.isFading
	r.mu.Unlock()

	if isFading {
		t := time.Since(fadeStart).Seconds() / fadeDur.Seconds()
		if t > 1.0 {
			t = 1.0

			r.mu.Lock()
			if r.texA != 0 {
				gl.DeleteTextures(1, &r.texA)
			}
			r.texA = r.texB
			r.texB = 0
			r.isFading = false
			r.mu.Unlock()
		}
		r.renderFade(float32(applyEasing(r.easingMode, t)), texA, texB)
	} else {
		r.renderStatic(texA)
	}
	r.win.SwapBuffers()
	glfw.PollEvents()
	time.Sleep(time.Second / time.Duration(r.framerate))
	return nil
}

func (r *glfwRenderer) renderStatic(tex uint32) {
	if tex == 0 {
		return
	}
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.Enable(gl.TEXTURE_2D)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.Color4f(1, 1, 1, 1)
	drawQuad()
}

func (r *glfwRenderer) renderFade(alpha float32, texA, texB uint32) {
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

func drawQuad() {
	gl.Begin(gl.QUADS)
	gl.TexCoord2f(0, 1)
	gl.Vertex2f(-1, -1)
	gl.TexCoord2f(1, 1)
	gl.Vertex2f(1, -1)
	gl.TexCoord2f(1, 0)
	gl.Vertex2f(1, 1)
	gl.TexCoord2f(0, 0)
	gl.Vertex2f(-1, 1)
	gl.End()
}

func (r *glfwRenderer) createTexture(img image.Image) (uint32, error) {
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

func (r *glfwRenderer) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.texA != 0 {
		gl.DeleteTextures(1, &r.texA)
	}
	if r.texB != 0 {
		gl.DeleteTextures(1, &r.texB)
	}
	if r.win != nil {
		r.win.Destroy()
	}
	glfw.Terminate()
}

func applyEasing(name EasingMode, t float64) float64 {
	switch name {
	case EasingLinear:
		return t
	case EasingEaseIn:
		return t * t
	case EasingEaseOut:
		return t * (2 - t)
	case EasingEaseInOut:
		if t < 0.5 {
			return 2 * t * t
		} else {
			return -1 + (4-2*t)*t
		}
	default:
		return t
	}
}
