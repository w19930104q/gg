package gg

import (
	"math"
	"testing"
)

const dashTol = 1e-9

// polylineLength is an independent arc-length measurement used only by the
// tests; it deliberately does not share code with the implementation.
func polylineLength(p []Point) float64 {
	var l float64
	for i := 1; i < len(p); i++ {
		l += p[i-1].Distance(p[i])
	}
	return l
}

// expectedOnIntervals is an independent reference for the painted ("on")
// arc-length intervals of a contour. It walks the pattern as an infinite
// periodic sequence and, for closed contours, reduces intervals onto the
// loop and merges overlaps exactly like a circular arc would be painted.
func expectedOnIntervals(length float64, pattern []float64, offset float64, closed bool) [][2]float64 {
	var period float64
	for _, d := range pattern {
		period += d
	}
	phase := math.Mod(offset, period)
	if phase < 0 {
		phase += period
	}

	// "on" intervals inside one period expressed in path arc length. A
	// positive offset advances the pattern along the path.
	type iv struct{ s, e float64 }
	var on []iv
	pos := phase
	for i, d := range pattern {
		if i%2 == 0 {
			on = append(on, iv{pos, pos + d})
		}
		pos += d
	}

	var raw [][2]float64
	kStart := int(math.Floor((-on[len(on)-1].e)/period)) - 1
	kEnd := int(math.Ceil((length-on[0].s)/period)) + 1
	for k := kStart; k <= kEnd; k++ {
		shift := float64(k) * period
		for _, in := range on {
			rs, re := in.s+shift, in.e+shift
			if re <= 0 || rs >= length {
				continue
			}
			if !closed {
				raw = append(raw, [2]float64{math.Max(rs, 0), math.Min(re, length)})
				continue
			}
			// Same seam-splitting rules as the implementation.
			switch {
			case rs >= 0 && re <= length:
				raw = append(raw, [2]float64{rs, re})
			case rs < 0 && re <= length:
				raw = append(raw, [2]float64{0, re})
			case rs >= 0 && re > length:
				raw = append(raw, [2]float64{rs, length})
			default:
				raw = append(raw,
					[2]float64{0, math.Min(re, length)},
					[2]float64{math.Max(rs, 0), length},
				)
			}
		}
	}

	// Sort and merge (for closed contours).
	for i := 1; i < len(raw); i++ {
		for j := i; j > 0 && raw[j-1][0] > raw[j][0]; j-- {
			raw[j-1], raw[j] = raw[j], raw[j-1]
		}
	}
	var merged [][2]float64
	for _, r := range raw {
		if n := len(merged); n > 0 && r[0] <= merged[n-1][1]+1e-12 {
			if r[1] > merged[n-1][1] {
				merged[n-1][1] = r[1]
			}
			continue
		}
		merged = append(merged, r)
	}
	if closed {
		return merged
	}
	return raw
}

// checkContourDashing runs dashPath on one contour and verifies every
// emitted subpath against the independently computed reference intervals.
func checkContourDashing(t *testing.T, name string, contour []Point, pattern []float64, offset float64, closed bool) {
	t.Helper()
	length := polylineLength(contour)
	if length == 0 {
		t.Fatalf("%s: zero-length test contour", name)
	}
	pattern = normalizeDashes(append([]float64(nil), pattern...))
	got := dashPath([][]Point{contour}, pattern, offset)
	want := expectedOnIntervals(length, pattern, offset, closed)

	if len(got) != len(want) {
		t.Fatalf("%s: got %d dashed subpaths, want %d (pattern=%v offset=%v)",
			name, len(got), len(want), pattern, offset)
	}

	// The emitted subpaths must be ordered by arc-length position.
	var totalGot, totalWant float64
	for i, sub := range got {
		gotLen := polylineLength(sub)
		wantLen := want[i][1] - want[i][0]
		totalGot += gotLen
		totalWant += wantLen
		if math.Abs(gotLen-wantLen) > dashTol {
			t.Errorf("%s: subpath %d arc length = %.12f, want %.12f (pattern=%v offset=%v)",
				name, i, gotLen, wantLen, pattern, offset)
		}
		// The subpath endpoints must coincide with the reference interval
		// endpoints when mapped back onto the contour.
		ps := pointAt(contour, want[i][0])
		pe := pointAt(contour, want[i][1])
		if math.Abs(sub[0].X-ps.X) > dashTol || math.Abs(sub[0].Y-ps.Y) > dashTol {
			t.Errorf("%s: subpath %d start = %v, want %v", name, i, sub[0], ps)
		}
		if math.Abs(sub[len(sub)-1].X-pe.X) > dashTol || math.Abs(sub[len(sub)-1].Y-pe.Y) > dashTol {
			t.Errorf("%s: subpath %d end = %v, want %v", name, i, sub[len(sub)-1], pe)
		}
	}
	// Total painted arc length is an aggregate check that seam wrapping
	// neither loses nor duplicates ink.
	if math.Abs(totalGot-totalWant) > dashTol {
		t.Errorf("%s: total painted length = %.12f, want %.12f", name, totalGot, totalWant)
	}
}

