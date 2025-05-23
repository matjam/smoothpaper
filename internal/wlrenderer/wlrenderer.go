package wlrenderer

/*
#cgo LDFLAGS: -lwayland-client -lwayland-egl -lEGL -lGLESv2
#include "wlrenderer.h"
*/
import "C"

import (
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"runtime/cgo"
	"time"
	"unsafe"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/types"
)

type texture struct {
	id            C.GLuint
	width, height int
}

type WLRenderer struct {
	width      int
	height     int
	scaleMode  types.ScalingMode
	easingMode types.EasingMode
	framerate  int

	// Wayland core
	display    *C.struct_wl_display
	registry   *C.struct_wl_registry
	surface    *C.struct_wl_surface
	layerSurf  *C.struct_zwlr_layer_surface_v1
	layerShell *C.struct_zwlr_layer_shell_v1
	eglDisplay C.EGLDisplay
	eglContext C.EGLContext
	eglSurface C.EGLSurface

	currentTex    texture
	transitionTex texture
	blackTex      texture

	start    time.Time
	duration time.Duration
	fading   bool

	shaderProgram C.GLuint
	attribPos     C.GLint
	attribTex     C.GLint
	uniformTex    C.GLint
	uniformAlpha  C.GLint

	registryHandle cgo.Handle

	compositor *C.struct_wl_compositor

	configured   bool
	configWidth  int
	configHeight int
	configChan   chan struct{}
}

func NewRenderer(scale types.ScalingMode, easing types.EasingMode, framerate int) (*WLRenderer, error) {
	runtime.LockOSThread() // Required: OpenGL contexts must be accessed from a single OS thread

	r := &WLRenderer{
		scaleMode:  scale,
		easingMode: easing,
		framerate:  framerate,
		configChan: make(chan struct{}, 1),
	}

	if err := r.connectToDisplay(); err != nil {
		return nil, err
	}

	return r, nil
}

//export goHandleGlobal
func goHandleGlobal(handle C.uintptr_t, registry *C.struct_wl_registry, name C.uint32_t, iface *C.char, _ C.uint32_t) {
	h := cgo.Handle(uintptr(handle))
	r := h.Value().(*WLRenderer)
	if r == nil {
		log.Error("goHandleGlobal: nil renderer")
		return
	}

	goIface := C.GoString(iface)

	switch goIface {
	case "zwlr_layer_shell_v1":
		r.layerShell = (*C.struct_zwlr_layer_shell_v1)(C.wl_registry_bind(registry, name, &C.zwlr_layer_shell_v1_interface, 1))
		log.Debug("bound zwlr_layer_shell_v1")
	case "wl_compositor":
		r.compositor = (*C.struct_wl_compositor)(C.wl_registry_bind(registry, name, &C.wl_compositor_interface, 1))
		log.Debug("bound wl_compositor")
	}
}

//export goHandleGlobalRemove
func goHandleGlobalRemove(handle C.uintptr_t, _ *C.struct_wl_registry, name C.uint32_t) {
	h := cgo.Handle(uintptr(handle))
	r := h.Value().(*WLRenderer)
	if r == nil {
		log.Error("goHandleGlobalRemove: nil renderer")
		return
	}

	log.Debugf("Global removed: name=%d", name)
	// Check if this was a crucial interface and handle accordingly
}

//export goHandleLayerSurfaceConfigure
func goHandleLayerSurfaceConfigure(handle C.uintptr_t, surface *C.struct_zwlr_layer_surface_v1,
	serial C.uint32_t, width, height C.uint32_t) {
	log.Debugf("goHandleLayerSurfaceConfigure: handle=%d, surface=%p, serial=%d, width=%d, height=%d",
		handle, surface, serial, width, height)

	h := cgo.Handle(uintptr(handle))
	r := h.Value().(*WLRenderer)
	if r == nil {
		log.Error("goHandleLayerSurfaceConfigure: nil renderer")
		return
	}

	log.Debugf("Layer surface configured: width=%d, height=%d", width, height)

	// Acknowledge the configure
	C.zwlr_layer_surface_v1_ack_configure(surface, serial)

	// Store the configuration
	r.configWidth = int(width)
	r.configHeight = int(height)
	r.configured = true

	// Signal that configuration is complete
	select {
	case r.configChan <- struct{}{}:
	default:
	}
}

