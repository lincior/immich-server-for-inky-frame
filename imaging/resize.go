// Package imaging provides image-resizing helpers tailored for the
// Pimoroni Inky Frame 7.3-inch ePaper display (800 × 480 pixels).
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"math"

	xdraw "golang.org/x/image/draw"
)

const (
	// InkyFrameWidth is the horizontal resolution of the 7.3-inch Inky Frame.
	InkyFrameWidth = 800
	// InkyFrameHeight is the vertical resolution of the 7.3-inch Inky Frame.
	InkyFrameHeight = 480
)

// ResizeForInkyFrame decodes imgData (JPEG or PNG), scales it to fit within the
// 800 × 480 Inky Frame resolution while preserving the aspect ratio, centres
// it on a black canvas of exactly 800 × 480, and returns the result as a JPEG.
func ResizeForInkyFrame(imgData []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	scaledW, scaledH := fitDimensions(
		src.Bounds().Dx(), src.Bounds().Dy(),
		InkyFrameWidth, InkyFrameHeight,
	)

	// Scale the source image using the high-quality CatmullRom kernel.
	scaledRect := image.Rect(0, 0, scaledW, scaledH)
	scaled := image.NewRGBA(scaledRect)
	xdraw.CatmullRom.Scale(scaled, scaledRect, src, src.Bounds(), xdraw.Over, nil)

	// Compose onto a black 800 × 480 canvas (letterbox / pillarbox).
	canvas := image.NewRGBA(image.Rect(0, 0, InkyFrameWidth, InkyFrameHeight))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)

	offsetX := (InkyFrameWidth - scaledW) / 2
	offsetY := (InkyFrameHeight - scaledH) / 2
	destRect := image.Rect(offsetX, offsetY, offsetX+scaledW, offsetY+scaledH)
	draw.Draw(canvas, destRect, scaled, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), nil
}

// fitDimensions returns the largest integer dimensions that fit srcW × srcH
// inside maxW × maxH while maintaining the original aspect ratio.
func fitDimensions(srcW, srcH, maxW, maxH int) (int, int) {
	if srcW <= 0 || srcH <= 0 {
		return maxW, maxH
	}
	ratio := math.Min(float64(maxW)/float64(srcW), float64(maxH)/float64(srcH))
	return int(math.Round(float64(srcW) * ratio)), int(math.Round(float64(srcH) * ratio))
}
