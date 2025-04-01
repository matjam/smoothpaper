package render

import (
	"image"
	"time"
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
	IsDisplayRunning() bool
}