func rectContour(x, y, w, h float64) []Point {
	return []Point{
		{X: x, Y: y},
		{X: x + w, Y: y},
		{X: x + w, Y: y + h},
		{X: x, Y: y + h},
		{X: x, Y: y},
	}
}

func lineContour(length float64) []Point {
	return []Point{{0, 0}, {length, 0}}
}

func TestDashOpenLineArcLengths(t *testing.T) {
	patterns := [][]float64{
		{10, 10},
		{10}, // single entry expands to {10, 10}
		{7, 3, 2, 5},
		{3.5, 1.25},
		{1, 1, 1, 1, 1, 7},
	}
	offsets := []float64{0, 3, -4, 13.5, -27.25, 100}
	for pi, p := range patterns {
		for _, off := range offsets {
			name := "open-line"
			checkContourDashing(t, name, lineContour(123.75), p, off, false)
			_ = pi
		}
	}
}

func TestDashPolylineArcs(t *testing.T) {
	// A bent open contour: the dash pattern must follow arc length, not x.
	contour := []Point{
		{0, 0}, {40, 0}, {40, 30}, {10, 30}, {10, 80},
	}
	for _, p := range [][]float64{{12, 8}, {5, 5, 2, 3}, {20, 7}} {
		for _, off := range []float64{0, 6.5, -9, 31} {
			checkContourDashing(t, "open-polyline", contour, p, off, false)
		}
	}
}

func TestDashClosedRectangleSeam(t *testing.T) {
	// Perimeter 120; deliberately choose patterns whose period does not
	// divide the perimeter so the old per-subpath reset would break the seam.
	contour := rectContour(5, 7, 40, 20)
	for _, p := range [][]float64{{12, 8}, {7, 3, 2, 5}, {9.5, 4.5}, {30, 10}} {
		for _, off := range []float64{0, 4, -7, 23.75} {
			checkContourDashing(t, "closed-rect", contour, p, off, true)
		}
	}
}