//export goHandleLayerSurfaceClosed
func goHandleLayerSurfaceClosed(handle C.uintptr_t, _ *C.struct_zwlr_layer_surface_v1) {
	log.Debugf("goHandleLayerSurfaceClosed: handle=%d", handle)

	h := cgo.Handle(uintptr(handle))
	r := h.Value().(*WLRenderer)
	if r == nil {
		log.Error("goHandleLayerSurfaceClosed: nil renderer")
		return
	}

	log.Debug("Layer surface closed")
	// Mark the layer surface as closed
	r.layerSurf = nil
	r.display = nil
	r.configured = false
	r.configWidth = 0
	r.configHeight = 0
	r.eglSurface = nil
	r.eglContext = nil
	r.eglDisplay = 0
}

func connectWaylandDisplay() (*C.struct_wl_display, error) {
	display := C.connect_wayland_display()
	if display == nil {
		return nil, fmt.Errorf("failed to connect to Wayland display")
	}
	return display, nil
}

func setupRegistry(r *WLRenderer) error {
	r.registry = C.wl_display_get_registry(r.display)
	if r.registry == nil {
		return fmt.Errorf("failed to get Wayland registry")
	}
	handle := cgo.NewHandle(r)
	r.registryHandle = handle
	C.wl_registry_add_listener(r.registry, C.get_registry_listener(), unsafe.Pointer(uintptr(handle)))
	C.wl_display_roundtrip(r.display)
	C.wl_display_roundtrip(r.display)
	runtime.KeepAlive(r)
	return nil
}

func createLayerSurface(r *WLRenderer) error {
	if r.layerShell == nil || r.surface == nil {
		return fmt.Errorf("required layer-shell or surface not available")
	}

	// Initialize configChan if not already done
	if r.configChan == nil {
		r.configChan = make(chan struct{}, 1)
	}

	output := (*C.struct_wl_output)(nil)
	layer := C.uint32_t(C.ZWLR_LAYER_SHELL_V1_LAYER_BACKGROUND)
	namespace := C.CString("smoothpaper")
	defer C.free(unsafe.Pointer(namespace))

	// First create the layer surface
	r.layerSurf = C.zwlr_layer_shell_v1_get_layer_surface(
		r.layerShell, r.surface, output, layer, namespace,
	)
	if r.layerSurf == nil {
		return fmt.Errorf("failed to create layer surface")
	}

	// Then add the listener to get configure events
	C.zwlr_layer_surface_v1_add_listener(r.layerSurf, C.get_layer_surface_listener(), unsafe.Pointer(uintptr(r.registryHandle)))

	// Set surface properties
	C.zwlr_layer_surface_v1_set_anchor(r.layerSurf,
		C.ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP|
			C.ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM|
			C.ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT|
			C.ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT)

	C.zwlr_layer_surface_v1_set_exclusive_zone(r.layerSurf, -1)
	C.zwlr_layer_surface_v1_set_size(r.layerSurf, 0, 0)
	C.zwlr_layer_surface_v1_set_keyboard_interactivity(r.layerSurf, 0)
	C.zwlr_layer_surface_v1_set_margin(r.layerSurf, 0, 0, 0, 0)

	log.Debugf("r.surface = %p", r.surface)

	// Commit the surface to request the initial configure event
	C.wl_surface_commit(r.surface)

	// First roundtrip to ensure events are processed
	C.wl_display_roundtrip(r.display)

	// Wait for the surface to be configured
	log.Debug("Waiting for layer surface configure...")
	select {
	case <-r.configChan:
		log.Debugf("Layer surface configured: width=%d, height=%d", r.configWidth, r.configHeight)
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for layer surface configure")
	}

	return nil
}

func createWLEGLWindow(surface *C.struct_wl_surface, width, height int) *C.struct_wl_egl_window {
	eglWindow := C.wl_egl_window_create(surface, C.int(width), C.int(height))
	if eglWindow == nil {
		panic("failed to create wl_egl_window")
	}
	return eglWindow
}

