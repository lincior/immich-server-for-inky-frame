package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
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

// restrictToNetworks returns a Gin middleware that rejects requests whose
// client IP falls outside every configured network. It relies on c.ClientIP()
// reading the real socket address rather than a client-supplied header, so
// callers must have disabled proxy trust (see SetTrustedProxies below);
// otherwise this check could be bypassed via a spoofed X-Forwarded-For.
func restrictToNetworks(networks []*net.IPNet) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawIP, ip := resolveClientIP(c)
		if ip == nil {
			log.Printf("blocking request: client ip could not be parsed (gin=%q remote=%q resolved=%q)", c.ClientIP(), c.Request.RemoteAddr, rawIP)
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		for _, network := range networks {
			if network.Contains(ip) {
				c.Next()
				return
			}
		}

		log.Printf("blocking request: client ip gin=%q remote=%q resolved=%q parsed=%s not in allowed networks=%s", c.ClientIP(), c.Request.RemoteAddr, rawIP, ip.String(), strings.Join(networkStrings(networks), ","))
		c.AbortWithStatus(http.StatusForbidden)
	}
}

func networkStrings(networks []*net.IPNet) []string {
	parts := make([]string, 0, len(networks))
	for _, n := range networks {
		parts = append(parts, n.String())
	}
	return parts
}

func parseClientIP(raw string) net.IP {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	// Strip IPv6 zone identifiers (e.g. fe80::1%wlan0) before parsing.
	if idx := strings.Index(raw, "%"); idx >= 0 {
		raw = raw[:idx]
	}
	return net.ParseIP(raw)
}

func parseRemoteAddrIP(remoteAddr string) string {
	addr := strings.TrimSpace(remoteAddr)
	if addr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}

	// Fallback for malformed or non-host:port remote addresses.
	return addr
}

func resolveClientIP(c *gin.Context) (string, net.IP) {
	if ginIP := strings.TrimSpace(c.ClientIP()); ginIP != "" {
		return ginIP, parseClientIP(ginIP)
	}

	raw := parseRemoteAddrIP(c.Request.RemoteAddr)
	return raw, parseClientIP(raw)
}

func parseAllowedNetworks(value string) ([]*net.IPNet, error) {
	tokens := strings.Split(value, ",")
	networks := make([]*net.IPNet, 0, len(tokens))
	seen := make(map[string]struct{})

	for _, token := range tokens {
		entry := strings.TrimSpace(token)
		if entry == "" {
			continue
		}

		if strings.EqualFold(entry, "auto") {
			autoNetworks, err := localInterfaceNetworks()
			if err != nil {
				return nil, err
			}
			for _, network := range autoNetworks {
				key := network.String()
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				networks = append(networks, network)
			}
			continue
		}

		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid ALLOWED_NETWORK entry %q: %w", entry, err)
		}

		key := network.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		networks = append(networks, network)
	}

	if len(networks) == 0 {
		return nil, fmt.Errorf("ALLOWED_NETWORK did not produce any CIDR entries")
	}

	return networks, nil
}

func localInterfaceNetworks() ([]*net.IPNet, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	seen := make(map[string]struct{})
	networks := make([]*net.IPNet, 0)

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("list addresses for interface %s: %w", iface.Name, err)
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			key := ipNet.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			networks = append(networks, ipNet)
		}
	}

	if len(networks) == 0 {
		return nil, fmt.Errorf("no local interface networks discovered")
	}

	sort.Slice(networks, func(i, j int) bool {
		return networks[i].String() < networks[j].String()
	})

	return networks, nil
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

	allowedNetworkSpec := os.Getenv("ALLOWED_NETWORK")
	if allowedNetworkSpec == "" {
		log.Fatal("ALLOWED_NETWORK environment variable is required (e.g. 192.168.1.0/24 or auto)")
	}

	allowedNetworks, err := parseAllowedNetworks(allowedNetworkSpec)
	if err != nil {
		log.Fatalf("invalid ALLOWED_NETWORK %q: %v", allowedNetworkSpec, err)
	}
	log.Printf("resolved ALLOWED_NETWORK=%q to: %s", allowedNetworkSpec, strings.Join(networkStrings(allowedNetworks), ","))

	r := gin.Default()

	// No reverse proxy sits in front of this server, so there are no trusted
	// hops to configure; disable proxy trust rather than defaulting to "trust
	// everyone", which would let a client spoof its address via X-Forwarded-For.
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatalf("failed to configure trusted proxies: %v", err)
	}

	r.Use(restrictToNetworks(allowedNetworks))

	// GET /image — returns an 800×480 JPEG from a random Immich asset, sized
	// for the Pimoroni Inky Frame 7.3-inch ePaper display.
	r.GET("/image", buildImageHandler(immichURL, apiKey))

	addr := ":" + strings.TrimPrefix(port, ":")
	log.Printf("Starting server on %s (IMMICH_URL=%s)", addr, immichURL)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
