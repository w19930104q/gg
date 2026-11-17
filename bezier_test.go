package gg

import (
	"math"
	"math/rand"
	"testing"
)

// pointSegmentDistance returns the distance from (px, py) to the segment
// (ax, ay)-(bx, by).
func pointSegmentDistance(px, py, ax, ay, bx, by float64) float64 {
	dx := bx - ax
	dy := by - ay
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// polylineDistance returns the distance from (x, y) to the nearest segment
// of the polyline.
func polylineDistance(points []Point, x, y float64) float64 {
	d := math.Inf(1)
	for i := 0; i+1 < len(points); i++ {
		a := points[i]
		b := points[i+1]
		if h := pointSegmentDistance(x, y, a.X, a.Y, b.X, b.Y); h < d {
			d = h
		}
	}
	return d
}

// maxQuadraticDeviation densely samples the true quadratic bezier curve and
// returns the maximum distance from any sampled point to the polyline.
func maxQuadraticDeviation(points []Point, x0, y0, x1, y1, x2, y2 float64) float64 {
	samples := 4096
	if n := 16 * len(points); n > samples {
		samples = n
	}
	maxd := 0.0
	for i := 0; i <= samples; i++ {
		t := float64(i) / float64(samples)
		x, y := quadratic(x0, y0, x1, y1, x2, y2, t)
		if d := polylineDistance(points, x, y); d > maxd {
			maxd = d
		}
	}
	return maxd
}

// maxCubicDeviation densely samples the true cubic bezier curve and returns
// the maximum distance from any sampled point to the polyline.
func maxCubicDeviation(points []Point, x0, y0, x1, y1, x2, y2, x3, y3 float64) float64 {
	samples := 4096
	if n := 16 * len(points); n > samples {
		samples = n
	}
	maxd := 0.0
	for i := 0; i <= samples; i++ {
		t := float64(i) / float64(samples)
		x, y := cubic(x0, y0, x1, y1, x2, y2, x3, y3, t)
		if d := polylineDistance(points, x, y); d > maxd {
			maxd = d
		}
	}
	return maxd
}

func TestQuadraticBezierFlattening(t *testing.T) {
	cases := []struct {
		name                   string
		x0, y0, x1, y1, x2, y2 float64
		maxPoints              int
	}{
		{"sharp spike", 0, 0, 50, 100, 100, 0, 64},
		{"acute hairpin", 0, 0, 100, 1, 0, 0, 64},
		{"nearly collinear", 0, 0, 50, 0.001, 100, 0, 16},
		{"straight line", 0, 0, 1, 1, 2, 2, 4},
		{"degenerate point", 5, 5, 5, 5, 5, 5, 4},
		{"duplicated control", 0, 0, 0, 0, 100, 50, 64},
		{"tiny curve", 0, 0, 0.001, 0.002, 0.002, 0, 4},
		{"large curve", 0, 0, 5000, 10000, 10000, 0, 1024},
	}
	for _, c := range cases {
		points := QuadraticBezier(c.x0, c.y0, c.x1, c.y1, c.x2, c.y2)
		if len(points) < 2 {
			t.Errorf("%s: got %d points, want at least 2", c.name, len(points))
			continue
		}
		if len(points) > c.maxPoints {
			t.Errorf("%s: got %d points, want at most %d", c.name, len(points), c.maxPoints)
		}
		if first := points[0]; first.X != c.x0 || first.Y != c.y0 {
			t.Errorf("%s: first point = %v, want (%v, %v)", c.name, first, c.x0, c.y0)
		}
		if last := points[len(points)-1]; last.X != c.x2 || last.Y != c.y2 {
			t.Errorf("%s: last point = %v, want (%v, %v)", c.name, last, c.x2, c.y2)
		}
		d := maxQuadraticDeviation(points, c.x0, c.y0, c.x1, c.y1, c.x2, c.y2)
		if d > bezierTolerance+1e-9 {
			t.Errorf("%s: max deviation %v exceeds tolerance %v", c.name, d, bezierTolerance)
		}
	}
}

func TestCubicBezierFlattening(t *testing.T) {
	cases := []struct {
		name                           string
		x0, y0, x1, y1, x2, y2, x3, y3 float64
		maxPoints                      int
	}{
		{"s-curve", 0, 0, 30, 100, 70, -100, 100, 0, 128},
		{"sharp corner", 0, 0, 0, 100, 0, 100, 100, 100, 128},
		{"cusp", 0, 0, 100, 0, -100, 0, 0, 0, 256},
		{"loop", 0, 0, 200, 0, -100, 100, 100, 0, 256},
		{"nearly collinear", 0, 0, 33.3, 0.001, 66.7, -0.001, 100, 0, 16},
		{"straight line", 0, 0, 1, 1, 2, 2, 3, 3, 4},
		{"degenerate point", 5, 5, 5, 5, 5, 5, 5, 5, 4},
		{"duplicated controls", 0, 0, 0, 0, 100, 100, 100, 100, 64},
		{"tiny curve", 0, 0, 0.001, 0.002, 0.002, -0.001, 0.003, 0, 4},
		{"large curve", 0, 0, 3000, 9000, 7000, -9000, 10000, 0, 1024},
	}
	for _, c := range cases {
		points := CubicBezier(c.x0, c.y0, c.x1, c.y1, c.x2, c.y2, c.x3, c.y3)
		if len(points) < 2 {
			t.Errorf("%s: got %d points, want at least 2", c.name, len(points))
			continue
		}
		if len(points) > c.maxPoints {
			t.Errorf("%s: got %d points, want at most %d", c.name, len(points), c.maxPoints)
		}
		if first := points[0]; first.X != c.x0 || first.Y != c.y0 {
			t.Errorf("%s: first point = %v, want (%v, %v)", c.name, first, c.x0, c.y0)
		}
		if last := points[len(points)-1]; last.X != c.x3 || last.Y != c.y3 {
			t.Errorf("%s: last point = %v, want (%v, %v)", c.name, last, c.x3, c.y3)
		}
		d := maxCubicDeviation(points, c.x0, c.y0, c.x1, c.y1, c.x2, c.y2, c.x3, c.y3)
		if d > bezierTolerance+1e-9 {
			t.Errorf("%s: max deviation %v exceeds tolerance %v", c.name, d, bezierTolerance)
		}
	}
}

// TestQuadraticBezierRandomized checks the deviation guarantee over a fixed
// pseudo-random set of control points, including nearly-collinear and
// sharp-corner configurations.
func TestQuadraticBezierRandomized(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < 500; i++ {
		scale := math.Pow(10, rnd.Float64()*6-3) // 1e-3 .. 1e3
		x0 := (rnd.Float64()*2 - 1) * scale
		y0 := (rnd.Float64()*2 - 1) * scale
		x1 := (rnd.Float64()*2 - 1) * scale
		y1 := (rnd.Float64()*2 - 1) * scale
		x2 := (rnd.Float64()*2 - 1) * scale
		y2 := (rnd.Float64()*2 - 1) * scale
		if i%3 == 0 {
			// nearly collinear control points
			y0 *= 1e-6
			y1 *= 1e-6
			y2 *= 1e-6
		}
		points := QuadraticBezier(x0, y0, x1, y1, x2, y2)
		if len(points) < 2 || len(points) > 1<<20 {
			t.Fatalf("case %d: unreasonable point count %d", i, len(points))
		}
		d := maxQuadraticDeviation(points, x0, y0, x1, y1, x2, y2)
		if d > bezierTolerance+1e-9 {
			t.Fatalf("case %d: max deviation %v exceeds tolerance %v", i, d, bezierTolerance)
		}
	}
}

// TestCubicBezierRandomized checks the deviation guarantee over a fixed
// pseudo-random set of control points, including nearly-collinear and
// sharp-corner configurations.
func TestCubicBezierRandomized(t *testing.T) {
	rnd := rand.New(rand.NewSource(2))
	for i := 0; i < 500; i++ {
		scale := math.Pow(10, rnd.Float64()*6-3) // 1e-3 .. 1e3
		var p [8]float64
		for j := range p {
			p[j] = (rnd.Float64()*2 - 1) * scale
		}
		if i%3 == 0 {
			// nearly collinear control points
			p[1] *= 1e-6
			p[3] *= 1e-6
			p[5] *= 1e-6
			p[7] *= 1e-6
		}
		points := CubicBezier(p[0], p[1], p[2], p[3], p[4], p[5], p[6], p[7])
		if len(points) < 2 || len(points) > 1<<20 {
			t.Fatalf("case %d: unreasonable point count %d", i, len(points))
		}
		d := maxCubicDeviation(points, p[0], p[1], p[2], p[3], p[4], p[5], p[6], p[7])
		if d > bezierTolerance+1e-9 {
			t.Fatalf("case %d: max deviation %v exceeds tolerance %v", i, d, bezierTolerance)
		}
	}
}

// TestBezierDegenerateInput ensures non-finite control points terminate
// instead of recursing forever.
func TestBezierDegenerateInput(t *testing.T) {
	nan := math.NaN()
	inf := math.Inf(1)
	if n := len(QuadraticBezier(0, 0, nan, 0, 1, 1)); n < 2 || n > 1<<20 {
		t.Errorf("quadratic with NaN: got %d points", n)
	}
	if n := len(QuadraticBezier(0, 0, inf, 0, 1, 1)); n < 2 || n > 1<<20 {
		t.Errorf("quadratic with Inf: got %d points", n)
	}
	if n := len(CubicBezier(0, 0, nan, 0, 0, nan, 1, 1)); n < 2 || n > 1<<20 {
		t.Errorf("cubic with NaN: got %d points", n)
	}
	if n := len(CubicBezier(0, 0, inf, 0, 0, inf, 1, 1)); n < 2 || n > 1<<20 {
		t.Errorf("cubic with Inf: got %d points", n)
	}
}
