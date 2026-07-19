package main

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lincior/immich-server-for-inky-frame/imaging"
)

// fakeImmichServer starts an httptest server that mimics the two Immich
// endpoints used by the service: /api/search/random and
// /api/assets/{id}/thumbnail.
func fakeImmichServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Build a small JPEG to return as the "preview thumbnail".
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 149, B: 237, A: 255})
		}
	}
	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, nil); err != nil {
		t.Fatalf("fakeImmichServer: encode jpeg: %v", err)
	}
	jpegData := jpegBuf.Bytes()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/search/random", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{{"id": "test-asset-id", "type": "IMAGE"}})
	})

	mux.HandleFunc("/api/assets/test-asset-id/thumbnail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegData)
	})

	return httptest.NewServer(mux)
}

func TestImageEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := fakeImmichServer(t)
	defer fake.Close()

	// Re-use the same handler wiring as main() but pointed at the fake server.
	r := gin.New()
	// Inline the handler so we can inject the fake URL without modifying main.
	r.GET("/image", buildImageHandler(fake.URL, "dummy-api-key"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/image", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", ct)
	}

	img, _, err := image.Decode(w.Body)
	if err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != imaging.InkyFrameWidth || b.Dy() != imaging.InkyFrameHeight {
		t.Errorf("response image size = %dx%d, want %dx%d",
			b.Dx(), b.Dy(), imaging.InkyFrameWidth, imaging.InkyFrameHeight)
	}
}