func initEGL(dpy *C.struct_wl_display, eglWindow *C.struct_wl_egl_window) (C.EGLDisplay, C.EGLSurface, C.EGLContext) {
	eglDisplay := C.eglGetDisplay(C.EGLNativeDisplayType(dpy))
	if eglDisplay == 0 {
		panic("failed to get EGL display")
	}
	if C.eglInitialize(eglDisplay, nil, nil) == C.EGL_FALSE {
		panic("failed to initialize EGL")
	}

	var config C.EGLConfig
	var numConfigs C.EGLint
	attribs := []C.EGLint{
		C.EGL_SURFACE_TYPE, C.EGL_WINDOW_BIT,
		C.EGL_RED_SIZE, 8,
		C.EGL_GREEN_SIZE, 8,
		C.EGL_BLUE_SIZE, 8,
		C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_ES2_BIT,
		C.EGL_NONE,
	}
	if C.eglChooseConfig(eglDisplay, &attribs[0], &config, 1, &numConfigs) == C.EGL_FALSE {
		panic("failed to choose EGL config")
	}

	ctxAttribs := []C.EGLint{
		C.EGL_CONTEXT_CLIENT_VERSION, 2,
		C.EGL_NONE,
	}
	eglContext := C.eglCreateContext(eglDisplay, config, nil, &ctxAttribs[0])
	if eglContext == nil {
		panic("failed to create EGL context")
	}
	eglSurface := C.eglCreateWindowSurface(eglDisplay, config, C.EGLNativeWindowType(uintptr(unsafe.Pointer(eglWindow))), nil)
	if eglSurface == nil {
		panic("failed to create EGL surface")
	}

	if C.eglMakeCurrent(eglDisplay, eglSurface, eglSurface, eglContext) == C.EGL_FALSE {
		panic("failed to make EGL context current")
	}

	return eglDisplay, eglSurface, eglContext
}

// connectToDisplay handles all the display connection logic
func (r *WLRenderer) connectToDisplay() error {
	var err error

	// 1. Connect to Wayland display
	r.display, err = connectWaylandDisplay()
	if err != nil {
		return err
	}

	// 2. Set up registry
	if err := setupRegistry(r); err != nil {
		return err
	}

	// 3. Create surface
	if r.compositor == nil {
		return fmt.Errorf("failed to bind compositor interface")
	}
	r.surface = C.wl_compositor_create_surface(r.compositor)
	if r.surface == nil {
		return fmt.Errorf("failed to create surface")
	}

	// 4. Create layer surface
	if err := createLayerSurface(r); err != nil {
		return err
	}

	// 5. Set up EGL
	width, height := r.configWidth, r.configHeight
	if width == 0 {
		width = 1
	}
	if height == 0 {
		height = 1
	}

	log.Debugf("Creating EGL window with size %dx%d", width, height)
	eglWindow := createWLEGLWindow(r.surface, width, height)
	runtime.KeepAlive(eglWindow)
	r.eglDisplay, r.eglSurface, r.eglContext = initEGL(r.display, eglWindow)

	// Use the configured size for the renderer
	r.width = width
	r.height = height

	// 6. Set up shader program
	r.setupShaderProgram()

	return nil
}

// setupShaderProgram creates and configures the GL shader program
func (r *WLRenderer) setupShaderProgram() {
	posStr := C.CString("a_position")
	defer C.free(unsafe.Pointer(posStr))
	texCoordStr := C.CString("a_texCoord")
	defer C.free(unsafe.Pointer(texCoordStr))
	texStr := C.CString("u_texture")
	defer C.free(unsafe.Pointer(texStr))
	alphaStr := C.CString("u_alpha")
	defer C.free(unsafe.Pointer(alphaStr))

	// Create shader program
	prog := compileProgram(vertexShaderSrc, fragmentShaderSrc)
	r.shaderProgram = prog
	r.attribPos = C.GLint(C.glGetAttribLocation(prog, posStr))
	r.attribTex = C.GLint(C.glGetAttribLocation(prog, texCoordStr))
	r.uniformTex = C.GLint(C.glGetUniformLocation(prog, texStr))
	r.uniformAlpha = C.GLint(C.glGetUniformLocation(prog, alphaStr))
}

