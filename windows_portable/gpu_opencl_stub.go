//go:build !windows

package main

import (
	"errors"
	"sync/atomic"
)

func openCLGPUAvailable() (bool, string) {
	return false, "OpenCL GPU backend is available only in the Windows portable build"
}

func renderOpenCL(scene *Scene, cam Camera, settings RenderSettings, cancel *atomic.Bool,
	onChunk func(y0, rows int, data []Vec3, progress float64)) ([]Vec3, []Guide, int64, string, error) {
	return nil, nil, 0, "", errors.New("OpenCL GPU backend unavailable on this platform")
}
