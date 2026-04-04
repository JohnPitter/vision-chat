package vision

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// createColorJPEG creates a JPEG filled with a solid color.
func createColorJPEG(width, height int, c color.Color) string {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// createPartialChangeJPEG creates a JPEG with a small changed region.
func createPartialChangeJPEG(width, height int, bg, fg color.Color, fgPercent float64) string {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	changePixels := int(float64(width*height) * fgPercent)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if idx < changePixels {
				img.Set(x, y, fg)
			} else {
				img.Set(x, y, bg)
			}
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestNewFrameCache(t *testing.T) {
	fc := NewFrameCache(DefaultCacheConfig())
	if fc == nil {
		t.Fatal("NewFrameCache returned nil")
	}
}

func TestFrameCache_FirstFrameAlwaysNew(t *testing.T) {
	fc := NewFrameCache(DefaultCacheConfig())
	frame := createColorJPEG(64, 64, color.White)

	result := fc.Analyze(frame)
	if !result.IsNew {
		t.Error("first frame should always be marked as new")
	}
	if result.ChangePercent != 1.0 {
		t.Errorf("first frame change should be 1.0, got %f", result.ChangePercent)
	}
}

func TestFrameCache_IdenticalFramesCached(t *testing.T) {
	fc := NewFrameCache(DefaultCacheConfig())
	frame := createColorJPEG(64, 64, color.White)

	fc.Analyze(frame) // first = always new
	result := fc.Analyze(frame)

	if result.IsNew {
		t.Error("identical frame should be cached (not new)")
	}
	if result.ChangePercent > 0.01 {
		t.Errorf("identical frames should have ~0%% change, got %.2f%%", result.ChangePercent*100)
	}
}

func TestFrameCache_DifferentFrameDetected(t *testing.T) {
	fc := NewFrameCache(DefaultCacheConfig())
	white := createColorJPEG(64, 64, color.White)
	black := createColorJPEG(64, 64, color.Black)

	fc.Analyze(white)
	result := fc.Analyze(black)

	if !result.IsNew {
		t.Error("completely different frame should be detected as new")
	}
	if result.ChangePercent < 0.5 {
		t.Errorf("white→black should have high change, got %.2f%%", result.ChangePercent*100)
	}
}

func TestFrameCache_SmallChangeIgnored(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.ChangeThreshold = 0.10 // 10% threshold
	fc := NewFrameCache(cfg)

	bg := createColorJPEG(64, 64, color.White)
	// 5% of pixels changed - below threshold
	small := createPartialChangeJPEG(64, 64, color.White, color.Black, 0.05)

	fc.Analyze(bg)
	result := fc.Analyze(small)

	if result.IsNew {
		t.Error("small change below threshold should be cached")
	}
}

func TestFrameCache_LargeChangeDetected(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.ChangeThreshold = 0.10
	fc := NewFrameCache(cfg)

	bg := createColorJPEG(64, 64, color.White)
	// 30% of pixels changed - above threshold
	large := createPartialChangeJPEG(64, 64, color.White, color.Black, 0.30)

	fc.Analyze(bg)
	result := fc.Analyze(large)

	if !result.IsNew {
		t.Error("large change above threshold should be detected as new")
	}
}

func TestFrameCache_CacheResponse(t *testing.T) {
	fc := NewFrameCache(DefaultCacheConfig())
	frame := createColorJPEG(64, 64, color.White)

	fc.Analyze(frame)
	fc.CacheResponse("This is a white image.")

	resp, ok := fc.GetCachedResponse()
	if !ok {
		t.Fatal("expected cached response")
	}
	if resp != "This is a white image." {
		t.Errorf("unexpected cached response: %s", resp)
	}
}

func TestFrameCache_CacheInvalidatedOnNewFrame(t *testing.T) {
	fc := NewFrameCache(DefaultCacheConfig())
	white := createColorJPEG(64, 64, color.White)
	black := createColorJPEG(64, 64, color.Black)

	fc.Analyze(white)
	fc.CacheResponse("White image.")

	fc.Analyze(black) // new frame = cache invalidated

	_, ok := fc.GetCachedResponse()
	if ok {
		t.Error("cache should be invalidated after new frame detected")
	}
}

func TestFrameCache_NoCachedResponseInitially(t *testing.T) {
	fc := NewFrameCache(DefaultCacheConfig())
	_, ok := fc.GetCachedResponse()
	if ok {
		t.Error("no cached response should exist initially")
	}
}

func TestFrameCache_AdaptiveInterval(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.MinProcessInterval = 16  // ~60fps capture
	cfg.MaxProcessInterval = 500 // slow down when static
	fc := NewFrameCache(cfg)

	// After identical frames, interval should increase
	frame := createColorJPEG(64, 64, color.White)
	fc.Analyze(frame)

	for i := 0; i < 10; i++ {
		fc.Analyze(frame) // all identical
	}

	interval := fc.CurrentInterval()
	if interval <= cfg.MinProcessInterval {
		t.Errorf("interval should increase for static scene, got %dms", interval)
	}

	// After a big change, interval should reset to minimum
	black := createColorJPEG(64, 64, color.Black)
	fc.Analyze(black)
	interval = fc.CurrentInterval()
	if interval != cfg.MinProcessInterval {
		t.Errorf("interval should reset to min after change, got %dms", interval)
	}
}

func TestFrameCache_FrameStats(t *testing.T) {
	fc := NewFrameCache(DefaultCacheConfig())
	white := createColorJPEG(64, 64, color.White)
	black := createColorJPEG(64, 64, color.Black)

	fc.Analyze(white)
	fc.Analyze(white) // cached
	fc.Analyze(white) // cached
	fc.Analyze(black) // new

	stats := fc.Stats()
	if stats.TotalFrames != 4 {
		t.Errorf("expected 4 total frames, got %d", stats.TotalFrames)
	}
	if stats.CachedFrames != 2 {
		t.Errorf("expected 2 cached frames, got %d", stats.CachedFrames)
	}
	if stats.ProcessedFrames != 2 {
		t.Errorf("expected 2 processed frames, got %d", stats.ProcessedFrames)
	}
}

func TestComputeFrameDiff_IdenticalImages(t *testing.T) {
	img1 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img2 := image.NewRGBA(image.Rect(0, 0, 10, 10))

	diff := ComputeFrameDiff(img1, img2)
	if diff > 0.001 {
		t.Errorf("identical images should have ~0 diff, got %f", diff)
	}
}

func TestComputeFrameDiff_CompletelyDifferent(t *testing.T) {
	img1 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img2 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img1.Set(x, y, color.White)
			img2.Set(x, y, color.Black)
		}
	}

	diff := ComputeFrameDiff(img1, img2)
	if diff < 0.5 {
		t.Errorf("completely different images should have high diff, got %f", diff)
	}
}

func TestComputeFrameDiff_DifferentSizes(t *testing.T) {
	img1 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img2 := image.NewRGBA(image.Rect(0, 0, 20, 20))

	// Should not panic, treat as fully different
	diff := ComputeFrameDiff(img1, img2)
	if diff != 1.0 {
		t.Errorf("different sizes should return 1.0, got %f", diff)
	}
}

func TestDownsampleForComparison(t *testing.T) {
	// Large image should be downsampled for fast comparison
	large := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	ds := DownsampleForComparison(large, 64)

	bounds := ds.Bounds()
	if bounds.Dx() > 64 || bounds.Dy() > 64 {
		t.Errorf("downsampled image should be <= 64x64, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestDownsampleForComparison_SmallImageUnchanged(t *testing.T) {
	small := image.NewRGBA(image.Rect(0, 0, 32, 32))
	ds := DownsampleForComparison(small, 64)

	bounds := ds.Bounds()
	if bounds.Dx() != 32 || bounds.Dy() != 32 {
		t.Errorf("small image should not be modified, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
