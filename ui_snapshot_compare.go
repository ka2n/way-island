package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/orisano/pixelmatch"
)

const defaultSnapshotDiffRatio = 0.01

type snapshotComparisonResult struct {
	Width      int
	Height     int
	DiffPixels int
	DiffRatio  float64
	DiffPath   string
}

func compareSnapshotImages(expectedPath, actualPath, diffPath string, threshold float64) (snapshotComparisonResult, error) {
	expected, err := loadPNG(expectedPath)
	if err != nil {
		return snapshotComparisonResult{}, fmt.Errorf("load expected snapshot: %w", err)
	}

	actual, err := loadPNG(actualPath)
	if err != nil {
		return snapshotComparisonResult{}, fmt.Errorf("load actual snapshot: %w", err)
	}

	if expected.Bounds().Dx() != actual.Bounds().Dx() || expected.Bounds().Dy() != actual.Bounds().Dy() {
		return snapshotComparisonResult{}, fmt.Errorf("snapshot size mismatch: expected=%dx%d actual=%dx%d",
			expected.Bounds().Dx(), expected.Bounds().Dy(), actual.Bounds().Dx(), actual.Bounds().Dy())
	}

	diffImage := image.NewRGBA(expected.Bounds())
	var diffOutput image.Image = diffImage
	diffPixels, err := pixelmatch.MatchPixel(
		expected,
		actual,
		pixelmatch.Threshold(threshold),
		pixelmatch.WriteTo(&diffOutput),
		pixelmatch.DiffColor(color.RGBA{255, 64, 64, 255}),
		pixelmatch.Alpha(0.5),
	)
	if err != nil {
		return snapshotComparisonResult{}, fmt.Errorf("compare snapshot: %w", err)
	}

	if err := writePNG(diffPath, diffImage); err != nil {
		return snapshotComparisonResult{}, fmt.Errorf("write diff snapshot: %w", err)
	}

	totalPixels := expected.Bounds().Dx() * expected.Bounds().Dy()
	return snapshotComparisonResult{
		Width:      expected.Bounds().Dx(),
		Height:     expected.Bounds().Dy(),
		DiffPixels: diffPixels,
		DiffRatio:  float64(diffPixels) / float64(totalPixels),
		DiffPath:   diffPath,
	}, nil
}

func loadPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}
