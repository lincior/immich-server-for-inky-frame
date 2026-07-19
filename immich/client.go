// Package immich provides a minimal client for the Immich photo server API.
package immich

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Asset represents the fields returned by the Immich /api/search/random endpoint
// that are relevant to this service.
type Asset struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Client is a minimal Immich API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Client.  baseURL should be the root URL of the Immich
// instance (e.g. "http://immich.local:2283").  apiKey is the Immich API key used
// for authentication via the x-api-key header.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// RandomAsset returns a single random IMAGE asset from the Immich server.
func (c *Client) RandomAsset() (*Asset, error) {
	url := c.baseURL + "/api/search/random"
	body, err := json.Marshal(map[string]any{
		"size": 1,
		"type": "IMAGE",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch random asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("immich returned status %d for /api/search/random", resp.StatusCode)
	}

	var assets []Asset
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(assets) == 0 {
		return nil, fmt.Errorf("no assets returned by /api/search/random")
	}

	return &assets[0], nil
}

// DownloadAsset fetches the preview JPEG of the asset identified by id.
// Immich preview thumbnails are always JPEG, which makes them straightforward
// to decode and resize regardless of the original file format (HEIC, RAW, etc.).
func (c *Client) DownloadAsset(id string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/assets/%s/thumbnail?size=preview", c.baseURL, id)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download asset %s: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("immich returned status %d for asset %s", resp.StatusCode, id)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read asset body: %w", err)
	}

	return data, nil
}
