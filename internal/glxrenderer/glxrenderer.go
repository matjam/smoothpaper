package glxrenderer

/*
#cgo LDFLAGS: -lGL -lX11 -lXrender -lva-glx
#include "glxrenderer.h"
*/
import "C"

import (
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"time"
	"unsafe"

	"github.com/charmbracelet/log"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/matjam/smoothpaper/internal/types"
)

// glxRenderer is the primary type that wraps the OpenGL context and X11 windowing.
// It uses GLX to interface between OpenGL and the X11 system.
type GLXRenderer struct {
	display *C.Display   // C pointer to the X11 display connection
	window  C.Window     // The X11 window used for rendering
	context C.GLXContext // The OpenGL context used with GLX
	width   int          // Width of the window in pixels
	height  int          // Height of the window in pixels

	texA texture // Primary texture (the currently displayed image)
	texB texture // Secondary texture (used for transitioning)

	start    time.Time     // Start time of the transition
	duration time.Duration // Duration of the transition
	fading   bool          // Whether a transition is currently in progress

	scaleMode  types.ScalingMode // How images should scale (stretch, fit, center, etc.)
	easingMode types.EasingMode  // The easing function to apply to alpha blending
	framerate  int               // Frame rate to maintain during rendering
}

// NewRenderer initializes the GLX context, creates a fullscreen override-redirect X11 window,
// and binds it to an OpenGL context so we can start rendering.
func NewRenderer(scale types.ScalingMode, easing types.EasingMode, framerate int) (*GLXRenderer, error) {
	runtime.LockOSThread() // Required: OpenGL contexts must be accessed from a single OS thread

	dpy := C.open_display() // Calls XOpenDisplay(NULL), connects to X11 server using DISPLAY env var
	if dpy == nil {
		return nil, fmt.Errorf("unable to open X11 display")
	}
	C.set_io_error_handler() // Prevents crashing if display disappears

	screen := C.XDefaultScreen(dpy) // Returns the default screen index for the display
	width := int(C.get_display_width(dpy, screen))
	height := int(C.get_display_height(dpy, screen))
	win := C.create_backed_window(dpy, screen, 0, 0, C.int(width), C.int(height)) // Creates a special window used as our OpenGL target

	// These attributes request a visual (pixel format) with RGBA, 24-bit depth, and double buffering
	attribs := []C.int{C.GLX_RGBA, C.GLX_DEPTH_SIZE, 24, C.GLX_DOUBLEBUFFER, 0}
	vi := C.glXChooseVisual(dpy, screen, &attribs[0]) // Finds an appropriate visual configuration
	if vi == nil {
		return nil, fmt.Errorf("no suitable visual")
	}

	ctx := C.glXCreateContext(dpy, vi, nil, C.True) // Creates an OpenGL context associated with the visual
	C.glXMakeCurrent(dpy, C.GLXDrawable(win), ctx)  // Makes this OpenGL context current to the created window

	// Initialize Go-side OpenGL bindings
	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("opengl init failed: %w", err)
	}
	gl.Viewport(0, 0, int32(width), int32(height)) // Sets up the viewport to match the window size
	gl.ClearColor(0.0, 0.0, 0.0, 1.0)              // Default clear color is opaque black

	return &GLXRenderer{
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

// SetRootPixmap reads pixels from the OpenGL backbuffer, flips vertically, and sets the root pixmap
func (r *GLXRenderer) SetRootPixmap() {
	w, h := r.width, r.height
	size := w * h * 4
	buf := make([]byte, size)
	gl.ReadPixels(0, 0, int32(w), int32(h), gl.BGRA, gl.UNSIGNED_BYTE, gl.Ptr(buf))

	// Flip vertically
	flipped := make([]byte, size)
	stride := w * 4
	for y := 0; y < h; y++ {
		src := buf[y*stride : (y+1)*stride]
		dst := flipped[(h-1-y)*stride : (h-y)*stride]
		copy(dst, src)
	}

	C.set_root_pixmap(r.display, C.XDefaultScreen(r.display), (*C.uchar)(unsafe.Pointer(&flipped[0])), C.int(w), C.int(h))
}

// GetSize returns the dimensions of the rendering window.
func (r *GLXRenderer) GetSize() (int, int) {
	return r.width, r.height
}

// SetImage loads a new image into texA. Any existing texture is deleted first.
func (r *GLXRenderer) SetImage(img image.Image) error {
	if r.texA.id != 0 {
		gl.DeleteTextures(1, &r.texA.id)
	}
	t, err := r.createTexture(img) // Converts Go image to OpenGL texture
	if err != nil {
		return err
	}
	r.texA = t
	r.fading = false
	return nil
}

// Transition sets up texB and blends it over texA for a specified duration using easing.
func (r *GLXRenderer) Transition(next image.Image, duration time.Duration) error {
	if r.texA.id == 0 {
		tex, err := r.createColorTexture(0, 0, 0)
		if err != nil {
			log.Errorf("failed to create fallback texture: %v", err)
			return err
		}
		r.texA = *tex

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

	// Main rendering loop for the duration of the transition
	for r.fading {
		err = r.Render()
		if err != nil {
			return err
		}
	}

	// After fade completes, set the backbuffer image as the root pixmap
	r.SetRootPixmap()

	return nil
}

// Render the current image; this blocks for the given frame rate. Ideally, you do not
// need to call this directly, as it is called in a loop by the renderer during Transition.
func (r *GLXRenderer) Render() error {
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

	// Swaps the front and back buffer to update the screen
	C.glXSwapBuffers(r.display, C.GLXDrawable(r.window))
	time.Sleep(time.Second / time.Duration(r.framerate))
	return nil
}

// texture holds an OpenGL texture ID and its size.
type texture struct {
	id     uint32
	width  int
	height int
}

// createTexture takes a Go image.Image and turns it into an OpenGL texture.
func (r *GLXRenderer) createTexture(img image.Image) (texture, error) {
	var tex texture
	bounds := img.Bounds()
	tex.width = bounds.Dx()
	tex.height = bounds.Dy()

	// Convert to RGBA (required format for OpenGL upload)
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Generate and bind OpenGL texture ID
	gl.GenTextures(1, &tex.id)
	gl.BindTexture(gl.TEXTURE_2D, tex.id)

	// Set texture parameters
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	// Upload pixel data to GPU
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		int32(tex.width), int32(tex.height), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	gl.GenerateMipmap(gl.TEXTURE_2D) // Create mipmaps for smoother scaling
	return tex, nil
}

func (r *GLXRenderer) Cleanup() {
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

func applyEasing(mode types.EasingMode, t float32) float32 {
	switch mode {
	case types.EasingLinear:
		return t
	case types.EasingEaseIn:
		return t * t
	case types.EasingEaseOut:
		return t * (2 - t)
	case types.EasingEaseInOut:
		if t < 0.5 {
			return 2 * t * t
		} else {
			return -1 + (4-2*t)*t
		}
	default:
		return t
	}
}

func (r *GLXRenderer) renderFade(alpha float32, texA, texB texture) {
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
func (r *GLXRenderer) renderStatic(tex texture) {
	if tex.id == 0 {
		return
	}
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.Enable(gl.TEXTURE_2D)
	gl.BindTexture(gl.TEXTURE_2D, tex.id)
	gl.Color4f(1, 1, 1, 1)
	r.drawCenteredQuad(tex)
}

func (r *GLXRenderer) drawCenteredQuad(tex texture) {
	tw := float32(tex.width)
	th := float32(tex.height)
	sw := float32(r.width)
	sh := float32(r.height)

	tAspect := tw / th
	sAspect := sw / sh

	var hx, hy float32

	switch r.scaleMode {
	case types.ScalingModeStretch:
		hx, hy = 1.0, 1.0

	case types.ScalingModeFitHorizontal: // "horizontal"
		hx = 1.0
		// height scaled proportionally to width
		hy = (th / tw) * (sw / sh)

	case types.ScalingModeFitVertical: // "vertical"
		hy = 1.0
		// width scaled proportionally to height
		hx = (tw / th) * (sh / sw)

	case types.ScalingModeCenter:
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

func (r *GLXRenderer) createColorTexture(rVal, gVal, bVal uint8) (*texture, error) {
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

	return &tex, nil
}

func (r *GLXRenderer) IsDisplayRunning() bool {
	if r.display == nil {
		return false
	}
	return C.is_display_dead() == 0
}
