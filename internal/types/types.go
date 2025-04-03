package types

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
