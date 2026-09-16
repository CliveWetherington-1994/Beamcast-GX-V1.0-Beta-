package main

import (
	"runtime"
	"testing"
	"time"
)

func benchScaling(b *testing.B, workers int) {
	s, c := NewShowcaseScene()
	q := Quality("Preview")
	q.Width, q.Height = 480, 270
	q.SPP, q.Bounces = 6, 6
	q.Backend = BackendCPU
	q.Integrator = IntegratorPath
	q.Sampler = SamplerRandom
	q.MIS = true
	q.PowerLightSampling = true
	q.Denoise = false
	q.FXAA = 0
	q.TAA = 0
	q.TSAA = 0
	q.TXAA = 0
	q.HBAO = 0
	q.HBAOPlus = 0
	q.Bloom = 0
	q.ProgressiveUpdates = false
	q.AdaptiveSampling = false
	q.MSAA = 1
	q.CPUWorkers = workers
	runtime.GC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := NewRenderState()
		st.Start(s, c, q)
		for st.Meta().Rendering {
			time.Sleep(250 * time.Microsecond)
		}
		if st.Meta().Status != "Render complete" {
			b.Fatal(st.Meta().Status)
		}
	}
}
func BenchmarkScale1(b *testing.B) { benchScaling(b, 1) }
func BenchmarkScale2(b *testing.B) { benchScaling(b, 2) }
func BenchmarkScale4(b *testing.B) { benchScaling(b, 4) }
func BenchmarkPostHeavy4(b *testing.B) {
	s, c := NewShowcaseScene()
	q := Quality("Preview")
	q.Width, q.Height = 640, 360
	q.SPP, q.Bounces = 2, 4
	q.Backend = BackendCPU
	q.Integrator = IntegratorPath
	q.Sampler = SamplerRandom
	q.MIS = true
	q.PowerLightSampling = true
	q.CPUWorkers = 4
	q.ProgressiveUpdates = false
	q.AdaptiveSampling = false
	q.MSAA = 1
	q.Denoise = true
	q.HBAO = 0
	q.HBAOPlus = 2
	q.Bloom = 2
	q.FXAA = 2
	q.TAA = 0
	q.TSAA = 0
	q.TXAA = 1
	q.Upscale = UpscaleOff
	runtime.GC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := NewRenderState()
		st.Start(s, c, q)
		for st.Meta().Rendering {
			time.Sleep(250 * time.Microsecond)
		}
		if st.Meta().Status != "Render complete" {
			b.Fatal(st.Meta().Status)
		}
	}
}
func benchTile(b *testing.B, tile int) {
	s, c := NewShowcaseScene()
	q := Quality("Preview")
	q.Width, q.Height = 480, 270
	q.SPP, q.Bounces = 6, 6
	q.Backend = BackendCPU
	q.Integrator = IntegratorPath
	q.Sampler = SamplerRandom
	q.MIS = true
	q.PowerLightSampling = true
	q.CPUWorkers = 4
	q.TileSize = tile
	q.ProgressiveUpdates = false
	q.AdaptiveSampling = false
	q.Denoise = false
	q.FXAA = 0
	q.TAA = 0
	q.TSAA = 0
	q.TXAA = 0
	q.HBAO = 0
	q.HBAOPlus = 0
	q.Bloom = 0
	q.MSAA = 1
	runtime.GC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := NewRenderState()
		st.Start(s, c, q)
		for st.Meta().Rendering {
			time.Sleep(250 * time.Microsecond)
		}
	}
}
func BenchmarkTile8(b *testing.B)  { benchTile(b, 8) }
func BenchmarkTile16(b *testing.B) { benchTile(b, 16) }
func BenchmarkTile32(b *testing.B) { benchTile(b, 32) }
