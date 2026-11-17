# Golden pixel baselines

Authoritative baselines for the pixel-level regression tests in
`../../golden_test.go` (test `TestGolden`). Do not edit these files by
hand; regenerate them with:

```
make update-golden
# or: go test -run TestGolden -update-golden ./...
```

## File layout

For each fixed drawing sequence `<name>` (text, gradient, bezier, clip,
rotated-image, pattern, composite):

- `<name>.rgba` — the **authoritative** baseline: exactly `width * height * 4`
  bytes of raw RGBA pixel data, compared byte-for-byte against the render.
  Raw bytes are used because `image/png` cannot round-trip the
  non-straight-alpha pixels produced by translucent geometry without
  +/-1 rounding, so a PNG would not be a lossless baseline.
- `<name>.png` — a human-readable preview of the same pixels, for visual
  inspection only. Never used by the comparison logic.
- `manifest.json` — per-case dimensions and the SHA-256 of the raw RGBA
  bytes, used as the fast hash baseline.

All scenes render at 240x240 and use only the built-in `basicfont` face
and procedurally generated images, so no external font or asset is
needed and renders are reproducible on any machine.

## On failure

`TestGolden` reports the number of differing pixels, the first differing
coordinate and the difference bounding box, and writes the actual render
plus a red difference mask to `../golden-failure/` (git-ignored).
