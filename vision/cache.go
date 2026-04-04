package vision

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sync"

	"golang.org/x/image/draw"
)

// CacheConfig controls the smart frame cache behavior.
type CacheConfig struct {
	ChangeThreshold    float64 // 0.0-1.0, how much change triggers a new frame (default: 0.05 = 5%)
	ComparisonSize     int     // Downsample to NxN for fast comparison (default: 64)
	MinProcessInterval int     // Minimum ms between AI calls (default: 16 = ~60fps capture)
	MaxProcessInterval int     // Maximum ms when scene is static (default: 500)
}

// DefaultCacheConfig returns production defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		ChangeThreshold:    0.05,
		ComparisonSize:     64,
		MinProcessInterval: 16,
		MaxProcessInterval: 500,
	}
}

// AnalyzeResult holds the result of frame analysis.
type AnalyzeResult struct {
	IsNew         bool    // true if frame differs enough from cached
	ChangePercent float64 // 0.0-1.0 how much changed
}

// CacheStats holds frame cache statistics.
type CacheStats struct {
	TotalFrames     int
	CachedFrames    int
	ProcessedFrames int
}

// FrameCache implements intelligent frame caching with motion detection.
// It operates at 60fps capture rate but only flags frames as "new" when
// significant visual change is detected — like human vision focusing
// only on what moves or changes.
type FrameCache struct {
	mu              sync.Mutex
	cfg             CacheConfig
	lastFrame       image.Image // downsampled last frame for comparison
	cachedResponse  string
	hasCache        bool
	currentInterval int
	staticCount     int // consecutive static frames
	stats           CacheStats
}

// NewFrameCache creates a new smart frame cache.
func NewFrameCache(cfg CacheConfig) *FrameCache {
	return &FrameCache{
		cfg:             cfg,
		currentInterval: cfg.MinProcessInterval,
	}
}

// Analyze checks if a base64 frame has changed significantly from the last one.
// Call this at 60fps — it's fast because it downsamples before comparing.
func (fc *FrameCache) Analyze(b64Frame string) AnalyzeResult {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.stats.TotalFrames++

	// Decode frame
	data, err := base64.StdEncoding.DecodeString(b64Frame)
	if err != nil {
		return AnalyzeResult{IsNew: true, ChangePercent: 1.0}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return AnalyzeResult{IsNew: true, ChangePercent: 1.0}
	}

	// Downsample for fast comparison
	small := DownsampleForComparison(img, fc.cfg.ComparisonSize)

	// First frame is always new
	if fc.lastFrame == nil {
		fc.lastFrame = small
		fc.hasCache = false
		fc.stats.ProcessedFrames++
		return AnalyzeResult{IsNew: true, ChangePercent: 1.0}
	}

	// Compare with last frame
	diff := ComputeFrameDiff(fc.lastFrame, small)

	if diff >= fc.cfg.ChangeThreshold {
		// Significant change detected
		fc.lastFrame = small
		fc.hasCache = false
		fc.staticCount = 0
		fc.currentInterval = fc.cfg.MinProcessInterval
		fc.stats.ProcessedFrames++
		return AnalyzeResult{IsNew: true, ChangePercent: diff}
	}

	// Scene is static — increase interval (adaptive slowdown)
	fc.staticCount++
	fc.adaptInterval()
	fc.stats.CachedFrames++
	return AnalyzeResult{IsNew: false, ChangePercent: diff}
}

// adaptInterval increases the processing interval when scene is static.
// Uses exponential backoff capped at MaxProcessInterval.
func (fc *FrameCache) adaptInterval() {
	if fc.staticCount > 5 {
		newInterval := fc.currentInterval + fc.currentInterval/4 // +25%
		if newInterval > fc.cfg.MaxProcessInterval {
			newInterval = fc.cfg.MaxProcessInterval
		}
		fc.currentInterval = newInterval
	}
}

// CacheResponse stores the AI response for the current frame.
func (fc *FrameCache) CacheResponse(response string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.cachedResponse = response
	fc.hasCache = true
}

// GetCachedResponse returns the cached AI response if available.
func (fc *FrameCache) GetCachedResponse() (string, bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if !fc.hasCache {
		return "", false
	}
	return fc.cachedResponse, true
}

// CurrentInterval returns the current recommended interval between AI calls in ms.
func (fc *FrameCache) CurrentInterval() int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.currentInterval
}

// Stats returns frame cache statistics.
func (fc *FrameCache) Stats() CacheStats {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.stats
}

// ComputeFrameDiff calculates the normalized pixel difference between two images.
// Returns 0.0 (identical) to 1.0 (completely different).
// Uses luminance-based comparison for speed (1 channel instead of 3).
func ComputeFrameDiff(img1, img2 image.Image) float64 {
	b1 := img1.Bounds()
	b2 := img2.Bounds()

	if b1.Dx() != b2.Dx() || b1.Dy() != b2.Dy() {
		return 1.0 // different sizes = treat as fully different
	}

	w, h := b1.Dx(), b1.Dy()
	totalPixels := w * h
	if totalPixels == 0 {
		return 0
	}

	var totalDiff float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r1, g1, b1a, _ := img1.At(x+img1.Bounds().Min.X, y+img1.Bounds().Min.Y).RGBA()
			r2, g2, b2a, _ := img2.At(x+img2.Bounds().Min.X, y+img2.Bounds().Min.Y).RGBA()

			// Luminance (fast approximation): 0.299R + 0.587G + 0.114B
			lum1 := 0.299*float64(r1) + 0.587*float64(g1) + 0.114*float64(b1a)
			lum2 := 0.299*float64(r2) + 0.587*float64(g2) + 0.114*float64(b2a)

			diff := math.Abs(lum1-lum2) / 65535.0 // normalize to 0-1
			totalDiff += diff
		}
	}

	return totalDiff / float64(totalPixels)
}

// DownsampleForComparison resizes an image to at most maxSize x maxSize
// for fast pixel comparison. Small images are returned as-is.
func DownsampleForComparison(img image.Image, maxSize int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w <= maxSize && h <= maxSize {
		return img
	}

	// Calculate new dimensions preserving aspect ratio
	ratio := float64(maxSize) / math.Max(float64(w), float64(h))
	newW := int(float64(w) * ratio)
	newH := int(float64(h) * ratio)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}
