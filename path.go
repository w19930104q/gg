package gg

import (
	"math"

	"github.com/golang/freetype/raster"
	"golang.org/x/image/math/fixed"
)

func flattenPath(p raster.Path) [][]Point {
	var result [][]Point
	var path []Point
	var cx, cy float64
	for i := 0; i < len(p); {
		switch p[i] {
		case 0:
			if len(path) > 0 {
				result = append(result, path)
				path = nil
			}
			x := unfix(p[i+1])
			y := unfix(p[i+2])
			path = append(path, Point{x, y})
			cx, cy = x, y
			i += 4
		case 1:
			x := unfix(p[i+1])
			y := unfix(p[i+2])
			path = append(path, Point{x, y})
			cx, cy = x, y
			i += 4
		case 2:
			x1 := unfix(p[i+1])
			y1 := unfix(p[i+2])
			x2 := unfix(p[i+3])
			y2 := unfix(p[i+4])
			points := QuadraticBezier(cx, cy, x1, y1, x2, y2)
			path = append(path, points...)
			cx, cy = x2, y2
			i += 6
		case 3:
			x1 := unfix(p[i+1])
			y1 := unfix(p[i+2])
			x2 := unfix(p[i+3])
			y2 := unfix(p[i+4])
			x3 := unfix(p[i+5])
			y3 := unfix(p[i+6])
			points := CubicBezier(cx, cy, x1, y1, x2, y2, x3, y3)
			path = append(path, points...)
			cx, cy = x3, y3
			i += 8
		default:
			panic("bad path")
		}
	}
	if len(path) > 0 {
		result = append(result, path)
	}
	return result
}

// dashSegment describes an "on" (painted) interval of a single contour,
// measured in arc length along the contour.
type dashSegment struct {
	// start and end are arc-length positions along the contour. For a
	// closed contour they are reduced into [0, length) so that a dash
	// crossing the contour's start/end seam is represented by two
	// segments whose painted arc lengths add up to the original dash.
	start, end float64
}

// normalizeDashes validates the dash pattern and returns a copy of it.
// An empty pattern disables dashing. A pattern with a single entry is
// expanded to equal on/off lengths, matching the semantics of Cairo and
// the HTML Canvas setLineDash API. Non-positive entries are invalid and
// cause dashing to be disabled rather than producing degenerate output.
func normalizeDashes(dashes []float64) []float64 {
	if len(dashes) == 0 {
		return nil
	}
	pattern := make([]float64, len(dashes))
	for i, d := range dashes {
		if math.IsNaN(d) || d <= 0 {
			return nil
		}
		pattern[i] = d
	}
	if len(pattern) == 1 {
		pattern = append(pattern, pattern[0])
	}
	return pattern
}

