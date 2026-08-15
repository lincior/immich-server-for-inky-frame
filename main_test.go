package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net"
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
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
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

func TestParseClientIP(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "ipv4", raw: "192.168.1.20", want: "192.168.1.20"},
		{name: "ipv6", raw: "2001:db8::1", want: "2001:db8::1"},
		{name: "scoped ipv6", raw: "fe80::1234%wlan0", want: "fe80::1234"},
		{name: "bracketed scoped ipv6", raw: "[fe80::1234%wlan0]", want: "fe80::1234"},
		{name: "invalid", raw: "not-an-ip", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseClientIP(tc.raw)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("parseClientIP(%q) = %v, want nil", tc.raw, got)
				}
				return
			}

			wantIP := net.ParseIP(tc.want)
			if wantIP == nil {
				t.Fatalf("test setup error: invalid want IP %q", tc.want)
			}

			if got == nil || !got.Equal(wantIP) {
				t.Fatalf("parseClientIP(%q) = %v, want %v", tc.raw, got, wantIP)
			}
		})
	}
}

func TestParseRemoteAddrIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{name: "ipv4 host port", remoteAddr: "192.168.1.16:50123", want: "192.168.1.16"},
		{name: "ipv6 host port", remoteAddr: "[fe80::abcd%wlan0]:50123", want: "fe80::abcd%wlan0"},
		{name: "already host", remoteAddr: "fe80::abcd%wlan0", want: "fe80::abcd%wlan0"},
		{name: "empty", remoteAddr: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRemoteAddrIP(tc.remoteAddr)
			if got != tc.want {
				t.Fatalf("parseRemoteAddrIP(%q) = %q, want %q", tc.remoteAddr, got, tc.want)
			}
		})
	}
}
