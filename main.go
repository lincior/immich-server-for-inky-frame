package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lincior/immich-server-for-inky-frame/imaging"
	"github.com/lincior/immich-server-for-inky-frame/immich"
)

// buildImageHandler returns a Gin handler that fetches a random image from the
// Immich server at immichURL, resizes it to 800×480 for the Inky Frame 7.3",
// and serves it back as a JPEG.
func buildImageHandler(immichURL, apiKey string) gin.HandlerFunc {
	client := immich.NewClient(immichURL, apiKey)

	return func(c *gin.Context) {
		// Step 1: obtain a random asset ID from the Immich server.
		asset, err := client.RandomAsset()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": fmt.Sprintf("failed to fetch random asset: %v", err),
			})
			return
		}

		// Step 2: download the asset image data.
		imgData, err := client.DownloadAsset(asset.ID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": fmt.Sprintf("failed to download asset %s: %v", asset.ID, err),
			})
			return
		}

		// Step 3: resize and letterbox to 800×480.
		resized, err := imaging.ResizeForInkyFrame(imgData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to resize asset %s: %v", asset.ID, err),
			})
			return
		}

		c.Data(http.StatusOK, "image/jpeg", resized)
	}
}

func main() {
	immichURL := os.Getenv("IMMICH_URL")
	if immichURL == "" {
		log.Fatal("IMMICH_URL environment variable is required (e.g. http://immich.local:2283)")
	}

	apiKey := os.Getenv("IMMICH_API_KEY")
	if apiKey == "" {
		log.Fatal("IMMICH_API_KEY environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()

	// GET /image — returns an 800×480 JPEG from a random Immich asset, sized
	// for the Pimoroni Inky Frame 7.3-inch ePaper display.
	r.GET("/image", buildImageHandler(immichURL, apiKey))

	addr := ":" + strings.TrimPrefix(port, ":")
	log.Printf("Starting server on %s (IMMICH_URL=%s)", addr, immichURL)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
