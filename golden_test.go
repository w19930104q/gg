package gg

// Pixel-level golden regression tests.
//
// Each test case renders a fixed, fully deterministic drawing sequence
// (text, gradients, beziers, clipping, rotated images, surface patterns)
// into an off-screen image.RGBA and compares every pixel against a golden
// baseline checked into the repository under testdata/golden:
//
//   - testdata/golden/<name>.rgba  authoritative baseline: the exact raw
//     RGBA pixel bytes (width*height*4), compared byte-for-byte. A raw dump
//     is used on purpose because image/png cannot losslessly encode the
//     non-straight-alpha pixels the rasterizer emits for translucent
//     geometry (a PNG roundtrip shifts them by +/-1).
//   - testdata/golden/manifest.json  per-case width/height and the sha256
//     of the raw RGBA bytes, used as the fast hash baseline.
//   - testdata/golden/<name>.png  human-readable preview of the same
//     pixels; documentation only, never used for comparison.
//
// Any single pixel change fails the test. On failure, the mismatching
// pixels are reported with their coordinates and bounding box, and the
// actual render plus a red difference mask are written to
// testdata/golden-failure/ for inspection.
//
// Regenerate baselines after an intentional rendering change with:
//
//	go test -run TestGolden -update-golden
//
// The baselines only depend on the Go standard library, the freetype
// rasterizer and the built-in basicfont face, so renders are reproducible
// across machines without any external font or asset.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate golden RGBA baselines, PNG previews and hash manifest")

const (
	goldenDir     = "testdata/golden"
	goldenFailDir = "testdata/golden-failure"
	goldenSize    = 240
)

// goldenCase is one fixed drawing sequence with an expected render.
type goldenCase struct {
	name string
	// render draws a deterministic scene into a fresh context. It must not
	// read any external file or use non-seeded randomness.
	render func(dc *Context)
}

// goldenEntry records the dimensions and pixel hash of one baseline.
type goldenEntry struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	SHA256 string `json:"sha256"`
}

// goldenManifest maps a case name to its baseline metadata.
type goldenManifest struct {
	Version int                    `json:"version"`
	Cases   map[string]goldenEntry `json:"cases"`
}

// goldenCases returns the fixed rendering sequences. Order is significant:
// the table is iterated in slice order so output is stable.
func goldenCases() []goldenCase {
	return []goldenCase{
		{
			name:   "text",
			render: renderGoldenText,
		},
		{
			name:   "gradient",
			render: renderGoldenGradient,
		},
		{
			name:   "bezier",
			render: renderGoldenBezier,
		},
		{
			name:   "clip",
			render: renderGoldenClip,
		},
		{
			name:   "rotated-image",
			render: renderGoldenRotatedImage,
		},
		{
			name:   "pattern",
			render: renderGoldenPattern,
		},
		{
			name:   "composite",
			render: renderGoldenComposite,
		},
	}
}

// renderGoldenText draws solid text with the built-in bitmap face and a
// rotated anchored string, exercising the glyph rasterization path.
func renderGoldenText(dc *Context) {
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0.1, 0.1, 0.35)
	dc.DrawString("Golden text 0123", 12, 30)
	dc.SetRGB(0.75, 0.1, 0.1)
	dc.DrawStringAnchored("anchored", 120, 70, 0.5, 0.5)
	dc.Push()
	dc.RotateAbout(Radians(-18), 120, 150)
	dc.SetRGB(0.05, 0.45, 0.1)
	dc.DrawStringAnchored("rotated string", 120, 150, 0.5, 0.5)
	dc.Pop()
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(1)
	dc.DrawRectangle(11.5, 12.5, 217, 215)
	dc.Stroke()
}

// renderGoldenGradient fills with linear, radial and conic gradients,
// including extra color stops and overlapping translucent geometry.
func renderGoldenGradient(dc *Context) {
	lin := NewLinearGradient(0, 0, 160, 160)
	lin.AddColorStop(0, color.RGBA{255, 0, 0, 255})
	lin.AddColorStop(0.5, color.RGBA{0, 255, 0, 255})
	lin.AddColorStop(1, color.RGBA{0, 0, 255, 255})
	dc.SetFillStyle(lin)
	dc.DrawRectangle(0, 0, 160, 160)
	dc.Fill()

	rad := NewRadialGradient(60, 180, 5, 60, 180, 70)
	rad.AddColorStop(0, color.RGBA{255, 255, 255, 255})
	rad.AddColorStop(0.6, color.RGBA{255, 200, 0, 255})
	rad.AddColorStop(1, color.RGBA{120, 0, 120, 255})
	dc.SetFillStyle(rad)
	dc.DrawCircle(60, 180, 70)
	dc.Fill()

	con := NewConicGradient(180, 180, 30)
	con.AddColorStop(0, color.RGBA{0, 0, 0, 255})
	con.AddColorStop(0.25, color.RGBA{255, 0, 0, 255})
	con.AddColorStop(0.5, color.RGBA{0, 255, 0, 255})
	con.AddColorStop(0.75, color.RGBA{0, 0, 255, 255})
	con.AddColorStop(1, color.RGBA{0, 0, 0, 255})
	dc.SetFillStyle(con)
	dc.DrawCircle(180, 180, 58)
	dc.Fill()

	dc.SetRGBA(1, 1, 1, 0.35)
	dc.DrawCircle(120, 120, 45)
	dc.Fill()
}

