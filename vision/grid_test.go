package vision

import (
	"testing"
)

func TestDrawGridOverlay(t *testing.T) {
	frame := createTestJPEG(512, 288)
	cfg := DefaultGridConfig()

	result, err := DrawGridOverlay(frame, cfg)
	if err != nil {
		t.Fatalf("DrawGridOverlay error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Result should be larger than input (has overlay drawn)
	if len(result) < len(frame) {
		t.Error("annotated image should be at least as large as original")
	}
}

func TestGetGridRegions(t *testing.T) {
	cfg := GridConfig{Cols: 4, Rows: 3}
	regions := GetGridRegions(400, 300, cfg)

	if len(regions) != 12 {
		t.Fatalf("expected 12 regions, got %d", len(regions))
	}

	// Region 1: top-left
	if regions[0].Number != 1 {
		t.Errorf("first region should be 1, got %d", regions[0].Number)
	}
	if regions[0].CenterX != 50 || regions[0].CenterY != 50 {
		t.Errorf("region 1 center: expected (50,50), got (%d,%d)", regions[0].CenterX, regions[0].CenterY)
	}

	// Region 12: bottom-right
	if regions[11].Number != 12 {
		t.Errorf("last region should be 12, got %d", regions[11].Number)
	}
	if regions[11].CenterX != 350 || regions[11].CenterY != 250 {
		t.Errorf("region 12 center: expected (350,250), got (%d,%d)", regions[11].CenterX, regions[11].CenterY)
	}
}

func TestRegionToCenter(t *testing.T) {
	cfg := GridConfig{Cols: 8, Rows: 6}

	// Region 1: top-left
	x, y, err := RegionToCenter(1, 512, 288, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if x != 32 || y != 24 {
		t.Errorf("region 1: expected (32,24), got (%d,%d)", x, y)
	}

	// Region 8: top-right
	x, y, err = RegionToCenter(8, 512, 288, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if x != 480 || y != 24 {
		t.Errorf("region 8: expected (480,24), got (%d,%d)", x, y)
	}

	// Region 48: bottom-right
	x, y, err = RegionToCenter(48, 512, 288, cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if x != 480 || y != 264 {
		t.Errorf("region 48: expected (480,264), got (%d,%d)", x, y)
	}

	// Invalid regions
	_, _, err = RegionToCenter(0, 512, 288, cfg)
	if err == nil {
		t.Error("region 0 should be invalid")
	}
	_, _, err = RegionToCenter(49, 512, 288, cfg)
	if err == nil {
		t.Error("region 49 should be invalid")
	}
}

func TestDefaultGridConfig(t *testing.T) {
	cfg := DefaultGridConfig()
	if cfg.Cols != 8 || cfg.Rows != 6 {
		t.Errorf("expected 8x6, got %dx%d", cfg.Cols, cfg.Rows)
	}
	total := cfg.Cols * cfg.Rows
	if total != 48 {
		t.Errorf("expected 48 regions, got %d", total)
	}
}