// dashOnSegments computes the painted arc-length intervals of one
// flattened contour.
//
// The dash pattern repeats with period equal to the sum of its entries;
// even-indexed entries are "on" (painted) intervals and odd-indexed
// entries are gaps. offset shifts the pattern phase at the start of the
// contour and may be negative or larger than one period.
//
// For open contours the pattern simply starts at arc length zero with
// the given phase, as in Cairo and PostScript. For closed contours the
// contour is treated as a loop: intervals are generated over the full
// real line, reduced modulo the contour length, and merged, so a dash
// crossing the start/end seam wraps around and the painted result joins
// up correctly instead of restarting the pattern at the seam.
func dashOnSegments(points []Point, pattern []float64, offset float64, closed bool) []dashSegment {
	var length float64
	for i := 1; i < len(points); i++ {
		length += points[i-1].Distance(points[i])
	}
	if length == 0 {
		return nil
	}

	var period float64
	for _, d := range pattern {
		period += d
	}

	// Reduce the phase into one period, preserving sign semantics:
	// a negative offset shifts the pattern backwards.
	phase := math.Mod(offset, period)
	if phase < 0 {
		phase += period
	}

	// Walk the pattern once from the reduced phase to collect the "on"
	// intervals inside one period. A positive offset advances the pattern
	// along the path, so the pattern position at arc length zero is
	// `phase`: the intervals generated here are expressed in path arc
	// length and shifted by +phase.
	type interval struct{ start, end float64 }
	var on []interval
	pos := phase
	for i, d := range pattern {
		if i%2 == 0 {
			on = append(on, interval{pos, pos + d})
		}
		pos += d
	}

	if !closed {
		// Iterate the periods whose intervals can intersect [0, length).
		kStart := int(math.Floor((-on[len(on)-1].end)/period)) - 1
		kEnd := int(math.Ceil((length-on[0].start)/period)) + 1
		var segments []dashSegment
		for k := kStart; k <= kEnd; k++ {
			shift := float64(k) * period
			for _, iv := range on {
				s := iv.start + shift
				e := iv.end + shift
				if e <= 0 || s >= length {
					continue
				}
				if s < 0 {
					s = 0
				}
				if e > length {
					e = length
				}
				if e > s {
					segments = append(segments, dashSegment{s, e})
				}
			}
		}
		return segments
	}

	// Closed contour: reduce every painted interval onto the loop. An
	// interval crossing the seam (arc length 0/length) is split into a
	// piece at the end of the loop and another at the beginning, whose
	// painted arc lengths together equal the clipped interval.
	kStart := int(math.Floor((-on[len(on)-1].end)/period)) - 1
	kEnd := int(math.Ceil((length-on[0].start)/period)) + 1
	var reduced []dashSegment
	for k := kStart; k <= kEnd; k++ {
		shift := float64(k) * period
		for _, iv := range on {
			rs, re := iv.start+shift, iv.end+shift
			if re <= 0 || rs >= length {
				continue
			}
			switch {
			case rs >= 0 && re <= length:
				// Fully contained in one pass around the loop.
				reduced = append(reduced, dashSegment{rs, re})
			case rs < 0 && re <= length:
				// Crosses the seam from before the start: the painted part
				// wraps to the beginning of the loop.
				reduced = append(reduced, dashSegment{0, re})
			case rs >= 0 && re > length:
				// Runs past the seam: the painted part wraps to the end.
				reduced = append(reduced, dashSegment{rs, length})
			default:
				// rs < 0 && re > length: spans the whole loop and crosses
				// the seam. Emit both pieces; overlaps with other periods
				// are resolved by mergeDashSegments.
				reduced = append(reduced,
					dashSegment{0, math.Min(re, length)},
					dashSegment{math.Max(rs, 0), length},
				)
			}
		}
	}

	// Pieces of dashes that wrap across the seam, or overlapping intervals
	// when the pattern period is shorter than the contour, are merged here
	// so the output has one contiguous subpath per painted region.
	return mergeDashSegments(reduced, length)
}

// mergeDashSegments sorts painted intervals on a closed loop and merges
// touching/overlapping ones. Intervals touching the seam are kept as two
// pieces because they must render as two subpaths sharing the seam point.
func mergeDashSegments(segments []dashSegment, length float64) []dashSegment {
	if len(segments) < 2 {
		return segments
	}
	// Simple insertion sort: the number of intervals per contour is small.
	for i := 1; i < len(segments); i++ {
		for j := i; j > 0 && segments[j-1].start > segments[j].start; j-- {
			segments[j-1], segments[j] = segments[j], segments[j-1]
		}
	}
	merged := segments[:1]
	for _, s := range segments[1:] {
		last := &merged[len(merged)-1]
		if s.start <= last.end {
			if s.end > last.end {
				last.end = s.end
			}
		} else {
			merged = append(merged, s)
		}
	}
	// Guard against floating point drift past the loop boundary.
	for i := range merged {
		if merged[i].end > length {
			merged[i].end = length
		}
	}
	return merged
}

