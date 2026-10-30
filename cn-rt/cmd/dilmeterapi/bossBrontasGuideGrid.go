package main

import "fmt"

// A fixed grid has exactly three adjacent columns and three adjacent rows.
// Calibration sets the middle band of each axis; adjacent centers are one
// FULL band width apart, never a separately chosen spacing. Each outer band
// must extend past the selected mineral boundary. No fourth slit is a cell.
type brontasGuideGrid struct {
	XCenter    float64 `json:"xCenter"`
	YCenter    float64 `json:"yCenter"`
	XHalfWidth float64 `json:"xHalfWidth"`
	YHalfWidth float64 `json:"yHalfWidth"`
}

// Used for labeled previews only. Live calibration must supply its own layout.
func brontasGuidePreviewGrid() brontasGuideGrid {
	return brontasGuideGrid{XCenter: 0, YCenter: -.12, XHalfWidth: .40, YHalfWidth: .51}
}

// Lane 0/1/2 means left/middle/right for vertical rays, top/middle/bottom
// for horizontal rays. Returned coordinates use the planner's +Y north axis.
func (g brontasGuideGrid) rayGeometry(axis string, lane int) (float64, float64, error) {
	if lane < 0 || lane > 2 || !finiteBrontasNumber(g.XCenter) || !finiteBrontasNumber(g.YCenter) ||
		!finiteBrontasNumber(g.XHalfWidth) || !finiteBrontasNumber(g.YHalfWidth) || g.XHalfWidth <= 0 || g.YHalfWidth <= 0 ||
		g.XCenter-3*g.XHalfWidth > -1 || g.XCenter+3*g.XHalfWidth < 1 ||
		g.YCenter-3*g.YHalfWidth > -1 || g.YCenter+3*g.YHalfWidth < 1 {
		return 0, 0, fmt.Errorf("invalid three-by-three grid")
	}
	switch axis {
	case "vertical":
		return g.XCenter + float64(lane-1)*2*g.XHalfWidth, g.XHalfWidth, nil
	case "horizontal":
		return g.YCenter + float64(1-lane)*2*g.YHalfWidth, g.YHalfWidth, nil
	default:
		return 0, 0, fmt.Errorf("invalid grid axis")
	}
}

// laneAt locks a fixed ray to the row/column occupied at warning time.
// Borders belong to the middle lane; adjacent bands still share that border.
func (g brontasGuideGrid) laneAt(axis string, p brontasPoint) (int, error) {
	if _, _, err := g.rayGeometry(axis, 1); err != nil {
		return 0, err
	}
	if !finiteBrontasNumber(p.X) || !finiteBrontasNumber(p.Y) {
		return 0, fmt.Errorf("invalid player position")
	}
	if axis == "vertical" {
		if p.X < g.XCenter-g.XHalfWidth {
			return 0, nil
		}
		if p.X > g.XCenter+g.XHalfWidth {
			return 2, nil
		}
	} else {
		if p.Y > g.YCenter+g.YHalfWidth {
			return 0, nil
		}
		if p.Y < g.YCenter-g.YHalfWidth {
			return 2, nil
		}
	}
	return 1, nil
}