func (r *WLRenderer) SetImage(img image.Image) error {
	// Delete previous texture
	if r.currentTex.id != 0 {
		C.glDeleteTextures(1, &r.currentTex.id)
		r.currentTex = texture{}
	}

	// Convert image to RGBA if not already
	rgba, ok := img.(*image.RGBA)
	if !ok {
		tmp := image.NewRGBA(img.Bounds())
		draw.Draw(tmp, tmp.Bounds(), img, image.Point{}, draw.Src)
		rgba = tmp
	}

	// Generate texture
	var tex C.GLuint
	C.glGenTextures(1, &tex)
	C.glBindTexture(C.GL_TEXTURE_2D, tex)

	// Set texture parameters
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)

	// Upload pixels
	b := rgba.Bounds()
	width := b.Dx()
	height := b.Dy()
	C.glTexImage2D(
		C.GL_TEXTURE_2D,
		0,
		C.GL_RGBA,
		C.GLsizei(width),
		C.GLsizei(height),
		0,
		C.GL_RGBA,
		C.GL_UNSIGNED_BYTE,
		unsafe.Pointer(&rgba.Pix[0]),
	)

	r.currentTex.id = tex
	r.currentTex.width = width
	r.currentTex.height = height

	r.fading = false // this is a static set, no transition yet
	return nil
}

