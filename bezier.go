package gg

import "math"

// bezierTolerance is the default flatness tolerance used when approximating
// bezier curves with polylines. The adaptive subdivision below guarantees
// that the distance between the curve and the returned polyline never
// exceeds this value (in the same units as the control points, i.e. pixels
// when drawing on a Context).
const bezierTolerance = 0.5

// bezierMaxDepth bounds the recursion depth of the adaptive subdivision so
// that degenerate inputs (e.g. NaN or infinite coordinates) still terminate.
const bezierMaxDepth = 32

func quadratic(x0, y0, x1, y1, x2, y2, t float64) (x, y float64) {
	u := 1 - t
	a := u * u
	b := 2 * u * t
	c := t * t
	x = a*x0 + b*x1 + c*x2
	y = a*y0 + b*y1 + c*y2
	return
}

// QuadraticBezier approximates the quadratic bezier curve defined by the
// three control points with a polyline and returns its vertices, starting
// at (x0, y0) and ending at (x2, y2). The curve is adaptively subdivided
// until every segment deviates from the true curve by no more than
// bezierTolerance.
func QuadraticBezier(x0, y0, x1, y1, x2, y2 float64) []Point {
	points := make([]Point, 0, 17)
	points = append(points, Point{x0, y0})
	return flattenQuadratic(points, x0, y0, x1, y1, x2, y2, 0)
}

func flattenQuadratic(points []Point, x0, y0, x1, y1, x2, y2 float64, depth int) []Point {
	// For a quadratic bezier, B''(t) = 2(P0 - 2P1 + P2) is constant and the
	// distance between the curve and its chord is at most |B''| / 8, i.e.
	// |P0 - 2P1 + P2| / 4. The control points passed in already reflect the
	// parameter scaling of the sub-curve being considered.
	dx := x0 - 2*x1 + x2
	dy := y0 - 2*y1 + y2
	m := math.Hypot(dx, dy)
	if depth >= bezierMaxDepth || !(m > 4*bezierTolerance) || math.IsInf(m, 0) {
		// !(m > x) instead of m <= x so that NaN inputs terminate.
		return append(points, Point{x2, y2})
	}
	// de Casteljau split at t = 0.5
	x01 := (x0 + x1) / 2
	y01 := (y0 + y1) / 2
	x12 := (x1 + x2) / 2
	y12 := (y1 + y2) / 2
	xm := (x01 + x12) / 2
	ym := (y01 + y12) / 2
	points = flattenQuadratic(points, x0, y0, x01, y01, xm, ym, depth+1)
	points = flattenQuadratic(points, xm, ym, x12, y12, x2, y2, depth+1)
	return points
}

func cubic(x0, y0, x1, y1, x2, y2, x3, y3, t float64) (x, y float64) {
	u := 1 - t
	a := u * u * u
	b := 3 * u * u * t
	c := 3 * u * t * t
	d := t * t * t
	x = a*x0 + b*x1 + c*x2 + d*x3
	y = a*y0 + b*y1 + c*y2 + d*y3
	return
}

// CubicBezier approximates the cubic bezier curve defined by the four
// control points with a polyline and returns its vertices, starting at
// (x0, y0) and ending at (x3, y3). The curve is adaptively subdivided
// until every segment deviates from the true curve by no more than
// bezierTolerance.
func CubicBezier(x0, y0, x1, y1, x2, y2, x3, y3 float64) []Point {
	points := make([]Point, 0, 33)
	points = append(points, Point{x0, y0})
	return flattenCubic(points, x0, y0, x1, y1, x2, y2, x3, y3, 0)
}

func flattenCubic(points []Point, x0, y0, x1, y1, x2, y2, x3, y3 float64, depth int) []Point {
	// For a cubic bezier,
	//   B''(t) = 6((1-t)(P0 - 2P1 + P2) + t(P1 - 2P2 + P3))
	// so max|B''| <= 6 * max(|D1|, |D2|) and the distance between the curve
	// and its chord is at most max|B''| / 8, i.e. 3 * max(|D1|, |D2|) / 4.
	// The control points passed in already reflect the parameter scaling of
	// the sub-curve being considered.
	d1x := x0 - 2*x1 + x2
	d1y := y0 - 2*y1 + y2
	d2x := x1 - 2*x2 + x3
	d2y := y1 - 2*y2 + y3
	m := math.Max(math.Hypot(d1x, d1y), math.Hypot(d2x, d2y))
	if depth >= bezierMaxDepth || !(3*m > 4*bezierTolerance) || math.IsInf(m, 0) {
		// !(x > y) instead of x <= y so that NaN inputs terminate.
		return append(points, Point{x3, y3})
	}
	// de Casteljau split at t = 0.5
	x01 := (x0 + x1) / 2
	y01 := (y0 + y1) / 2
	x12 := (x1 + x2) / 2
	y12 := (y1 + y2) / 2
	x23 := (x2 + x3) / 2
	y23 := (y2 + y3) / 2
	x012 := (x01 + x12) / 2
	y012 := (y01 + y12) / 2
	x123 := (x12 + x23) / 2
	y123 := (y12 + y23) / 2
	xm := (x012 + x123) / 2
	ym := (y012 + y123) / 2
	points = flattenCubic(points, x0, y0, x01, y01, x012, y012, xm, ym, depth+1)
	points = flattenCubic(points, xm, ym, x123, y123, x23, y23, x3, y3, depth+1)
	return points
}