// renderGoldenBezier strokes and fills quadratic and cubic beziers with
// fixed control points and varying widths.
func renderGoldenBezier(dc *Context) {
	dc.SetRGB(0.96, 0.96, 0.93)
	dc.Clear()

	dc.SetRGB(0.8, 0.15, 0.15)
	dc.SetLineWidth(4)
	dc.MoveTo(20, 200)
	dc.CubicTo(40, 20, 120, 20, 140, 200)
	dc.Stroke()

	dc.SetRGB(0.15, 0.3, 0.8)
	dc.SetLineWidth(2.5)
	dc.MoveTo(150, 40)
	dc.QuadraticTo(220, 60, 200, 150)
	dc.Stroke()

	dc.SetRGBA(0.1, 0.6, 0.2, 0.85)
	dc.MoveTo(30, 30)
	dc.CubicTo(80, 5, 60, 90, 110, 100)
	dc.QuadraticTo(70, 130, 30, 30)
	dc.ClosePath()
	dc.Fill()

	dc.SetRGBA(0, 0, 0, 0.25)
	dc.SetLineWidth(1)
	dc.MoveTo(20, 200)
	dc.LineTo(40, 20)
	dc.LineTo(120, 20)
	dc.LineTo(140, 200)
	dc.Stroke()

	dc.SetRGB(0.9, 0.5, 0)
	dc.DrawCircle(120, 210, 18)
	dc.Fill()
}

// renderGoldenClip intersects two clip regions (circle and rotated
// rectangle) before painting.
func renderGoldenClip(dc *Context) {
	dc.SetRGB(0.08, 0.08, 0.12)
	dc.Clear()

	dc.DrawCircle(120, 120, 95)
	dc.Clip()
	dc.Push()
	dc.RotateAbout(Radians(30), 120, 120)
	dc.DrawRectangle(60, 60, 120, 120)
	dc.Clip()
	dc.Pop()

	lin := NewLinearGradient(0, 0, 240, 240)
	lin.AddColorStop(0, color.RGBA{255, 230, 80, 255})
	lin.AddColorStop(1, color.RGBA{20, 120, 220, 255})
	dc.SetFillStyle(lin)
	dc.DrawRectangle(0, 0, 240, 240)
	dc.Fill()

	dc.ResetClip()
	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(1.5)
	dc.DrawCircle(120, 120, 95)
	dc.Stroke()
}

// makeGoldenSource builds the procedurally generated source image used by
// the rotated-image case, so no external PNG asset is required.
func makeGoldenSource() image.Image {
	src := NewContext(96, 96)
	src.SetRGB(0.95, 0.95, 0.95)
	src.Clear()
	src.SetRGB(0.85, 0.1, 0.1)
	src.DrawCircle(48, 48, 34)
	src.Fill()
	src.SetRGB(0.1, 0.2, 0.7)
	src.DrawRectangle(12, 12, 30, 30)
	src.Fill()
	for i := 0; i < 96; i += 12 {
		pos := float64(i) + 0.5
		src.DrawLine(pos, 0, pos, 96)
		src.DrawLine(0, pos, 96, pos)
	}
	src.SetRGB(0, 0, 0)
	src.SetLineWidth(0.5)
	src.Stroke()
	return src.Image()
}

// renderGoldenRotatedImage draws a procedurally generated image through a
// rotation matrix, with and without an active clip region.
func renderGoldenRotatedImage(dc *Context) {
	dc.SetRGB(0.12, 0.14, 0.18)
	dc.Clear()
	src := makeGoldenSource()

	dc.Push()
	dc.RotateAbout(Radians(25), 80, 90)
	dc.DrawImageAnchored(src, 80, 90, 0.5, 0.5)
	dc.Pop()

	dc.DrawCircle(175, 165, 52)
	dc.Clip()
	dc.Push()
	dc.RotateAbout(Radians(-40), 175, 165)
	dc.DrawImageAnchored(src, 175, 165, 0.5, 0.5)
	dc.Pop()
	dc.ResetClip()

	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(1)
	dc.DrawCircle(175, 165, 52)
	dc.Stroke()
}

