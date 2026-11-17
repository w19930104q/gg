package gg

import (
	"image"
	"testing"
)

// countDiff counts pixels that differ between two equally sized images.
func countDiff(a, b *image.RGBA) int {
	n := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

func newClipTestContext() *Context {
	dc := NewContext(200, 200)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	return dc
}

// strokeReference renders the stroke used by the tests without any clipping
// or state stack involved.
func strokeReference() *image.RGBA {
	dc := newClipTestContext()
	dc.SetLineWidth(4)
	dc.DrawRectangle(20, 30, 60, 40)
	dc.Stroke()
	return dc.Image().(*image.RGBA)
}

// TestNestedSaveClipRestoreStroke is the reported regression: a clip is
// established inside nested Push/Scale ... Pop sections. After the Pops the
// transformation must be fully restored and a subsequent stroke must render
// exactly as it would without any clipping (the clip region covers the whole
// canvas, so clipping must be a no-op on the output).
func TestNestedSaveClipRestoreStroke(t *testing.T) {
	dc := newClipTestContext()

	dc.Push()
	dc.Push()
	dc.Scale(2, 2)
	dc.DrawRectangle(0, 0, 100, 100)
	dc.Clip()
	dc.Pop()
	dc.Pop()

	if dc.mask == nil {
		t.Fatal("clipping mask did not survive Push/Pop")
	}
	if dc.matrix != Identity() {
		t.Fatalf("matrix not restored after Pop: %+v", dc.matrix)
	}
	if len(dc.stack) != 0 {
		t.Fatalf("state stack not empty after matching Pops: depth=%d", len(dc.stack))
	}

	dc.SetLineWidth(4)
	dc.DrawRectangle(20, 30, 60, 40)
	dc.Stroke()

	if n := countDiff(strokeReference(), dc.Image().(*image.RGBA)); n != 0 {
		t.Fatalf("stroke drifted after nested clip: %d pixels differ from no-clip reference", n)
	}
}

// TestClipUnderScaledTransform verifies that a clip path built under a scaling
// transform is rasterized in device space correctly: its mask must match the
// same region expressed directly in identity-matrix coordinates.
func TestClipUnderScaledTransform(t *testing.T) {
	// Reference clip region (20,20)-(180,180) built without transforms.
	ref := newClipTestContext()
	ref.DrawRectangle(20, 20, 160, 160)
	ref.Clip()
	ref.SetLineWidth(4)
	ref.DrawRectangle(20, 30, 60, 40)
	ref.Stroke()

	// Same region: DrawRectangle(10,10,80,80) under Scale(2,2).
	dc := newClipTestContext()
	dc.Push()
	dc.Scale(2, 2)
	dc.DrawRectangle(10, 10, 80, 80)
	dc.Clip()
	dc.Pop()
	dc.SetLineWidth(4)
	dc.DrawRectangle(20, 30, 60, 40)
	dc.Stroke()

	if n := countDiff(ref.Image().(*image.RGBA), dc.Image().(*image.RGBA)); n != 0 {
		t.Fatalf("scaled clip produced %d differing pixels vs identity clip", n)
	}
}

// TestPathSurvivesPushClipPop tracks the context state step by step and
// verifies that path segments existing before Push remain present and
// extendable after the matching Pop, even though the pushed section built
// and consumed its own path via Clip.
func TestPathSurvivesPushClipPop(t *testing.T) {
	dc := newClipTestContext()
	dc.SetLineWidth(4)
	dc.MoveTo(20, 30)

	if len(dc.strokePath) != 4 || !dc.hasCurrent {
		t.Fatalf("unexpected initial path state: len=%d hasCurrent=%v", len(dc.strokePath), dc.hasCurrent)
	}

	dc.Push()
	if len(dc.stack) != 1 {
		t.Fatalf("stack depth after Push = %d, want 1", len(dc.stack))
	}
	// The pushed snapshot must hold its own copy of the path so that path
	// construction below cannot overwrite the outer segments.
	if len(dc.stack[0].strokePath) != 4 {
		t.Fatalf("snapshot did not capture pre-Push path: len=%d", len(dc.stack[0].strokePath))
	}

	dc.Scale(2, 2)
	dc.DrawRectangle(0, 0, 100, 100)
	dc.Clip() // consumes the inner path
	if len(dc.strokePath) != 0 || dc.hasCurrent {
		t.Fatalf("Clip did not consume the inner path: len=%d hasCurrent=%v", len(dc.strokePath), dc.hasCurrent)
	}
	if dc.mask == nil {
		t.Fatal("clip mask missing after Clip")
	}

	dc.Pop()
	// Matrix restored, mask retained, pre-Push path restored.
	if dc.matrix != Identity() {
		t.Fatalf("matrix not restored: %+v", dc.matrix)
	}
	if dc.mask == nil {
		t.Fatal("clip mask did not survive Pop")
	}
	if len(dc.strokePath) != 4 || !dc.hasCurrent {
		t.Fatalf("pre-Push path lost after Pop: len=%d hasCurrent=%v", len(dc.strokePath), dc.hasCurrent)
	}
	if dc.start != (Point{20, 30}) || dc.current != (Point{20, 30}) {
		t.Fatalf("current point not restored: start=%v current=%v", dc.start, dc.current)
	}

	// The restored path remains extendable and strokes identically to the
	// no-clip equivalent.
	dc.LineTo(80, 30)
	dc.LineTo(80, 70)
	dc.LineTo(20, 70)
	dc.ClosePath()
	dc.Stroke()

	ref := newClipTestContext()
	ref.SetLineWidth(4)
	ref.MoveTo(20, 30)
	ref.LineTo(80, 30)
	ref.LineTo(80, 70)
	ref.LineTo(20, 70)
	ref.ClosePath()
	ref.Stroke()

	if n := countDiff(ref.Image().(*image.RGBA), dc.Image().(*image.RGBA)); n != 0 {
		t.Fatalf("stroke on restored path drifted: %d differing pixels", n)
	}
}

// TestPathBuiltInsidePushSurvivesPop guards the DrawPoint use case: a path
// constructed inside a Push section and left unconsumed must still be
// available after Pop so it can be filled/stroked.
func TestPathBuiltInsidePushSurvivesPop(t *testing.T) {
	dc := newClipTestContext()
	dc.Push()
	dc.DrawRectangle(20, 30, 60, 40)
	dc.Pop()
	if len(dc.strokePath) == 0 || !dc.hasCurrent {
		t.Fatalf("path built inside Push lost after Pop: len=%d hasCurrent=%v", len(dc.strokePath), dc.hasCurrent)
	}
	dc.SetLineWidth(4)
	dc.Stroke()

	if n := countDiff(strokeReference(), dc.Image().(*image.RGBA)); n != 0 {
		t.Fatalf("stroke on inner-built path drifted: %d differing pixels", n)
	}
}

// TestScaledClipDoesNotDriftStroke reproduces the reported symptom directly:
// a path is started before Push, a scaled clip is established inside the
// pushed section, and after Pop the path is continued and stroked. The line
// width and position of the resulting stroke must match the no-clip
// reference exactly (the clip covers the full canvas).
func TestScaledClipDoesNotDriftStroke(t *testing.T) {
	dc := newClipTestContext()
	dc.SetLineWidth(4)
	dc.MoveTo(20, 30)

	dc.Push()
	dc.Scale(2, 2)
	dc.DrawRectangle(0, 0, 100, 100)
	dc.Clip()
	dc.Pop()

	dc.LineTo(80, 30)
	dc.LineTo(80, 70)
	dc.LineTo(20, 70)
	dc.ClosePath()
	dc.Stroke()

	ref := newClipTestContext()
	ref.SetLineWidth(4)
	ref.MoveTo(20, 30)
	ref.LineTo(80, 30)
	ref.LineTo(80, 70)
	ref.LineTo(20, 70)
	ref.ClosePath()
	ref.Stroke()

	if n := countDiff(ref.Image().(*image.RGBA), dc.Image().(*image.RGBA)); n != 0 {
		t.Fatalf("stroke width/position drifted after scaled clip: %d differing pixels", n)
	}
}

// TestNestedClipStateTracking exercises two nesting levels with different
// transforms and asserts matrix, mask and stack state at every Pop.
func TestNestedClipStateTracking(t *testing.T) {
	dc := NewContext(200, 200)
	dc.Push()
	dc.Translate(4, 4)
	outerMatrix := dc.matrix
	dc.Push()
	dc.Scale(2, 2)
	// Device-space clip region: (20,20)-(180,180).
	dc.DrawRectangle(8, 8, 80, 80)
	dc.Clip()

	dc.Pop()
	if dc.matrix != outerMatrix {
		t.Fatalf("inner Pop restored wrong matrix: got %+v want %+v", dc.matrix, outerMatrix)
	}
	if dc.mask == nil || len(dc.stack) != 1 {
		t.Fatalf("state after inner Pop wrong: maskNil=%v stack=%d", dc.mask == nil, len(dc.stack))
	}

	dc.Pop()
	if dc.matrix != Identity() || len(dc.stack) != 0 {
		t.Fatalf("state after outer Pop wrong: matrix=%+v stack=%d", dc.matrix, len(dc.stack))
	}
	if dc.mask == nil {
		t.Fatal("clip mask lost after outer Pop")
	}
}
