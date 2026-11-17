.PHONY: test test-golden update-golden vet

# One-command entry point: runs the full suite including the pixel-level
# golden regression tests in golden_test.go.
test:
	go test ./...

# Only the golden pixel regression tests, with verbose per-scene output.
test-golden:
	go test -run TestGolden -v ./...

# Regenerate golden baselines after an intentional rendering change.
update-golden:
	go test -run TestGolden -update-golden ./...

vet:
	go vet ./...
