package main

import "github.com/fogleman/gg"

func main() {
	const S = 1024
	dc := gg.NewContext(S, S)
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// A dashed rectangle: the dash pattern wraps around the closed path,
	// so the corners and the start/end seam join up correctly.
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(6)
	dc.SetDash(24, 16)
	dc.SetDashOffset(8)
	dc.DrawRectangle(80, 80, S-160, S-160)
	dc.Stroke()

	// A phase-shifted dashed circle with an asymmetric pattern.
	dc.SetDash(40, 12, 8, 12)
	dc.SetDashOffset(0)
	dc.DrawCircle(S/2, S/2, 300)
	dc.Stroke()

	// An open dashed path; the pattern restarts at the first point.
	dc.SetDash(16, 10)
	dc.DrawLine(80, S-80, S-80, S-80)
	dc.Stroke()

	dc.SavePNG("dash.png")
}