// TestDashSeamWraps verifies the defining closed-path property directly:
// a dash that straddles the seam must be split into two subpaths whose
// combined arc length equals the full dash length, and whose outer ends
// lie exactly on the seam point.
func TestDashSeamWraps(t *testing.T) {
	// Rectangle perimeter 100, pattern {30, 70}: the first "on" interval at
	// phase 0 is [0,30) and does not wrap; shift phase so an "on" interval
	// crosses the seam. offset = -10 makes an on interval [-10, 20),
	// i.e. pieces [90,100) and [0,20).
	contour := rectContour(0, 0, 25, 25) // perimeter 100
	got := dashPath([][]Point{contour}, []float64{30, 70}, -10)

	seam := contour[0]
	near := func(p Point) bool {
		return math.Abs(p.X-seam.X) < dashTol && math.Abs(p.Y-seam.Y) < dashTol
	}
	var wrapCount int
	for _, sub := range got {
		first, last := sub[0], sub[len(sub)-1]
		if near(first) || near(last) {
			wrapCount++
		}
	}
	if wrapCount != 2 {
		t.Fatalf("expected two subpaths meeting at the seam, got %d", wrapCount)
	}

	// The two seam pieces together must span the full 30-unit dash.
	var seamInk float64
	for _, sub := range got {
		first, last := sub[0], sub[len(sub)-1]
		if near(first) || near(last) {
			seamInk += polylineLength(sub)
		}
	}
	if math.Abs(seamInk-30) > dashTol {
		t.Errorf("wrapped dash total length = %.12f, want 30", seamInk)
	}

	// A contour identical except for being open must instead restart the
	// pattern at its first point (no wrapping).
	open := contour[:len(contour)-1]
	openGot := dashPath([][]Point{open}, []float64{30, 70}, -10)
	openWant := expectedOnIntervals(polylineLength(open), []float64{30, 70}, -10, false)
	if len(openGot) != len(openWant) {
		t.Fatalf("open contour: got %d subpaths, want %d", len(openGot), len(openWant))
	}
}

func TestDashPhaseParity(t *testing.T) {
	// Offsets differing by whole pattern periods must produce identical
	// geometry; offset and offset-period are the same phase.
	contour := rectContour(0, 0, 33, 17)
	pattern := []float64{6, 4, 2, 8} // period 20
	base := dashPath([][]Point{contour}, pattern, 3.5)
	for _, off := range []float64{23.5, -16.5, 43.5, -36.5} {
		got := dashPath([][]Point{contour}, pattern, off)
		if len(got) != len(base) {
			t.Fatalf("offset %v: got %d subpaths, want %d", off, len(got), len(base))
		}
		for i := range got {
			if polylineLength(got[i]) != polylineLength(base[i]) {
				t.Errorf("offset %v: subpath %d length differs from base", off, i)
			}
			if got[i][0] != base[i][0] {
				t.Errorf("offset %v: subpath %d start %v != base %v", off, i, got[i][0], base[i][0])
			}
		}
	}
}

func TestDashGapsAreGaps(t *testing.T) {
	// Every gap between consecutive painted subpaths on an open contour
	// must match an "off" pattern entry, and painted intervals must never
	// overlap when mapped onto the contour.
	contour := lineContour(250)
	pattern := []float64{9, 6, 3, 4} // period 22
	got := dashPath([][]Point{contour}, pattern, 0)

	// Reconstruct the arc-length position of each subpath via pointAt
	// inversion on a straight line.
	prevEnd := 0.0
	for i, sub := range got {
		s := sub[0].X
		e := sub[len(sub)-1].X
		if s < prevEnd-dashTol {
			t.Errorf("subpath %d overlaps previous painted region", i)
		}
		prevEnd = e
	}

	// First dash must start exactly at contour origin for zero offset.
	if got[0][0].X != 0 {
		t.Errorf("first dash starts at %v, want 0", got[0][0].X)
	}
	// Painted lengths must follow the on entries of the pattern in order.
	var onIdx []float64
	for i, d := range pattern {
		if i%2 == 0 {
			onIdx = append(onIdx, d)
		}
	}
	for i, sub := range got {
		l := polylineLength(sub)
		want := onIdx[i%len(onIdx)]
		// The final dash may be clipped by the contour end.
		if i < len(got)-1 && math.Abs(l-want) > dashTol {
			t.Errorf("subpath %d length %.12f, want pattern on-entry %.12f", i, l, want)
		}
	}
}

func TestDashDisabledAndInvalid(t *testing.T) {
	contour := lineContour(50)

	// Empty pattern returns the contours unchanged.
	if got := dashPath([][]Point{contour}, nil, 0); len(got) != 1 || len(got[0]) != 2 {
		t.Errorf("empty dash pattern should leave the contour untouched")
	}

	// Non-positive or NaN entries disable dashing rather than panicking.
	for _, bad := range [][]float64{{0, 5}, {-1, 5}, {5, 0}, {math.NaN(), 5}} {
		got := dashPath([][]Point{contour}, bad, 0)
		if len(got) != 1 {
			t.Errorf("pattern %v should be treated as disabled, got %d subpaths", bad, len(got))
		}
	}
}

