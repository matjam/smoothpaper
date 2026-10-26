package types

type ScalingMode string

const (
	ScalingModeCenter        ScalingMode = "center"
	ScalingModeStretch       ScalingMode = "stretched"
	ScalingModeFitHorizontal ScalingMode = "horizontal"
	ScalingModeFitVertical   ScalingMode = "vertical"
)

func (s ScalingMode) Valid() bool {
	switch s {
	case ScalingModeCenter,
		ScalingModeStretch,
		ScalingModeFitHorizontal,
		ScalingModeFitVertical:
		return true
	default:
		return false
	}
}

type EasingMode string

const (
	EasingLinear    EasingMode = "linear"
	EasingEaseIn    EasingMode = "ease-in"
	EasingEaseOut   EasingMode = "ease-out"
	EasingEaseInOut EasingMode = "ease-in-out"
)

func (e EasingMode) Valid() bool {
	switch e {
	case EasingLinear,
		EasingEaseIn,
		EasingEaseOut,
		EasingEaseInOut:
		return true
	default:
		return false
	}
}