// sliceContour returns the polyline of the contour spanning arc lengths
// [start, end]. Contour vertices that fall inside the interval are
// preserved, so bends keep their exact shape; only the two endpoints are
// interpolated onto their containing edges.
func sliceContour(points []Point, start, end float64) []Point {
	if end <= start {
		return nil
	}
	result := []Point{pointAt(points, start)}
	var acc float64
	for i := 1; i < len(points); i++ {
		// Vertex points[i] sits at arc length `next` along the contour.
		next := acc + points[i-1].Distance(points[i])
		// Include a vertex when it lies strictly inside the interval.
		// Boundary points are produced by pointAt below, which also handles
		// a boundary coinciding with a vertex without duplicating it.
		if next > start && next < end {
			result = append(result, points[i])
		}
		acc = next
		if acc >= end {
			break
		}
	}
	result = append(result, pointAt(points, end))
	return result
}

// pointAt returns the point at arc length s along the flattened contour.
func pointAt(points []Point, s float64) Point {
	if s <= 0 {
		return points[0]
	}
	var acc float64
	for i := 1; i < len(points); i++ {
		edge := points[i-1].Distance(points[i])
		if acc+edge >= s {
			if edge == 0 {
				return points[i]
			}
			return points[i-1].Interpolate(points[i], (s-acc)/edge)
		}
		acc += edge
	}
	return points[len(points)-1]
}

// contourLength returns the total arc length of a flattened contour.
func contourLength(points []Point) float64 {
	var length float64
	for i := 1; i < len(points); i++ {
		length += points[i-1].Distance(points[i])
	}
	return length
}

// isClosedContour reports whether the contour's last point coincides with
// its first, i.e. it was terminated by ClosePath (or otherwise returns to
// its starting point).
func isClosedContour(points []Point) bool {
	return len(points) >= 2 && points[0] == points[len(points)-1]
}

// dashPath splits flattened contours into painted subpaths according to
// the dash pattern and phase offset. The arc length of every emitted
// subpath matches the corresponding "on" entry of the pattern (clipped at
// contour ends), including when a dash wraps across the seam of a closed
// contour.
func dashPath(paths [][]Point, dashes []float64, offset float64) [][]Point {
	pattern := normalizeDashes(dashes)
	if pattern == nil {
		return paths
	}
	var result [][]Point
	for _, contour := range paths {
		if len(contour) < 2 || contourLength(contour) == 0 {
			continue
		}
		closed := isClosedContour(contour)
		segments := dashOnSegments(contour, pattern, offset, closed)
		for _, seg := range segments {
			sub := sliceContour(contour, seg.start, seg.end)
			if len(sub) >= 2 {
				result = append(result, sub)
			}
		}
	}
	return result
}

func rasterPath(paths [][]Point) raster.Path {
	var result raster.Path
	for _, path := range paths {
		var previous fixed.Point26_6
		for i, point := range path {
			f := point.Fixed()
			if i == 0 {
				result.Start(f)
			} else {
				dx := f.X - previous.X
				dy := f.Y - previous.Y
				if dx < 0 {
					dx = -dx
				}
				if dy < 0 {
					dy = -dy
				}
				if dx+dy > 8 {
					// TODO: this is a hack for cases where two points are
					// too close - causes rendering issues with joins / caps
					result.Add1(f)
				}
			}
			previous = f
		}
	}
	return result
}

// dashRasterPath converts painted subpaths into a raster path without the
// near-point filtering used by rasterPath. Dash endpoints must be retained
// exactly so that painted arc lengths stay within fixed-point precision of
// the requested pattern.
func dashRasterPath(paths [][]Point) raster.Path {
	var result raster.Path
	for _, path := range paths {
		if len(path) < 2 {
			continue
		}
		result.Start(path[0].Fixed())
		for _, point := range path[1:] {
			result.Add1(point.Fixed())
		}
	}
	return result
}

func dashed(path raster.Path, dashes []float64, offset float64) raster.Path {
	return dashRasterPath(dashPath(flattenPath(path), dashes, offset))
}