// makeCheckerPattern builds a small repeating tile as a surface pattern.
func makeCheckerPattern() Pattern {
	tile := NewContext(24, 24)
	tile.SetRGB(0.2, 0.2, 0.25)
	tile.Clear()
	tile.SetRGB(0.9, 0.9, 0.85)
	tile.DrawRectangle(0, 0, 12, 12)
	tile.Fill()
	tile.DrawRectangle(12, 12, 12, 12)
	tile.Fill()
	return NewSurfacePattern(tile.Image(), RepeatBoth)
}

// renderGoldenPattern fills paths with a repeating surface pattern and
// overlays a translucent solid fill.
func renderGoldenPattern(dc *Context) {
	dc.SetRGB(0, 0, 0)
	dc.Clear()
	pattern := makeCheckerPattern()

	dc.SetFillStyle(pattern)
	dc.DrawRoundedRectangle(18, 18, 204, 90, 16)
	dc.Fill()

	dc.MoveTo(120, 130)
	dc.LineTo(210, 222)
	dc.LineTo(30, 222)
	dc.ClosePath()
	dc.Fill()

	dc.SetRGBA(0.95, 0.4, 0.1, 0.45)
	dc.DrawCircle(120, 120, 42)
	dc.Fill()

	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(18, 18, 204, 90, 16)
	dc.Stroke()
}

// renderGoldenComposite exercises every feature in one fixed sequence on a
// single canvas: pattern background, clipped gradient, rotated image,
// beziers and text on top.
func renderGoldenComposite(dc *Context) {
	dc.SetFillStyle(makeCheckerPattern())
	dc.DrawRectangle(0, 0, goldenSize, goldenSize)
	dc.Fill()

	dc.DrawCircle(85, 95, 70)
	dc.Clip()
	lin := NewLinearGradient(20, 20, 150, 170)
	lin.AddColorStop(0, color.RGBA{20, 60, 180, 255})
	lin.AddColorStop(1, color.RGBA{180, 40, 160, 255})
	dc.SetFillStyle(lin)
	dc.DrawRectangle(0, 0, 170, 170)
	dc.Fill()
	dc.ResetClip()

	dc.Push()
	dc.RotateAbout(Radians(15), 175, 175)
	dc.DrawImageAnchored(makeGoldenSource(), 175, 175, 0.5, 0.5)
	dc.Pop()

	dc.SetRGBA(1, 1, 1, 0.9)
	dc.SetLineWidth(3)
	dc.MoveTo(15, 220)
	dc.CubicTo(70, 150, 130, 230, 225, 160)
	dc.Stroke()

	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored("GOLDEN", 120, 30, 0.5, 0)
	dc.SetRGB(1, 1, 0.7)
	dc.DrawString("pixel baseline", 14, 232)
}

// pixelHash returns the lowercase hex sha256 of the raw RGBA pixel bytes.
func pixelHash(im *image.RGBA) string {
	sum := sha256.Sum256(im.Pix)
	return hex.EncodeToString(sum[:])
}

// pixelDiff describes the differences between two equally sized RGBA images.
type pixelDiff struct {
	count                     int
	firstX, firstY            int
	minX, minY, maxX, maxY    int
}

func diffImages(got, want *image.RGBA) pixelDiff {
	d := pixelDiff{
		firstX: -1, firstY: -1,
		minX: math.MaxInt32, minY: math.MaxInt32,
		maxX: -1, maxY: -1,
	}
	b := got.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := got.PixOffset(x, y)
			if got.Pix[i] != want.Pix[i] || got.Pix[i+1] != want.Pix[i+1] ||
				got.Pix[i+2] != want.Pix[i+2] || got.Pix[i+3] != want.Pix[i+3] {
				d.count++
				if d.firstX < 0 {
					d.firstX, d.firstY = x, y
				}
				if x < d.minX {
					d.minX = x
				}
				if x > d.maxX {
					d.maxX = x
				}
				if y < d.minY {
					d.minY = y
				}
				if y > d.maxY {
					d.maxY = y
				}
			}
		}
	}
	return d
}

