# immich-server-for-inky-frame
Polling images from an Immich server and serving them to an inky frame

## Overview

A small Go/Gin HTTP server that pulls a random photo from an
[Immich](https://immich.app/) instance, resizes it to **800 × 480 pixels**
(letterboxed on a black canvas) for the
[Pimoroni Inky Frame 7.3-inch](https://shop.pimoroni.com/products/inky-frame)
ePaper display, and serves the result as a JPEG.

### How it works

1. `GET /image` is called by the Inky Frame (or any HTTP client).
2. The server calls Immich's `GET /api/search/random` to obtain a random asset ID.
3. It then downloads the asset preview via `GET /api/assets/{id}/thumbnail?size=preview`.
4. The image is scaled to fit within 800 × 480 while preserving its aspect ratio,
   centred on a black 800 × 480 canvas (letterbox / pillarbox), and returned as JPEG.

## Configuration

All configuration is via environment variables:

| Variable        | Required | Default | Description                                   |
|-----------------|----------|---------|-----------------------------------------------|
| `IMMICH_URL`    | ✅       | —       | Base URL of the Immich server, e.g. `http://immich.local:2283` |
| `IMMICH_API_KEY`| ✅       | —       | Immich API key (Settings → API Keys)          |
| `PORT`          | ❌       | `8080`  | Port the server listens on                    |

## Running

```bash
export IMMICH_URL=http://immich.local:2283
export IMMICH_API_KEY=your_api_key_here
go run .
# or build first:
go build -o immich-inky-frame .
./immich-inky-frame
```

The server starts on port `8080` by default.  Point the Inky Frame at
`http://<server-host>:8080/image`.

## Development

```bash
# Run tests
go test ./...

# Build
go build ./...
```
