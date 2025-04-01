package xrender

import "C"

import (
	"fmt"
	"image"
	"time"
)

type WallpaperRenderer struct {
	Desktop  *DesktopWindow
	Renderer *Renderer
	Scale    FitMode
}

func NewWallpaperRenderer(scale FitMode) (*WallpaperRenderer, error) {
	desktop := CreateDesktopWindow()
	if desktop == nil {
		return nil, fmt.Errorf("failed to create desktop window")
	}

	renderer, err := NewRenderer(desktop.Display, desktop.Window, desktop.Width, desktop.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}

	return &WallpaperRenderer{
		Desktop:  desktop,
		Renderer: renderer,
		Scale:    scale,
	}, nil
}

func (wr *WallpaperRenderer) Transition(current, next image.Image, duration time.Duration, easing EasingMode) error {
	scaledA := ScaleImage(current, wr.Desktop.Width, wr.Desktop.Height, wr.Scale)
	scaledB := ScaleImage(next, wr.Desktop.Width, wr.Desktop.Height, wr.Scale)

	if err := wr.Renderer.LoadTextures(scaledA, scaledB); err != nil {
		return fmt.Errorf("failed to load textures: %w", err)
	}

	wr.Renderer.RenderFadeWithEasing(duration, easing)

	root := &RootWindow{
		Display: wr.Desktop.Display,
		Window:  wr.Desktop.Root,
		Screen:  0, // Safe to keep as 0 since you already used CreateDesktopWindow()
		Width:   C.int(wr.Desktop.Width),
		Height:  C.int(wr.Desktop.Height),
	}

	if err := root.SetImage(scaledB); err != nil {
		return fmt.Errorf("failed to persist image: %w", err)
	}

	realRoot, err := GetRootWindow()
	if err != nil {
		return fmt.Errorf("failed to get real root window: %w", err)
	}

	if err := realRoot.SetImage(scaledB); err != nil {
		return fmt.Errorf("failed to set image on real root: %w", err)
	}

	return nil
}

func (wr *WallpaperRenderer) Cleanup() {
	if wr.Renderer != nil {
		wr.Renderer.Cleanup()
	}
}