func (r *WLRenderer) Transition(next image.Image, duration time.Duration) error {
	// If no blackTex, create a black texture
	if r.blackTex.id == 0 {
		blackTex, err := r.createColorTexture(0, 0, 0)
		if err != nil {
			return fmt.Errorf("failed to create black transition texture: %w", err)
		}
		r.blackTex = blackTex
	}

	// If no currentTex, fade from black
	if r.currentTex.id == 0 {
		blackTex, err := r.createColorTexture(0, 0, 0)
		if err != nil {
			return fmt.Errorf("failed to create black fallback texture: %w", err)
		}
		r.currentTex = blackTex
	}

	// Delete old transitionTex if it exists
	if r.transitionTex.id != 0 {
		C.glDeleteTextures(1, &r.transitionTex.id)
		r.transitionTex = texture{}
	}

	// Upload next image to transitionTex
	tex, err := r.uploadImageToTexture(next)
	if err != nil {
		return fmt.Errorf("failed to upload transition image: %w", err)
	}
	r.transitionTex.id = tex
	r.transitionTex.width = next.Bounds().Dx()
	r.transitionTex.height = next.Bounds().Dy()
	r.start = time.Now()
	r.duration = duration
	r.fading = true

	// Frame loop
	for r.fading {
		if !r.IsDisplayRunning() {
			log.Info("Display connection lost, returning to main loop")
			return fmt.Errorf("display connection lost in transition")
		}

		if err := r.Render(); err != nil {
			return fmt.Errorf("failed to render during transition: %w", err)
		}
	}

	return nil
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

func (r *WLRenderer) Render() error {
	if r.layerSurf == nil {
		return fmt.Errorf("layer surface is nil")
	}

	// if t := C.wl_display_roundtrip(r.display); (t == -1) || (t == C.EGL_FALSE) {
	// 	return fmt.Errorf("failed to roundtrip display")
	// }

	// Calculate alpha value
	alpha := float32(1.0)
	if r.fading {
		elapsed := time.Since(r.start)
		progress := float32(elapsed.Seconds() / r.duration.Seconds())

		if progress >= 1.0 {
			// Transition complete
			progress = 1.0

			// Store the old texture for cleanup
			oldTexture := r.currentTex

			// Swap textures
			r.currentTex = r.transitionTex
			r.transitionTex = texture{}

			// Set fading to false to exit the transition loop
			r.fading = false

			// Render with the final texture before deletion
			alpha = 1.0

			// Delete old texture after swap
			if oldTexture.id != 0 {
				C.glDeleteTextures(1, &oldTexture.id)
			}
		} else {
			// Apply easing to alpha based on progress
			alpha = applyEasing(r.easingMode, progress)
		}
	}

	// Set up rendering
	C.glClear(C.GL_COLOR_BUFFER_BIT)
	C.glUseProgram(r.shaderProgram)

	if r.fading && r.currentTex.id != 0 {
		// When fading, first draw current texture at full opacity (no fading)
		C.glUniform1f(r.uniformAlpha, 1.0)
		C.glActiveTexture(C.GL_TEXTURE0)
		C.glBindTexture(C.GL_TEXTURE_2D, r.currentTex.id)
		C.glUniform1i(r.uniformTex, 0)

		// Draw the quad with the current texture
		drawTexturedQuad(r.width, r.height, r.scaleMode, r.attribPos, r.attribTex, C.GLint(r.currentTex.width), C.GLint(r.currentTex.height))

		// Enable blending for black texture and new texture
		C.glEnable(C.GL_BLEND)
		C.glBlendFunc(C.GL_SRC_ALPHA, C.GL_ONE_MINUS_SRC_ALPHA)

		// Draw black texture with increasing alpha
		if r.blackTex.id != 0 {
			C.glUniform1f(r.uniformAlpha, C.GLfloat(alpha))
			C.glActiveTexture(C.GL_TEXTURE0)
			C.glBindTexture(C.GL_TEXTURE_2D, r.blackTex.id)
			C.glUniform1i(r.uniformTex, 0)

			// Draw the black quad (use stretch mode to cover entire screen)
			drawTexturedQuad(r.width, r.height, types.ScalingModeStretch, r.attribPos, r.attribTex, C.GLint(r.width), C.GLint(r.height))
		}

		// Draw new texture with same alpha
		if r.transitionTex.id != 0 {
			C.glUniform1f(r.uniformAlpha, C.GLfloat(alpha))
			C.glActiveTexture(C.GL_TEXTURE0)
			C.glBindTexture(C.GL_TEXTURE_2D, r.transitionTex.id)
			C.glUniform1i(r.uniformTex, 0)

			// Draw the transition texture with the same scaling mode as current texture
			drawTexturedQuad(r.width, r.height, r.scaleMode, r.attribPos, r.attribTex, C.GLint(r.transitionTex.width), C.GLint(r.transitionTex.height))
		}

		C.glDisable(C.GL_BLEND)
	} else if r.currentTex.id != 0 {
		// Not fading, just draw the current texture at full opacity
		C.glUniform1f(r.uniformAlpha, 1.0)
		C.glActiveTexture(C.GL_TEXTURE0)
		C.glBindTexture(C.GL_TEXTURE_2D, r.currentTex.id)
		C.glUniform1i(r.uniformTex, 0)

		// Draw the quad with the current texture
		drawTexturedQuad(r.width, r.height, r.scaleMode, r.attribPos, r.attribTex, C.GLint(r.currentTex.width), C.GLint(r.currentTex.height))
	}

	// Ensure all rendering is complete before swap
	C.glFinish()

	if r.layerSurf == nil || r.eglSurface == nil || r.eglDisplay == 0 {
		log.Debug("Display disconnected")
		return fmt.Errorf("display disconnected")
	}

	if C.eglGetError() != C.EGL_SUCCESS {
		log.Debug("EGL error occurred")
		return fmt.Errorf("EGL error occurred")
	}

	if C.glGetError() != C.GL_NO_ERROR {
		log.Debug("OpenGL error occurred")
		return fmt.Errorf("OpenGL error occurred")
	}

	if t := C.wl_display_roundtrip(r.display); (t == -1) || (t == C.EGL_FALSE) {
		return fmt.Errorf("failed to roundtrip display")
	}

	// Swap buffers and wait for next frame
	C.eglSwapBuffers(r.eglDisplay, r.eglSurface)
	time.Sleep(time.Second / time.Duration(r.framerate))

	return nil
}

func (r *WLRenderer) GetSize() (int, int) {
	return r.width, r.height
}

func (r *WLRenderer) Cleanup() {
	// Delete GL textures
	if r.currentTex.id != 0 {
		C.glDeleteTextures(1, &r.currentTex.id)
		r.currentTex = texture{}
	}
	if r.transitionTex.id != 0 {
		C.glDeleteTextures(1, &r.transitionTex.id)
		r.transitionTex = texture{}
	}
	if r.blackTex.id != 0 {
		C.glDeleteTextures(1, &r.blackTex.id)
		r.blackTex = texture{}
	}

	// Delete shader program
	if r.shaderProgram != 0 {
		C.glDeleteProgram(r.shaderProgram)
		r.shaderProgram = 0
	}

	// Release EGL resources
	if r.eglDisplay != 0 {
		if r.eglSurface != nil {
			C.eglDestroySurface(r.eglDisplay, r.eglSurface)
			r.eglSurface = nil
		}
		if r.eglContext != nil {
			C.eglDestroyContext(r.eglDisplay, r.eglContext)
			r.eglContext = nil
		}
		C.eglTerminate(r.eglDisplay)
		r.eglDisplay = 0
	}

	if r.layerSurf != nil {
		C.zwlr_layer_surface_v1_destroy(r.layerSurf)
		r.layerSurf = nil
	}

	if r.surface != nil {
		C.wl_surface_destroy(r.surface)
		r.surface = nil
	}

	// Disconnect Wayland
	if r.display != nil {
		C.wl_display_disconnect(r.display)
		r.display = nil
	}

	// this should not throw a panic, but it does
	// r.registryHandle.Delete()
}

// TryReconnect attempts to reconnect to the Wayland display after a disconnect
func (r *WLRenderer) TryReconnect() error {
	// Clean up any existing resources
	r.Cleanup()

	// Initialize a new config channel
	r.configChan = make(chan struct{}, 1)

	// Reconnect to display
	return r.connectToDisplay()
}

func (r *WLRenderer) IsDisplayRunning() bool {
	if r.layerSurf == nil || r.eglSurface == nil || r.eglDisplay == 0 {
		log.Debug("Display disconnected, cleaning up...")
		r.Cleanup()
		return false
	}

	return true
}

func (r *WLRenderer) uploadImageToTexture(img image.Image) (C.GLuint, error) {
	rgba, ok := img.(*image.RGBA)
	if !ok {
		tmp := image.NewRGBA(img.Bounds())
		draw.Draw(tmp, tmp.Bounds(), img, image.Point{}, draw.Src)
		rgba = tmp
	}

	var tex C.GLuint
	C.glGenTextures(1, &tex)
	C.glBindTexture(C.GL_TEXTURE_2D, tex)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)

	b := rgba.Bounds()
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA,
		C.GLsizei(b.Dx()), C.GLsizei(b.Dy()),
		0, C.GL_RGBA, C.GL_UNSIGNED_BYTE,
		unsafe.Pointer(&rgba.Pix[0]),
	)

	return tex, nil
}

