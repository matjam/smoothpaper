package wlrenderer

/*
#cgo LDFLAGS: -lwayland-client -lwayland-egl -lEGL -lGLESv2
#include "wlrenderer.h"
// Forward declare wl_output_interface so we can bind outputs without including extra headers here
extern const struct wl_interface wl_output_interface;
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
	compositor *C.struct_wl_compositor
	// protocol version negotiated when binding wl_compositor
	compositorVersion int

	// EGL (shared display/context; per-output surfaces)
	eglDisplay C.EGLDisplay
	eglContext C.EGLContext
	eglSurface C.EGLSurface // kept for backward compatibility, unused in multi-output path
	eglConfig  C.EGLConfig

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

	configured   bool
	configWidth  int
	configHeight int
	configChan   chan struct{}

	// Output change detection
	outputsDirty bool
	initialized  bool

	// Per-output
	outputs map[uint32]*outputSurface
}

type outputSurface struct {
	id         uint32
	output     *C.struct_wl_output
	surface    *C.struct_wl_surface
	layerSurf  *C.struct_zwlr_layer_surface_v1
	eglWindow  *C.struct_wl_egl_window
	eglSurface C.EGLSurface
	configured bool
	width      int
	height     int
	scale      int
	configChan chan struct{}
}

// removed per-output helpers (not used in single-surface reconnect strategy)

func NewRenderer(scale types.ScalingMode, easing types.EasingMode, framerate int) (*WLRenderer, error) {
	runtime.LockOSThread() // Required: OpenGL contexts must be accessed from a single OS thread

	r := &WLRenderer{
		scaleMode:  scale,
		easingMode: easing,
		framerate:  framerate,
		configChan: make(chan struct{}, 1),
		outputs:    make(map[uint32]*outputSurface),
	}

	if err := r.connectToDisplay(); err != nil {
		return nil, err
	}

	return r, nil
}

//export goHandleGlobal
func goHandleGlobal(handle C.uintptr_t, registry *C.struct_wl_registry, name C.uint32_t, iface *C.char, version C.uint32_t) {
	h := cgo.Handle(uintptr(handle))
	r := h.Value().(*WLRenderer)
	if r == nil {
		log.Error("goHandleGlobal: nil renderer")
		return
	}

	goIface := C.GoString(iface)

	switch goIface {
	case "zwlr_layer_shell_v1":
		// layer-shell v1 is sufficient for our needs
		r.layerShell = (*C.struct_zwlr_layer_shell_v1)(C.wl_registry_bind(registry, name, &C.zwlr_layer_shell_v1_interface, 1))
		log.Debug("bound zwlr_layer_shell_v1")
	case "wl_compositor":
		// Need wl_surface.set_buffer_scale which is available since compositor v3
		want := C.uint32_t(4)
		if version < want {
			want = version
		}
		r.compositor = (*C.struct_wl_compositor)(C.wl_registry_bind(registry, name, &C.wl_compositor_interface, want))
		r.compositorVersion = int(want)
		log.Debug("bound wl_compositor")
	case "wl_output":
		// Bind and track this output (we need scale event which exists since v2)
		want := C.uint32_t(3)
		if version < want {
			want = version
		}
		wlOut := (*C.struct_wl_output)(C.wl_registry_bind(registry, name, &C.wl_output_interface, want))
		id := uint32(name)
		if _, exists := r.outputs[id]; !exists {
			out := &outputSurface{id: id, output: wlOut, configChan: make(chan struct{}, 1), scale: 1}
			r.outputs[id] = out
			C.wl_output_add_listener(wlOut, C.get_output_listener(), unsafe.Pointer(uintptr(r.registryHandle)))
			log.Debugf("bound wl_output id=%d", id)
			if r.initialized {
				r.outputsDirty = true
			}
		}
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
	id := uint32(name)
	if out, ok := r.outputs[id]; ok {
		// Destroy and drop this output
		if out.eglSurface != nil && r.eglDisplay != 0 {
			C.eglDestroySurface(r.eglDisplay, out.eglSurface)
			out.eglSurface = nil
		}
		if out.layerSurf != nil {
			C.zwlr_layer_surface_v1_destroy(out.layerSurf)
			out.layerSurf = nil
		}
		if out.surface != nil {
			C.wl_surface_destroy(out.surface)
			out.surface = nil
		}
		delete(r.outputs, id)
	}
	if r.initialized {
		r.outputsDirty = true
	}
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

	// If multi-output: find the matching output and store configuration
	for _, out := range r.outputs {
		if out.layerSurf == surface {
			out.width = int(width)
			out.height = int(height)
			out.configured = true
			// Resize EGL window to buffer size (logical * scale) if already created
			if out.eglWindow != nil {
				bufW := out.width
				bufH := out.height
				if out.scale > 1 {
					bufW *= out.scale
					bufH *= out.scale
				}
				C.wl_egl_window_resize(out.eglWindow, C.int(bufW), C.int(bufH), 0, 0)
			}
			select {
			case out.configChan <- struct{}{}:
			default:
			}
			return
		}
	}

	// Fallback to single-surface path
	r.configWidth = int(width)
	r.configHeight = int(height)
	r.configured = true
	select {
	case r.configChan <- struct{}{}:
	default:
	}
}

//export goHandleOutputScale
func goHandleOutputScale(handle C.uintptr_t, output *C.struct_wl_output, factor C.int32_t) {
	h := cgo.Handle(uintptr(handle))
	r := h.Value().(*WLRenderer)
	if r == nil {
		log.Error("goHandleOutputScale: nil renderer")
		return
	}
	for _, out := range r.outputs {
		if out.output == output {
			newScale := int(factor)
			if newScale <= 0 {
				newScale = 1
			}
			if out.scale != newScale {
				out.scale = newScale
				if out.surface != nil {
					C.wl_surface_set_buffer_scale(out.surface, C.int(out.scale))
				}
				if out.eglWindow != nil {
					bufW := out.width
					bufH := out.height
					if out.scale > 1 {
						bufW *= out.scale
						bufH *= out.scale
					}
					C.wl_egl_window_resize(out.eglWindow, C.int(bufW), C.int(bufH), 0, 0)
				}
			}
			break
		}
	}
}

//export goHandleLayerSurfaceClosed
func goHandleLayerSurfaceClosed(handle C.uintptr_t, surf *C.struct_zwlr_layer_surface_v1) {
	log.Debugf("goHandleLayerSurfaceClosed: handle=%d", handle)

	h := cgo.Handle(uintptr(handle))
	r := h.Value().(*WLRenderer)
	if r == nil {
		log.Error("goHandleLayerSurfaceClosed: nil renderer")
		return
	}

	log.Debug("Layer surface closed")
	// If this belongs to any output, destroy just that output
	for id, out := range r.outputs {
		if out.layerSurf == surf {
			if out.eglSurface != nil && r.eglDisplay != 0 {
				C.eglDestroySurface(r.eglDisplay, out.eglSurface)
				out.eglSurface = nil
			}
			if out.layerSurf != nil {
				C.zwlr_layer_surface_v1_destroy(out.layerSurf)
				out.layerSurf = nil
			}
			if out.surface != nil {
				C.wl_surface_destroy(out.surface)
				out.surface = nil
			}
			delete(r.outputs, id)
			return
		}
	}

	// Otherwise, fallback to single-surface cleanup
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

// Create per-output layer surface and block until configured
func createLayerSurfaceForOutput(r *WLRenderer, out *outputSurface) error {
	if r.layerShell == nil || out.surface == nil {
		return fmt.Errorf("required layer-shell or surface not available")
	}

	if out.configChan == nil {
		out.configChan = make(chan struct{}, 1)
	}

	layer := C.uint32_t(C.ZWLR_LAYER_SHELL_V1_LAYER_BACKGROUND)
	namespace := C.CString("smoothpaper")
	defer C.free(unsafe.Pointer(namespace))

	out.layerSurf = C.zwlr_layer_shell_v1_get_layer_surface(
		r.layerShell, out.surface, out.output, layer, namespace,
	)
	if out.layerSurf == nil {
		return fmt.Errorf("failed to create layer surface")
	}

	C.zwlr_layer_surface_v1_add_listener(out.layerSurf, C.get_layer_surface_listener(), unsafe.Pointer(uintptr(r.registryHandle)))

	C.zwlr_layer_surface_v1_set_anchor(out.layerSurf,
		C.ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP|
			C.ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM|
			C.ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT|
			C.ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT)

	C.zwlr_layer_surface_v1_set_exclusive_zone(out.layerSurf, -1)
	C.zwlr_layer_surface_v1_set_size(out.layerSurf, 0, 0)
	C.zwlr_layer_surface_v1_set_keyboard_interactivity(out.layerSurf, 0)
	C.zwlr_layer_surface_v1_set_margin(out.layerSurf, 0, 0, 0, 0)

	// Respect scale if provided and supported (wl_compositor v3+)
	if out.scale <= 0 {
		out.scale = 1
	}
	if r.compositorVersion >= 3 {
		C.wl_surface_set_buffer_scale(out.surface, C.int(out.scale))
	}
	C.wl_surface_commit(out.surface)
	C.wl_display_roundtrip(r.display)

	select {
	case <-out.configChan:
		log.Debugf("Output %d configured: %dx%d", out.id, out.width, out.height)
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for output configure")
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

// Initialize EGL display and context without creating a surface yet
func initEGLNoSurface(dpy *C.struct_wl_display) (C.EGLDisplay, C.EGLContext, C.EGLConfig) {
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

	return eglDisplay, eglContext, config
}

func createEGLSurface(eglDisplay C.EGLDisplay, config C.EGLConfig, eglWindow *C.struct_wl_egl_window) C.EGLSurface {
	eglSurface := C.eglCreateWindowSurface(eglDisplay, config, C.EGLNativeWindowType(uintptr(unsafe.Pointer(eglWindow))), nil)
	if eglSurface == nil {
		panic("failed to create EGL surface")
	}
	return eglSurface
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

	// 3. Initialize shared EGL context without surfaces
	r.eglDisplay, r.eglContext, r.eglConfig = initEGLNoSurface(r.display)

	// 4. Create a wl_surface/layer surface per output
	for _, out := range r.outputs {
		out.surface = C.wl_compositor_create_surface(r.compositor)
		if out.surface == nil {
			return fmt.Errorf("failed to create wl_surface for output")
		}
		if err := createLayerSurfaceForOutput(r, out); err != nil {
			return err
		}
		w := out.width
		h := out.height
		if w == 0 {
			w = 1
		}
		if h == 0 {
			h = 1
		}
		if out.scale > 1 {
			w *= out.scale
			h *= out.scale
		}
		out.eglWindow = createWLEGLWindow(out.surface, w, h)
		out.eglSurface = createEGLSurface(r.eglDisplay, r.eglConfig, out.eglWindow)
		// Make current once to initialize GL state
		if C.eglMakeCurrent(r.eglDisplay, out.eglSurface, out.eglSurface, r.eglContext) == C.EGL_FALSE {
			return fmt.Errorf("failed to make EGL context current for output")
		}
		if r.shaderProgram == 0 {
			r.setupShaderProgram()
		}
	}

	// Mark initialization done so future output add/remove triggers reconfigure
	r.initialized = true

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
	if r.outputsDirty {
		// Rebuild surfaces matching current outputs
		r.outputsDirty = false
		// Destroy any per-output resources for outputs that lost their layer surface
		// but keep the output entry so we can recreate surfaces
		for _, out := range r.outputs {
			if out.layerSurf == nil {
				if out.eglSurface != nil && r.eglDisplay != 0 {
					C.eglDestroySurface(r.eglDisplay, out.eglSurface)
					out.eglSurface = nil
				}
				if out.surface != nil {
					C.wl_surface_destroy(out.surface)
					out.surface = nil
				}
			}
		}
		// Recreate for all known outputs missing surfaces
		for _, out := range r.outputs {
			if out.surface == nil {
				out.surface = C.wl_compositor_create_surface(r.compositor)
				if out.surface == nil {
					return fmt.Errorf("failed to create wl_surface for output")
				}
				if err := createLayerSurfaceForOutput(r, out); err != nil {
					return err
				}
				w := out.width
				h := out.height
				if w == 0 {
					w = 1
				}
				if h == 0 {
					h = 1
				}
				if out.scale > 1 {
					w *= out.scale
					h *= out.scale
				}
				out.eglWindow = createWLEGLWindow(out.surface, w, h)
				out.eglSurface = createEGLSurface(r.eglDisplay, r.eglConfig, out.eglWindow)
			}
		}
	}

	// Calculate alpha
	alpha := float32(1.0)
	if r.fading {
		elapsed := time.Since(r.start)
		progress := float32(elapsed.Seconds() / r.duration.Seconds())
		if progress >= 1.0 {
			progress = 1.0
			oldTexture := r.currentTex
			r.currentTex = r.transitionTex
			r.transitionTex = texture{}
			r.fading = false
			alpha = 1.0
			if oldTexture.id != 0 {
				C.glDeleteTextures(1, &oldTexture.id)
			}
		} else {
			alpha = applyEasing(r.easingMode, progress)
		}
	}

	anyRendered := false
	for _, out := range r.outputs {
		if !out.configured || out.eglSurface == nil {
			continue
		}
		anyRendered = true
		if C.eglMakeCurrent(r.eglDisplay, out.eglSurface, out.eglSurface, r.eglContext) == C.EGL_FALSE {
			return fmt.Errorf("failed to make EGL context current for output")
		}
		if r.shaderProgram == 0 {
			r.setupShaderProgram()
		}
		// Ensure viewport matches buffer size
		C.glViewport(0, 0, C.GLsizei(out.width*out.scale), C.GLsizei(out.height*out.scale))
		C.glClear(C.GL_COLOR_BUFFER_BIT)
		C.glUseProgram(r.shaderProgram)

		if r.fading && r.currentTex.id != 0 {
			C.glUniform1f(r.uniformAlpha, 1.0)
			C.glActiveTexture(C.GL_TEXTURE0)
			C.glBindTexture(C.GL_TEXTURE_2D, r.currentTex.id)
			C.glUniform1i(r.uniformTex, 0)
			drawTexturedQuad(out.width, out.height, r.scaleMode, r.attribPos, r.attribTex, C.GLint(r.currentTex.width), C.GLint(r.currentTex.height))
			C.glEnable(C.GL_BLEND)
			C.glBlendFunc(C.GL_SRC_ALPHA, C.GL_ONE_MINUS_SRC_ALPHA)
			if r.blackTex.id != 0 {
				C.glUniform1f(r.uniformAlpha, C.GLfloat(alpha))
				C.glActiveTexture(C.GL_TEXTURE0)
				C.glBindTexture(C.GL_TEXTURE_2D, r.blackTex.id)
				C.glUniform1i(r.uniformTex, 0)
				drawTexturedQuad(out.width, out.height, types.ScalingModeStretch, r.attribPos, r.attribTex, C.GLint(out.width), C.GLint(out.height))
			}
			if r.transitionTex.id != 0 {
				C.glUniform1f(r.uniformAlpha, C.GLfloat(alpha))
				C.glActiveTexture(C.GL_TEXTURE0)
				C.glBindTexture(C.GL_TEXTURE_2D, r.transitionTex.id)
				C.glUniform1i(r.uniformTex, 0)
				drawTexturedQuad(out.width, out.height, r.scaleMode, r.attribPos, r.attribTex, C.GLint(r.transitionTex.width), C.GLint(r.transitionTex.height))
			}
			C.glDisable(C.GL_BLEND)
		} else if r.currentTex.id != 0 {
			C.glUniform1f(r.uniformAlpha, 1.0)
			C.glActiveTexture(C.GL_TEXTURE0)
			C.glBindTexture(C.GL_TEXTURE_2D, r.currentTex.id)
			C.glUniform1i(r.uniformTex, 0)
			drawTexturedQuad(out.width, out.height, r.scaleMode, r.attribPos, r.attribTex, C.GLint(r.currentTex.width), C.GLint(r.currentTex.height))
		}

		C.glFinish()
		if C.eglGetError() != C.EGL_SUCCESS {
			return fmt.Errorf("EGL error occurred")
		}
		if C.glGetError() != C.GL_NO_ERROR {
			return fmt.Errorf("OpenGL error occurred")
		}
		C.eglSwapBuffers(r.eglDisplay, out.eglSurface)
	}

	if !anyRendered {
		// Fallback: if no outputs tracked (rare), keep old behavior checks
		if r.layerSurf == nil || r.eglSurface == nil || r.eglDisplay == 0 {
			return fmt.Errorf("display disconnected")
		}
		if t := C.wl_display_roundtrip(r.display); (t == -1) || (t == C.EGL_FALSE) {
			return fmt.Errorf("failed to roundtrip display")
		}
		C.eglSwapBuffers(r.eglDisplay, r.eglSurface)
	} else {
		// Process wayland events once per frame
		if t := C.wl_display_roundtrip(r.display); (t == -1) || (t == C.EGL_FALSE) {
			return fmt.Errorf("failed to roundtrip display")
		}
	}

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

	// Destroy per-output resources
	for id, out := range r.outputs {
		if out.eglSurface != nil && r.eglDisplay != 0 {
			C.eglDestroySurface(r.eglDisplay, out.eglSurface)
			out.eglSurface = nil
		}
		if out.layerSurf != nil {
			C.zwlr_layer_surface_v1_destroy(out.layerSurf)
			out.layerSurf = nil
		}
		if out.surface != nil {
			C.wl_surface_destroy(out.surface)
			out.surface = nil
		}
		delete(r.outputs, id)
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

	// Reconnect to display
	return r.connectToDisplay()
}

func (r *WLRenderer) IsDisplayRunning() bool {
	if r.eglDisplay == 0 || r.eglContext == nil {
		r.Cleanup()
		return false
	}
	// Consider running if at least one output is configured
	for _, out := range r.outputs {
		if out.configured && out.eglSurface != nil {
			return true
		}
	}
	// Fallback to single-surface check
	if r.layerSurf != nil && r.eglSurface != nil {
		return true
	}
	r.Cleanup()
	return false
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
