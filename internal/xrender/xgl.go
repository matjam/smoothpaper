package xrender

/*
#cgo LDFLAGS: -lX11 -lGL -lGLEW
#include <X11/Xlib.h>
#include <GL/glx.h>
#include <GL/gl.h>
#include <stdlib.h>

Display* open_display() {
    return XOpenDisplay(NULL);
}

GLXContext create_gl_context(Display* display, Window win) {
    static int visual_attribs[] = {
        GLX_RGBA,
        GLX_DEPTH_SIZE, 24,
        GLX_DOUBLEBUFFER,
        None
    };
    int screen = DefaultScreen(display);
    XVisualInfo* vi = glXChooseVisual(display, screen, visual_attribs);
    if (!vi) return NULL;
    GLXContext ctx = glXCreateContext(display, vi, NULL, GL_TRUE);
    glXMakeCurrent(display, win, ctx);
    return ctx;
}

void swap_buffers(Display* display, Window win) {
    glXSwapBuffers(display, win);
}

void destroy_gl_context(Display* display, GLXContext ctx) {
    glXMakeCurrent(display, None, NULL);
    glXDestroyContext(display, ctx);
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
	"github.com/spf13/viper"
)

type Renderer struct {
	display  *C.Display
	window   C.Window
	context  C.GLXContext
	width    int
	height   int
	textureA uint32
	textureB uint32
}

func NewRenderer(display *C.Display, window C.Window, width, height int) (*Renderer, error) {
	runtime.LockOSThread()

	ctx := C.create_gl_context(display, window)
	if ctx == nil {
		return nil, fmt.Errorf("failed to create OpenGL context")
	}

	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenGL: %w", err)
	}

	gl.Viewport(0, 0, int32(width), int32(height))
	gl.ClearColor(0.1, 0.1, 0.1, 1.0)

	return &Renderer{
		display: display,
		window:  window,
		context: ctx,
		width:   width,
		height:  height,
	}, nil
}

func (r *Renderer) LoadTextures(imgA, imgB image.Image) error {
	// Cleanup old textures if any
	if r.textureA != 0 {
		gl.DeleteTextures(1, &r.textureA)
		r.textureA = 0
	}
	if r.textureB != 0 {
		gl.DeleteTextures(1, &r.textureB)
		r.textureB = 0
	}

	tA, err := createTexture(imgA)
	if err != nil {
		return err
	}
	tB, err := createTexture(imgB)
	if err != nil {
		gl.DeleteTextures(1, &tA)
		return err
	}
	r.textureA = tA
	r.textureB = tB
	return nil
}

func createTexture(img image.Image) (uint32, error) {
	imgRGBA := image.NewRGBA(img.Bounds())
	draw.Draw(imgRGBA, imgRGBA.Bounds(), img, img.Bounds().Min, draw.Src)
	rgba := FlipVertical(imgRGBA)

	if i, ok := img.(*image.RGBA); ok {
		rgba = i
	} else {
		rgba = image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)
	}

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	gl.TexImage2D(
		gl.TEXTURE_2D, 0, gl.RGBA,
		int32(rgba.Rect.Dx()), int32(rgba.Rect.Dy()), 0,
		gl.RGBA, gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix),
	)
	return tex, nil
}

func (r *Renderer) RenderFade(alpha float32) {
	gl.ClearColor(0.0, 0.0, 0.0, 1.0) // <- explicitly black
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Draw texture A with alpha 1 - alpha
	gl.Color4f(1, 1, 1, 1.0-alpha)
	gl.BindTexture(gl.TEXTURE_2D, r.textureA)
	drawQuad()

	// Draw texture B with alpha
	gl.Color4f(1, 1, 1, alpha)
	gl.BindTexture(gl.TEXTURE_2D, r.textureB)
	drawQuad()

	gl.Disable(gl.BLEND)
	C.swap_buffers(r.display, r.window)
}

func drawQuad() {
	gl.Enable(gl.TEXTURE_2D)
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

func (r *Renderer) RenderFadeWithEasing(duration time.Duration, easing EasingMode) {
	framerate := viper.GetInt("framerate_limit")
	if framerate == 0 || framerate > 240 || framerate < 1 {
		framerate = 30
	}
	frameDelay := time.Second / time.Duration(framerate)
	frames := int(duration / frameDelay)

	start := time.Now()

	for i := 0; i <= frames; i++ {
		elapsed := time.Since(start)
		t := min(float64(elapsed)/float64(duration), 1.0)
		alpha := applyEasing(easing, t)
		r.RenderFade(float32(alpha))
		time.Sleep(frameDelay)
	}
}

type EasingMode string

const (
	EasingLinear    EasingMode = "linear"
	EasingEaseIn    EasingMode = "ease-in"
	EasingEaseOut   EasingMode = "ease-out"
	EasingEaseInOut EasingMode = "ease-in-out"
)

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

func FlipVertical(img *image.RGBA) *image.RGBA {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	flipped := image.NewRGBA(bounds)

	for y := 0; y < h; y++ {
		srcStart := y * img.Stride
		dstStart := (h - 1 - y) * flipped.Stride
		copy(flipped.Pix[dstStart:dstStart+w*4], img.Pix[srcStart:srcStart+w*4])
	}

	return flipped
}

func (r *Renderer) Cleanup() {
	if r.textureA != 0 {
		gl.DeleteTextures(1, &r.textureA)
		r.textureA = 0
	}
	if r.textureB != 0 {
		gl.DeleteTextures(1, &r.textureB)
		r.textureB = 0
	}
	if r.context != nil {
		C.destroy_gl_context(r.display, r.context)
		r.context = nil
	}
}
