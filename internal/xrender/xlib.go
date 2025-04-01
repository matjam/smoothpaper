package xrender

/*
#cgo LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <malloc.h>
#include "xlib.h"
*/
import "C"

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/charmbracelet/log"
)

// RootWindow holds the Xlib display and root window identifiers.
type RootWindow struct {
	Display *C.Display
	Window  C.Window
	Screen  C.int
	Width   C.int
	Height  C.int
}

// GetRootWindow opens the default display and returns the root window.
func GetRootWindow() (*RootWindow, error) {
	dpy := C.XOpenDisplay(nil)
	if dpy == nil {
		return nil, fmt.Errorf("unable to open display")
	}
	screen := C.XDefaultScreen(dpy)
	root := C.XRootWindow(dpy, screen)
	rw := &RootWindow{Display: dpy, Window: root, Screen: screen}

	rw.Width = C.screen_width(dpy)
	rw.Height = C.screen_height(dpy)

	if rw.Width == 0 || rw.Height == 0 {
		C.XCloseDisplay(dpy)
		return nil, fmt.Errorf("unable to get screen dimensions")
	}

	return rw, nil
}

// SetImage takes a Go image.Image, converts it to an XImage, copies it into a Pixmap,
// and then sets that pixmap as the root window’s background.
func (rw *RootWindow) SetImage(img image.Image) error {
	// Convert the image to RGBA if needed.
	var rgba *image.RGBA
	if r, ok := img.(*image.RGBA); ok {
		rgba = r
	} else {
		bounds := img.Bounds()
		rgba = image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	}

	w := rgba.Bounds().Dx()
	h := rgba.Bounds().Dy()

	visual := C.XDefaultVisual(rw.Display, rw.Screen)
	depth := C.XDefaultDepth(rw.Display, rw.Screen)

	// Determine bytes per pixel (bpp) from depth.
	var bpp int
	if int(depth) == 24 {
		bpp = 4
	} else {
		bpp = (int(depth) + 7) / 8
	}
	rawRowSize := w * bpp
	// Rows must be padded to a 4-byte boundary.
	paddedRowSize := (rawRowSize + 3) &^ 3

	// Create a buffer to hold pixel data.
	data := make([]byte, h*paddedRowSize)
	for y := 0; y < h; y++ {
		dstRowStart := y * paddedRowSize
		for x := 0; x < w; x++ {
			srcIndex := y*rgba.Stride + x*4
			dstIndex := dstRowStart + x*bpp

			data[dstIndex] = rgba.Pix[srcIndex+2]   // B
			data[dstIndex+1] = rgba.Pix[srcIndex+1] // G
			data[dstIndex+2] = rgba.Pix[srcIndex]   // R

			if bpp == 4 {
				data[dstIndex+3] = 0xFF // Opaque alpha
			}
		}
	}

	// Allocate C memory for the pixel data.
	cData := C.CBytes(data)
	defer C.free(cData)

	log.Infof("width=%d, height=%d, depth=%d, bpp=%d, paddedRowSize=%d",
		w, h, int(depth), bpp, paddedRowSize)

	ximage := C.XCreateImage(
		rw.Display,
		visual,
		C.uint(depth),
		C.ZPixmap,
		0, // offset
		(*C.char)(cData),
		C.uint(w),
		C.uint(h),
		32, // bitmap pad
		C.int(paddedRowSize),
	)
	C.XSync(rw.Display, C.False)

	if ximage == nil {
		return fmt.Errorf("failed to create XImage")
	}

	// Create a pixmap to hold the image.
	pixmap := C.XCreatePixmap(
		rw.Display,
		rw.Window,
		C.uint(w),
		C.uint(h),
		C.uint(depth),
	)

	// Get the default graphics context.
	gc := C.XDefaultGC(rw.Display, rw.Screen)

	// Copy the XImage onto the pixmap.
	C.XPutImage(
		rw.Display,
		pixmap,
		gc,
		ximage,
		0, 0, // src x, y
		0, 0, // dst x, y
		C.uint(w), C.uint(h), // width, height
	)

	// Set the root window's background to the pixmap.
	C.XSetWindowBackgroundPixmap(rw.Display, rw.Window, pixmap)
	C.XClearWindow(rw.Display, rw.Window)

	// Detach the data pointer so XDestroyImage doesn't free it as it was
	// allocated in Go.
	ximage.data = nil

	// Free the XImage.
	C.destroyXImage(ximage)

	// Flush the changes
	C.XFlush(rw.Display)

	return nil
}

type DesktopWindow struct {
	Display *C.Display
	Root    C.Window
	Desktop C.Window
	Window  C.Window
	Width   int
	Height  int
}

func CreateDesktopWindow() *DesktopWindow {
	cwin := C.create_desktop_window()
	if cwin.display == nil || cwin.window == 0 {
		return nil
	}
	return &DesktopWindow{
		Display: cwin.display,
		Root:    cwin.root,
		Desktop: cwin.desktop,
		Window:  cwin.window,
		Width:   int(cwin.width),
		Height:  int(cwin.height),
	}
}
