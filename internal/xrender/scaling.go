package xrender

import (
	"image"

	"golang.org/x/image/draw"
)

// FitMode defines how to scale the image
type FitMode string

const (
	FitCenter  FitMode = "center"
	FitStretch FitMode = "stretched"
	FitWidth   FitMode = "horizontal"
	FitHeight  FitMode = "vertical"
)

// ScaleImage scales the image according to the fit mode and target size
func ScaleImage(img image.Image, targetW, targetH int, mode FitMode) *image.RGBA {
	var dstRect image.Rectangle
	srcW := img.Bounds().Dx()
	srcH := img.Bounds().Dy()

	switch mode {
	case FitStretch:
		dstRect = image.Rect(0, 0, targetW, targetH)
	case FitWidth:
		scale := float64(targetW) / float64(srcW)
		h := int(float64(srcH) * scale)
		dstRect = image.Rect(0, 0, targetW, h)
	case FitHeight:
		scale := float64(targetH) / float64(srcH)
		w := int(float64(srcW) * scale)
		dstRect = image.Rect(0, 0, w, targetH)
	case FitCenter:
		fallthrough
	default:
		// Keep original size, center inside target canvas
		x := (targetW - srcW) / 2
		y := (targetH - srcH) / 2
		dstRect = image.Rect(x, y, x+srcW, y+srcH)
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	d := draw.CatmullRom
	d.Scale(dst, dstRect, img, img.Bounds(), draw.Over, nil)
	return dst
}