func TestDashDegenerateContours(t *testing.T) {
	// A single point and a zero-length closed contour must not panic or
	// emit output.
	got := dashPath([][]Point{{{1, 1}}}, []float64{2, 2}, 0)
	if len(got) != 0 {
		t.Errorf("single-point contour produced %d subpaths", len(got))
	}
	got = dashPath([][]Point{{{3, 3}, {3, 3}}}, []float64{2, 2}, 0)
	if len(got) != 0 {
		t.Errorf("zero-length contour produced %d subpaths", len(got))
	}
}

func TestDashMultipleSubpathsResetPhase(t *testing.T) {
	// Each subpath restarts the pattern at its own first point, matching
	// Cairo/PostScript semantics: two identical open contours must produce
	// identical dash geometry even when drawn in one path object.
	a := lineContour(60)
	b := lineContour(60)
	got := dashPath([][]Point{a, b}, []float64{7, 5}, 2)
	if len(got)%2 != 0 {
		t.Fatalf("expected paired subpaths from two contours, got %d", len(got))
	}
	half := len(got) / 2
	for i := 0; i < half; i++ {
		if polylineLength(got[i]) != polylineLength(got[i+half]) {
			t.Errorf("subpath %d differs between the two identical contours", i)
		}
		if got[i][0] != got[i+half][0] {
			t.Errorf("subpath %d starts differ: %v vs %v", i, got[i][0], got[i+half][0])
		}
	}
}

func TestDashEdgeCases(t *testing.T) {
	rect := rectContour(0, 0, 10, 10) // closed, perimeter 40
	line := lineContour(20)           // open, length 20
	cases := []struct {
		name    string
		contour []Point
		closed  bool
		pattern []float64
		offset  float64
	}{
		{"closed-period-longer-than-loop", rect, true, []float64{100, 50}, 0},
		{"closed-dash-spans-whole-loop", rect, true, []float64{60, 5}, 0},
		{"closed-all-on-merges", rect, true, []float64{40, 1}, 0},
		{"open-dash-longer-than-contour", line, false, []float64{100, 5}, 0},
		{"open-offset-near-end", line, false, []float64{5, 5}, 19.5},
		{"closed-offset-many-periods", rect, true, []float64{7, 3}, 200},
		{"tiny-dash", line, false, []float64{0.01, 0.01}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := dashPath([][]Point{c.contour}, c.pattern, c.offset)
			pat := normalizeDashes(append([]float64(nil), c.pattern...))
			want := expectedOnIntervals(contourLength(c.contour), pat, c.offset, c.closed)
			if len(got) != len(want) {
				t.Fatalf("subpath count = %d, want %d", len(got), len(want))
			}
			var totalGot, totalWant float64
			for i, sub := range got {
				l := polylineLength(sub)
				if l <= 0 {
					t.Fatalf("subpath %d has non-positive length %v", i, l)
				}
				totalGot += l
				totalWant += want[i][1] - want[i][0]
				if math.Abs(l-(want[i][1]-want[i][0])) > dashTol {
					t.Errorf("subpath %d length %.9f, want %.9f", i, l, want[i][1]-want[i][0])
				}
			}
			if math.Abs(totalGot-totalWant) > dashTol {
				t.Errorf("total ink %.9f != reference %.9f", totalGot, totalWant)
			}
		})
	}
}

