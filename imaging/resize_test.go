package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// makeJPEG encodes a solid-colour image of the given dimensions as JPEG.
func makeJPEG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("makeJPEG: %v", err)
	}
	return buf.Bytes()
}

func TestFitDimensions(t *testing.T) {
	tests := []struct {
		name        string
		srcW, srcH  int
		maxW, maxH  int
		wantW, wantH int
	}{
		{
			name:  "landscape wider than frame",
			srcW:  1600, srcH: 900,
			maxW:  800, maxH: 480,
			wantW: 800, wantH: 450,
		},
		{
			name:  "portrait taller than frame",
			srcW:  600, srcH: 1200,
			maxW:  800, maxH: 480,
			wantW: 240, wantH: 480,
		},
		{
			name:  "already fits",
			srcW:  400, srcH: 200,
			maxW:  800, maxH: 480,
			wantW: 800, wantH: 400,
		},
		{
			name:  "exact frame size",
			srcW:  800, srcH: 480,
			maxW:  800, maxH: 480,
			wantW: 800, wantH: 480,
		},
		{
			name:  "square image",
			srcW:  1000, srcH: 1000,
			maxW:  800, maxH: 480,
			wantW: 480, wantH: 480,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := fitDimensions(tt.srcW, tt.srcH, tt.maxW, tt.maxH)
			if gotW != tt.wantW || gotH != tt.wantH {
				t.Errorf("fitDimensions(%d,%d,%d,%d) = (%d,%d), want (%d,%d)",
					tt.srcW, tt.srcH, tt.maxW, tt.maxH,
					gotW, gotH, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestResizeForInkyFrame_OutputDimensions(t *testing.T) {
	tests := []struct {
		name       string
		srcW, srcH int
	}{
		{"landscape 16:9", 1920, 1080},
		{"portrait 9:16", 1080, 1920},
		{"square", 600, 600},
		{"exact frame size", 800, 480},
		{"small image", 100, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := makeJPEG(t, tt.srcW, tt.srcH, color.RGBA{R: 200, G: 100, B: 50, A: 255})
			out, err := ResizeForInkyFrame(data)
			if err != nil {
				t.Fatalf("ResizeForInkyFrame error: %v", err)
			}

			img, _, err := image.Decode(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("decode output: %v", err)
			}

			b := img.Bounds()
			if b.Dx() != InkyFrameWidth || b.Dy() != InkyFrameHeight {
				t.Errorf("output size = %dx%d, want %dx%d",
					b.Dx(), b.Dy(), InkyFrameWidth, InkyFrameHeight)
			}
		})
	}
}

func TestResizeForInkyFrame_InvalidInput(t *testing.T) {
	_, err := ResizeForInkyFrame([]byte("not an image"))
	if err == nil {
		t.Error("expected error for invalid image data, got nil")
	}
}