func (r *WLRenderer) createColorTexture(rVal, gVal, bVal uint8) (texture, error) {
	const w, h = 2, 2
	pix := make([]uint8, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = rVal
		pix[i+1] = gVal
		pix[i+2] = bVal
		pix[i+3] = 255
	}

	var tex C.GLuint
	C.glGenTextures(1, &tex)
	C.glBindTexture(C.GL_TEXTURE_2D, tex)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)

	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA,
		w, h, 0, C.GL_RGBA, C.GL_UNSIGNED_BYTE,
		unsafe.Pointer(&pix[0]),
	)

	return texture{tex, w, h}, nil
}

const vertexShaderSrc = `
    attribute vec2 a_position;
    attribute vec2 a_texCoord;
    varying vec2 v_texCoord;

    void main() {
        gl_Position = vec4(a_position, 0.0, 1.0);
        v_texCoord = a_texCoord;
    }
`

const fragmentShaderSrc = `
    precision mediump float;
    varying vec2 v_texCoord;
    uniform sampler2D u_texture;
    uniform float u_alpha;

    void main() {
        vec4 texColor = texture2D(u_texture, v_texCoord);
        gl_FragColor = vec4(texColor.rgb, texColor.a * u_alpha);
    }
`

func compileShader(src string, shaderType C.GLenum) C.GLuint {
	csrc := C.CString(src)
	defer C.free(unsafe.Pointer(csrc))

	shader := C.glCreateShader(shaderType)
	C.glShaderSource(shader, 1, &csrc, nil)
	C.glCompileShader(shader)

	var status C.GLint
	C.glGetShaderiv(shader, C.GL_COMPILE_STATUS, &status)
	if status == C.GL_FALSE {
		var logLen C.GLint
		C.glGetShaderiv(shader, C.GL_INFO_LOG_LENGTH, &logLen)
		log := make([]byte, int(logLen))
		C.glGetShaderInfoLog(shader, logLen, nil, (*C.GLchar)(unsafe.Pointer(&log[0])))
		panic(fmt.Sprintf("shader compile error: %s", log))
	}
	return shader
}

