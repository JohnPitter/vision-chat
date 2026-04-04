package vision

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func createTestJPEG(width, height int) string {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func createTestPNG(width, height int) string {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestValidateBase64Image_ValidJPEG(t *testing.T) {
	b64 := createTestJPEG(100, 100)
	err := ValidateBase64Image(b64)
	if err != nil {
		t.Errorf("expected valid JPEG, got error: %v", err)
	}
}

func TestValidateBase64Image_ValidPNG(t *testing.T) {
	b64 := createTestPNG(100, 100)
	err := ValidateBase64Image(b64)
	if err != nil {
		t.Errorf("expected valid PNG, got error: %v", err)
	}
}

func TestValidateBase64Image_InvalidBase64(t *testing.T) {
	err := ValidateBase64Image("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestValidateBase64Image_NotAnImage(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("hello world"))
	err := ValidateBase64Image(b64)
	if err == nil {
		t.Error("expected error for non-image data")
	}
}

func TestProcessFrame_ResizeDown(t *testing.T) {
	b64 := createTestJPEG(1920, 1080)
	result, err := ProcessFrame(b64, 512)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Fatalf("result is not valid base64: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("result is not a valid image: %v", err)
	}

	bounds := img.Bounds()
	maxDim := bounds.Dx()
	if bounds.Dy() > maxDim {
		maxDim = bounds.Dy()
	}
	if maxDim > 512 {
		t.Errorf("expected max dimension <= 512, got %d", maxDim)
	}
}

func TestProcessFrame_SmallImageNotUpscaled(t *testing.T) {
	b64 := createTestJPEG(200, 150)
	result, err := ProcessFrame(b64, 512)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, _ := base64.StdEncoding.DecodeString(result)
	img, _, _ := image.Decode(bytes.NewReader(decoded))
	bounds := img.Bounds()
	if bounds.Dx() != 200 || bounds.Dy() != 150 {
		t.Errorf("small image should not be upscaled: got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestProcessFrame_OutputIsJPEG(t *testing.T) {
	b64 := createTestPNG(100, 100)
	result, err := ProcessFrame(b64, 512)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, _ := base64.StdEncoding.DecodeString(result)
	_, format, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("expected jpeg output format, got %s", format)
	}
}

func TestFormatAsDataURI(t *testing.T) {
	b64 := "abc123"
	uri := FormatAsDataURI(b64)
	expected := "data:image/jpeg;base64,abc123"
	if uri != expected {
		t.Errorf("expected %s, got %s", expected, uri)
	}
}

func TestExtractBase64FromDataURI(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"data:image/jpeg;base64,abc123", "abc123"},
		{"data:image/png;base64,xyz", "xyz"},
		{"abc123", "abc123"},
	}
	for _, tc := range tests {
		result := ExtractBase64FromDataURI(tc.input)
		if result != tc.expected {
			t.Errorf("input %s: expected %s, got %s", tc.input, tc.expected, result)
		}
	}
}

func TestProcessFrame_EmptyInput(t *testing.T) {
	_, err := ProcessFrame("", 512)
	if err == nil {
		t.Error("expected error for empty input")
	}
}
