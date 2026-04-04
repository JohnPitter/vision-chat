package vision

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"strings"

	"golang.org/x/image/draw"
)

// ValidateBase64Image decodes a base64 string and verifies it is a valid image.
func ValidateBase64Image(b64 string) error {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return errors.New("invalid base64 encoding")
	}
	_, _, err = image.Decode(bytes.NewReader(data))
	if err != nil {
		return errors.New("data is not a valid image")
	}
	return nil
}

// ProcessFrame decodes a base64 image, resizes it if larger than maxDimension,
// and re-encodes as JPEG base64.
func ProcessFrame(b64 string, maxDimension int) (string, error) {
	if b64 == "" {
		return "", errors.New("empty input")
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", errors.New("invalid base64 encoding")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("data is not a valid image")
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Resize if needed (only downscale, never upscale)
	maxDim := w
	if h > maxDim {
		maxDim = h
	}

	if maxDim > maxDimension {
		ratio := float64(maxDimension) / float64(maxDim)
		newW := int(float64(w) * ratio)
		newH := int(float64(h) * ratio)

		dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return "", errors.New("failed to encode as JPEG")
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// FormatAsDataURI wraps a base64 JPEG string as a data URI.
func FormatAsDataURI(b64 string) string {
	return "data:image/jpeg;base64," + b64
}

// ExtractBase64FromDataURI strips the data URI prefix if present.
func ExtractBase64FromDataURI(uri string) string {
	if idx := strings.Index(uri, ";base64,"); idx != -1 {
		return uri[idx+8:]
	}
	return uri
}