func compileProgram(vsrc, fsrc string) C.GLuint {
	vs := compileShader(vsrc, C.GL_VERTEX_SHADER)
	fs := compileShader(fsrc, C.GL_FRAGMENT_SHADER)

	prog := C.glCreateProgram()
	C.glAttachShader(prog, vs)
	C.glAttachShader(prog, fs)
	C.glLinkProgram(prog)

	var status C.GLint
	C.glGetProgramiv(prog, C.GL_LINK_STATUS, &status)
	if status == C.GL_FALSE {
		var logLen C.GLint
		C.glGetProgramiv(prog, C.GL_INFO_LOG_LENGTH, &logLen)
		log := make([]byte, int(logLen))
		C.glGetProgramInfoLog(prog, logLen, nil, (*C.GLchar)(unsafe.Pointer(&log[0])))
		panic(fmt.Sprintf("program link error: %s", log))
	}

	C.glDeleteShader(vs)
	C.glDeleteShader(fs)

	return prog
}

func drawTexturedQuad(screenWidth, screenHeight int, scaleMode types.ScalingMode, attribPos, attribTex C.GLint, texWidth, texHeight C.GLint) {
	// Default to full screen quad
	var x1, y1, x2, y2 float32 = -1.0, -1.0, 1.0, 1.0

	// Texture coordinates (always use full texture)
	var u1, v1, u2, v2 float32 = 0.0, 1.0, 1.0, 0.0

	// Calculate aspect ratios
	screenAspect := float32(screenWidth) / float32(screenHeight)
	textureAspect := float32(texWidth) / float32(texHeight)

	// Adjust coordinates based on scaling mode
	switch scaleMode {
	case types.ScalingModeStretch:
		// Use full screen coordinates (already set)

	case types.ScalingModeFitHorizontal:
		// Keep width at 100%, adjust height to maintain texture aspect ratio
		scaledHeight := 1.0 / (textureAspect / screenAspect)
		y1 = -scaledHeight
		y2 = scaledHeight

	case types.ScalingModeFitVertical:
		// Keep height at 100%, adjust width to maintain texture aspect ratio
		scaledWidth := textureAspect / screenAspect
		x1 = -scaledWidth
		x2 = scaledWidth

	case types.ScalingModeCenter:
		fallthrough
	default:
		// "Fill" mode - use the smaller scaling factor to ensure no cropping
		if textureAspect > screenAspect {
			// Texture is wider than screen
			// Fit to width and adjust height
			scaledHeight := screenAspect / textureAspect
			y1 = -scaledHeight
			y2 = scaledHeight
		} else {
			// Texture is taller than screen
			// Fit to height and adjust width
			scaledWidth := textureAspect / screenAspect
			x1 = -scaledWidth
			x2 = scaledWidth
		}
	}

	// Interleaved vertex data: [x, y, u, v]
	vertices := []float32{
		x1, y1, u1, v1, // Bottom left
		x2, y1, u2, v1, // Bottom right
		x1, y2, u1, v2, // Top left
		x2, y1, u2, v1, // Bottom right
		x2, y2, u2, v2, // Top right
		x1, y2, u1, v2, // Top left
	}

	// Set up vertex attribute pointers
	C.glEnableVertexAttribArray(C.GLuint(attribPos))
	C.glEnableVertexAttribArray(C.GLuint(attribTex))

	// Position attribute (first 2 floats)
	C.glVertexAttribPointer(
		C.GLuint(attribPos),
		2,                            // size (x,y)
		C.GL_FLOAT,                   // type
		C.GL_FALSE,                   // normalized
		4*4,                          // stride (4 floats * 4 bytes)
		unsafe.Pointer(&vertices[0]), // pointer to first position
	)

	// Texture coordinate attribute (second 2 floats)
	C.glVertexAttribPointer(
		C.GLuint(attribTex),
		2,          // size (u,v)
		C.GL_FLOAT, // type
		C.GL_FALSE, // normalized
		4*4,        // stride (4 floats * 4 bytes)
		unsafe.Pointer(uintptr(unsafe.Pointer(&vertices[0]))+8), // pointer to first texcoord
	)

	// Draw the triangles
	C.glDrawArrays(C.GL_TRIANGLES, 0, 6)

	// Disable attribute arrays
	C.glDisableVertexAttribArray(C.GLuint(attribPos))
	C.glDisableVertexAttribArray(C.GLuint(attribTex))
}