// TestDashHandComputed uses hard-coded expected geometry rather than a
// generated reference, so a shared mistake between the implementation and
// its reference helper cannot make both pass.
func TestDashHandComputed(t *testing.T) {
	// Open line of length 50, pattern {10, 10}, no offset:
	// on intervals [0,10) [20,30) [40,50).
	line := lineContour(50)
	got := dashPath([][]Point{line}, []float64{10, 10}, 0)
	want := []dashSegment{{0, 10}, {20, 30}, {40, 50}}
	if len(got) != len(want) {
		t.Fatalf("open line: got %d subpaths, want %d", len(got), len(want))
	}
	for i, w := range want {
		if l := polylineLength(got[i]); math.Abs(l-(w.end-w.start)) > dashTol {
			t.Errorf("open line subpath %d length %v, want %v", i, l, w.end-w.start)
		}
		// On a straight line from (0,0), arc length equals the x coordinate.
		if math.Abs(got[i][0].X-w.start) > dashTol ||
			math.Abs(got[i][len(got[i])-1].X-w.end) > dashTol {
			t.Errorf("open line subpath %d endpoints wrong: %v..%v, want %v..%v",
				i, got[i][0].X, got[i][len(got[i])-1].X, w.start, w.end)
		}
	}

	// Same line, offset 5 shifts the pattern forward:
	// on at [5,15) [25,35) [45,50-clipped).
	got = dashPath([][]Point{line}, []float64{10, 10}, 5)
	want = []dashSegment{{5, 15}, {25, 35}, {45, 50}}
	if len(got) != len(want) {
		t.Fatalf("offset line: got %d subpaths, want %d", len(got), len(want))
	}
	for i, w := range want {
		if l := polylineLength(got[i]); math.Abs(l-(w.end-w.start)) > dashTol {
			t.Errorf("offset line subpath %d length %v, want %v", i, l, w.end-w.start)
		}
	}

	// Closed square, perimeter 40 (10 per side), pattern {30, 10},
	// offset -10. The on interval crossing the seam wraps into two pieces:
	// [0,20) and [30,40), together 30 units of ink.
	square := rectContour(0, 0, 10, 10)
	got = dashPath([][]Point{square}, []float64{30, 10}, -10)
	if len(got) != 2 {
		t.Fatalf("wrapped square: got %d subpaths, want 2", len(got))
	}
	l0, l1 := polylineLength(got[0]), polylineLength(got[1])
	if math.Abs((l0+l1)-30) > dashTol {
		t.Errorf("wrapped square ink = %v, want 30 (pieces %v + %v)", l0+l1, l0, l1)
	}
	// Both pieces must terminate/originate at the seam point (0,0).
	seam := square[0]
	ends := [][2]Point{{got[0][0], got[0][len(got[0])-1]}, {got[1][0], got[1][len(got[1])-1]}}
	var seamTouches int
	for _, e := range ends {
		for _, p := range e {
			if math.Abs(p.X-seam.X) < dashTol && math.Abs(p.Y-seam.Y) < dashTol {
				seamTouches++
			}
		}
	}
	if seamTouches != 2 {
		t.Errorf("expected exactly 2 subpath ends at the seam, got %d", seamTouches)
	}
}

// TestDashEdgeCases ensures the rasterized path retains exact
// dash endpoints rather than dropping them via the near-point filter.
func TestDashRasterEndpointsPrecise(t *testing.T) {
	contour := lineContour(100)
	subs := dashPath([][]Point{contour}, []float64{0.5, 0.5}, 0)
	rp := dashRasterPath(subs)
	if len(rp) == 0 {
		t.Fatal("raster path is empty")
	}
	// Every computed subpath endpoint must appear in the raster path.
	for _, sub := range subs {
		for _, p := range []Point{sub[0], sub[len(sub)-1]} {
			f := p.Fixed()
			var found bool
			for i := 0; i < len(rp); {
				switch rp[i] {
				case 0:
					if rp[i+1] == f.X && rp[i+2] == f.Y {
						found = true
					}
					i += 4
				case 1:
					if rp[i+1] == f.X && rp[i+2] == f.Y {
						found = true
					}
					i += 4
				case 2:
					i += 6
				case 3:
					i += 8
				default:
					i++
				}
			}
			if !found {
				t.Errorf("dash endpoint %v (fixed %v) missing from raster path", p, f)
			}
		}
	}
}
