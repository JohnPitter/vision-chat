package vision

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
)

// GridConfig controls the grid overlay dimensions.
type GridConfig struct {
	Cols int // number of columns (default: 8)
	Rows int // number of rows (default: 6)
}

// DefaultGridConfig returns an 8x6 grid (48 regions).
func DefaultGridConfig() GridConfig {
	return GridConfig{Cols: 8, Rows: 6}
}

// GridRegion represents a numbered region on the screen.
type GridRegion struct {
	Number int
	CenterX, CenterY int // center in image coordinates
	X1, Y1, X2, Y2   int // bounds in image coordinates
}

// DrawGridOverlay takes a base64 JPEG frame and draws a numbered grid over it.
// Returns the annotated image as base64 JPEG.
func DrawGridOverlay(b64Frame string, cfg GridConfig) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64Frame)
	if err != nil {
		return "", fmt.Errorf("invalid base64: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("invalid image: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Create a mutable copy
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)

	cellW := w / cfg.Cols
	cellH := h / cfg.Rows

	// Draw grid lines
	gridColor := color.RGBA{R: 255, G: 100, B: 0, A: 180} // orange
	for col := 1; col < cfg.Cols; col++ {
		x := col * cellW
		drawVerticalLine(dst, x, 0, h, gridColor)
	}
	for row := 1; row < cfg.Rows; row++ {
		y := row * cellH
		drawHorizontalLine(dst, 0, w, y, gridColor)
	}

	// Draw numbers in each cell
	num := 1
	for row := 0; row < cfg.Rows; row++ {
		for col := 0; col < cfg.Cols; col++ {
			x := col*cellW + 3
			y := row*cellH + 2
			drawNumber(dst, x, y, num, gridColor)
			num++
		}
	}

	// Encode result
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("encode failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// GetGridRegions returns all grid regions with their center coordinates.
func GetGridRegions(imgWidth, imgHeight int, cfg GridConfig) []GridRegion {
	cellW := imgWidth / cfg.Cols
	cellH := imgHeight / cfg.Rows
	regions := make([]GridRegion, 0, cfg.Cols*cfg.Rows)

	num := 1
	for row := 0; row < cfg.Rows; row++ {
		for col := 0; col < cfg.Cols; col++ {
			x1 := col * cellW
			y1 := row * cellH
			x2 := x1 + cellW
			y2 := y1 + cellH
			regions = append(regions, GridRegion{
				Number:  num,
				CenterX: (x1 + x2) / 2,
				CenterY: (y1 + y2) / 2,
				X1: x1, Y1: y1, X2: x2, Y2: y2,
			})
			num++
		}
	}
	return regions
}

// RegionToCenter converts a grid region number to the center coordinates in image space.
func RegionToCenter(regionNum int, imgWidth, imgHeight int, cfg GridConfig) (int, int, error) {
	if regionNum < 1 || regionNum > cfg.Cols*cfg.Rows {
		return 0, 0, fmt.Errorf("invalid region number %d (must be 1-%d)", regionNum, cfg.Cols*cfg.Rows)
	}

	cellW := imgWidth / cfg.Cols
	cellH := imgHeight / cfg.Rows

	idx := regionNum - 1
	col := idx % cfg.Cols
	row := idx / cfg.Cols

	centerX := col*cellW + cellW/2
	centerY := row*cellH + cellH/2

	return centerX, centerY, nil
}

// === Simple pixel drawing (no external font dependency) ===

func drawVerticalLine(img *image.RGBA, x, y1, y2 int, c color.Color) {
	for y := y1; y < y2; y++ {
		img.Set(x, y, c)
	}
}

func drawHorizontalLine(img *image.RGBA, x1, x2, y int, c color.Color) {
	for x := x1; x < x2; x++ {
		img.Set(x, y, c)
	}
}

// drawNumber draws a small number at (x,y) using a tiny pixel font.
func drawNumber(img *image.RGBA, x, y, num int, c color.Color) {
	s := fmt.Sprintf("%d", num)
	// Draw background box for readability
	boxW := len(s)*5 + 4
	boxH := 9
	bgColor := color.RGBA{R: 0, G: 0, B: 0, A: 180}
	for by := y; by < y+boxH && by < img.Bounds().Dy(); by++ {
		for bx := x; bx < x+boxW && bx < img.Bounds().Dx(); bx++ {
			img.Set(bx, by, bgColor)
		}
	}

	// Draw each digit
	offsetX := x + 2
	for _, ch := range s {
		digit := int(ch - '0')
		drawDigit(img, offsetX, y+1, digit, c)
		offsetX += 5
	}
}

// 3x5 pixel font for digits 0-9
var digitPatterns = [10][5][3]bool{
	// 0
	{{true, true, true}, {true, false, true}, {true, false, true}, {true, false, true}, {true, true, true}},
	// 1
	{{false, true, false}, {true, true, false}, {false, true, false}, {false, true, false}, {true, true, true}},
	// 2
	{{true, true, true}, {false, false, true}, {true, true, true}, {true, false, false}, {true, true, true}},
	// 3
	{{true, true, true}, {false, false, true}, {true, true, true}, {false, false, true}, {true, true, true}},
	// 4
	{{true, false, true}, {true, false, true}, {true, true, true}, {false, false, true}, {false, false, true}},
	// 5
	{{true, true, true}, {true, false, false}, {true, true, true}, {false, false, true}, {true, true, true}},
	// 6
	{{true, true, true}, {true, false, false}, {true, true, true}, {true, false, true}, {true, true, true}},
	// 7
	{{true, true, true}, {false, false, true}, {false, false, true}, {false, false, true}, {false, false, true}},
	// 8
	{{true, true, true}, {true, false, true}, {true, true, true}, {true, false, true}, {true, true, true}},
	// 9
	{{true, true, true}, {true, false, true}, {true, true, true}, {false, false, true}, {true, true, true}},
}

func drawDigit(img *image.RGBA, x, y, digit int, c color.Color) {
	if digit < 0 || digit > 9 {
		return
	}
	pattern := digitPatterns[digit]
	for row := 0; row < 5; row++ {
		for col := 0; col < 3; col++ {
			if pattern[row][col] {
				px := x + col
				py := y + row + 1
				if px < img.Bounds().Dx() && py < img.Bounds().Dy() {
					img.Set(px, py, c)
				}
			}
		}
	}
}