// loadGoldenRGBA reads the authoritative raw RGBA baseline into an
// image.RGBA with the same in-memory layout as a freshly rendered canvas.
func loadGoldenRGBA(t *testing.T, path string, width, height int) *image.RGBA {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden baseline %s: %v (run with -update-golden to create baselines)", path, err)
	}
	if want := width * height * 4; len(data) != want {
		t.Fatalf("golden baseline %s has %d bytes, expected %d for %dx%d RGBA",
			path, len(data), want, width, height)
	}
	im := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(im.Pix, data)
	return im
}

// writePNG encodes im to path, creating parent directories as needed.
func writePNG(t *testing.T, path string, im image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		t.Fatalf("encode PNG %s: %v", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write PNG %s: %v", path, err)
	}
}

// diffMask renders differing pixels red over a dimmed copy of the actual
// render so the changed region is easy to locate visually.
func diffMask(got, want *image.RGBA) *image.RGBA {
	out := image.NewRGBA(got.Bounds())
	b := got.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := got.PixOffset(x, y)
			r, g, bl, a := got.Pix[i], got.Pix[i+1], got.Pix[i+2], got.Pix[i+3]
			if r == want.Pix[i] && g == want.Pix[i+1] && bl == want.Pix[i+2] && a == want.Pix[i+3] {
				out.SetRGBA(x, y, color.RGBA{R: r / 3, G: g / 3, B: bl / 3, A: a})
			} else {
				out.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			}
		}
	}
	return out
}

func loadManifest(t *testing.T) goldenManifest {
	t.Helper()
	path := filepath.Join(goldenDir, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden manifest %s: %v (run with -update-golden to create baselines)", path, err)
	}
	var m goldenManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse golden manifest %s: %v", path, err)
	}
	return m
}

func TestGolden(t *testing.T) {
	cases := goldenCases()

	if *updateGolden {
		manifest := goldenManifest{Version: 1, Cases: map[string]goldenEntry{}}
		for _, tc := range cases {
			dc := NewContext(goldenSize, goldenSize)
			tc.render(dc)
			rgbaPath := filepath.Join(goldenDir, tc.name+".rgba")
			if err := os.MkdirAll(goldenDir, 0o755); err != nil {
				t.Fatalf("create golden dir: %v", err)
			}
			if err := os.WriteFile(rgbaPath, dc.im.Pix, 0o644); err != nil {
				t.Fatalf("write golden baseline %s: %v", rgbaPath, err)
			}
			// Visual preview only; the .rgba file remains authoritative.
			writePNG(t, filepath.Join(goldenDir, tc.name+".png"), dc.im)
			manifest.Cases[tc.name] = goldenEntry{
				Width:  goldenSize,
				Height: goldenSize,
				SHA256: pixelHash(dc.im),
			}
		}
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			t.Fatalf("marshal manifest: %v", err)
		}
		if err := os.WriteFile(filepath.Join(goldenDir, "manifest.json"),
			append(data, '\n'), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		t.Logf("regenerated %d golden baselines in %s", len(cases), goldenDir)
		return
	}

	manifest := loadManifest(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			entry, ok := manifest.Cases[tc.name]
			if !ok {
				t.Fatalf("no manifest entry for %q", tc.name)
			}

			dc := NewContext(goldenSize, goldenSize)
			tc.render(dc)
			got := dc.im

			if got.Bounds().Dx() != entry.Width || got.Bounds().Dy() != entry.Height {
				t.Fatalf("render size for %q: got %dx%d, want %dx%d",
					tc.name, got.Bounds().Dx(), got.Bounds().Dy(), entry.Width, entry.Height)
			}

			// Fast hash check against the manifest, then an exhaustive
			// pixel-by-pixel check against the stored RGBA baseline.
			if gotHash := pixelHash(got); gotHash != entry.SHA256 {
				t.Errorf("pixel hash mismatch for %q: got %s, want %s",
					tc.name, gotHash, entry.SHA256)
			}

			want := loadGoldenRGBA(t, filepath.Join(goldenDir, tc.name+".rgba"),
				entry.Width, entry.Height)
			d := diffImages(got, want)
			if d.count == 0 {
				return
			}
			t.Errorf("pixel mismatch for %q: %d of %d pixels differ; "+
				"first at (x=%d, y=%d); diff bbox x[%d..%d] y[%d..%d]",
				tc.name, d.count, entry.Width*entry.Height,
				d.firstX, d.firstY, d.minX, d.maxX, d.minY, d.maxY)
			writePNG(t, filepath.Join(goldenFailDir, tc.name+".actual.png"), got)
			writePNG(t, filepath.Join(goldenFailDir, tc.name+".diff.png"), diffMask(got, want))
			t.Logf("failure artifacts written under %s/", goldenFailDir)
		})
	}
}
