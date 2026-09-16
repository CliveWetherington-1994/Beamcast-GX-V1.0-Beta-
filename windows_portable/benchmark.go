package main

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"
)

type GigaBenchCase struct {
	Name       string
	Seconds    float64
	Rays       int64
	MRays      float64
	MPixels    float64
	Backend    string
	Device     string
	Workers    int
	Resolution string
	Note       string
}

type GigaBenchReport struct {
	Title    string
	Cases    []GigaBenchCase
	Summary  []string
	Duration time.Duration
}

func (r GigaBenchReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "BEAMCAST GIGA BENCHMARK\r\n%s\r\n\r\n", r.Title)
	for _, c := range r.Cases {
		fmt.Fprintf(&b, "%s\r\n", c.Name)
		fmt.Fprintf(&b, "  %s | %.3f s", c.Resolution, c.Seconds)
		if c.Workers > 0 {
			fmt.Fprintf(&b, " | %d worker(s)", c.Workers)
		}
		b.WriteString("\r\n")
		if c.Rays > 0 {
			fmt.Fprintf(&b, "  %d rays | %.2f Mray/s\r\n", c.Rays, c.MRays)
		}
		if c.MPixels > 0 {
			fmt.Fprintf(&b, "  %.2f Mpix/s\r\n", c.MPixels)
		}
		if c.Backend != "" {
			fmt.Fprintf(&b, "  backend: %s", c.Backend)
			if c.Device != "" {
				fmt.Fprintf(&b, " | %s", c.Device)
			}
			b.WriteString("\r\n")
		}
		if c.Note != "" {
			fmt.Fprintf(&b, "  %s\r\n", c.Note)
		}
		b.WriteString("\r\n")
	}
	if len(r.Summary) > 0 {
		b.WriteString("SUMMARY\r\n")
		for _, s := range r.Summary {
			fmt.Fprintf(&b, "  %s\r\n", s)
		}
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "Suite wall time: %.2f s\r\n", r.Duration.Seconds())
	b.WriteString("\r\nNumbers are comparable only when the same Beamcast benchmark mode and build are used.")
	return b.String()
}

func benchmarkSettings(w, h, spp, bounces, workers int) RenderSettings {
	q := Quality("Draft")
	q.Width, q.Height = w, h
	q.SPP, q.Bounces = spp, bounces
	q.Backend = BackendCPU
	q.Integrator = IntegratorPath
	q.CPUWorkers = workers
	q.TileSize = 8
	q.TileOrder = "Center"
	q.ProgressiveUpdates = false
	q.PublishHz = 0
	q.AdaptiveSampling = false
	q.Upscale = UpscaleOff
	q.Denoise = false
	q.FXAA, q.TAA, q.TSAA, q.TXAA = 0, 0, 0, 0
	q.HBAO, q.HBAOPlus, q.Bloom = 0, 0, 0
	q.DebugView = DebugBeauty
	q.MSAA = 1
	q.Sampler = SamplerRandom
	q.MIS = true
	q.PowerLightSampling = true
	q.BVHMode, q.BVHBins, q.BVHLeafSize = "SAH", 16, 4
	return q
}

func prepareBenchScene(scene *Scene, q RenderSettings) {
	scene.BVHMode = q.BVHMode
	scene.BVHBins = q.BVHBins
	scene.BVHLeafSize = q.BVHLeafSize
	scene.Build()
}

func runBenchCase(name string, scene *Scene, cam Camera, q RenderSettings) GigaBenchCase {
	prepareBenchScene(scene, q)
	state := NewRenderState()
	start := time.Now()
	state.Start(scene, cam, q)
	for state.Meta().Rendering {
		time.Sleep(2 * time.Millisecond)
	}
	wall := time.Since(start).Seconds()
	m := state.Meta()
	sec := m.Seconds
	if sec <= 0 {
		sec = wall
	}
	c := GigaBenchCase{
		Name:       name,
		Seconds:    sec,
		Rays:       m.Rays,
		Backend:    m.BackendUsed,
		Device:     m.DeviceName,
		Workers:    q.CPUWorkers,
		Resolution: fmt.Sprintf("%dx%d, %d spp, %d bounces", q.Width, q.Height, q.SPP, q.Bounces),
	}
	if c.Workers == 0 && q.Backend == BackendCPU {
		c.Workers = cpuWorkerCount(q)
	}
	if sec > 0 {
		c.MRays = float64(m.Rays) / sec / 1e6
		c.MPixels = float64(q.Width*q.Height) / sec / 1e6
	}
	return c
}

func benchWorkerList() []int {
	maxW := runtime.GOMAXPROCS(0)
	if maxW < 1 {
		maxW = 1
	}
	candidates := []int{1, 2, 4, 8, 16, 32}
	out := make([]int, 0, len(candidates))
	for _, w := range candidates {
		if w <= maxW {
			out = append(out, w)
		}
	}
	if out[len(out)-1] != maxW && maxW <= 64 {
		out = append(out, maxW)
	}
	return out
}

func benchmarkQuick() GigaBenchReport {
	started := time.Now()
	scene, cam := NewStressScene()
	q := benchmarkSettings(384, 216, 3, 5, 0)
	c := runBenchCase("Quick CPU path trace", scene, cam, q)
	summary := []string{fmt.Sprintf("CPU throughput: %.2f Mray/s", c.MRays), fmt.Sprintf("Runtime logical CPUs visible: %d", runtime.GOMAXPROCS(0))}
	return GigaBenchReport{Title: "Quick CPU", Cases: []GigaBenchCase{c}, Summary: summary, Duration: time.Since(started)}
}

func benchmarkScaling() GigaBenchReport {
	started := time.Now()
	cases := []GigaBenchCase{}
	for _, workers := range benchWorkerList() {
		scene, cam := NewStressScene()
		q := benchmarkSettings(448, 252, 4, 6, workers)
		cases = append(cases, runBenchCase(fmt.Sprintf("CPU scaling - %d worker(s)", workers), scene, cam, q))
	}
	summary := []string{}
	if len(cases) > 1 && cases[0].Seconds > 0 {
		last := cases[len(cases)-1]
		speedup := cases[0].Seconds / last.Seconds
		eff := speedup / float64(last.Workers) * 100
		summary = append(summary, fmt.Sprintf("Peak tested speedup: %.2fx", speedup))
		summary = append(summary, fmt.Sprintf("Parallel efficiency at %d workers: %.1f%%", last.Workers, eff))
	}
	return GigaBenchReport{Title: "Multicore Scaling", Cases: cases, Summary: summary, Duration: time.Since(started)}
}

func benchmarkPost() GigaBenchReport {
	started := time.Now()
	scene, cam := NewShowcaseScene()
	base := benchmarkSettings(640, 360, 2, 4, 0)
	base.DebugView = DebugBeauty
	c0 := runBenchCase("Tracing baseline", scene, cam, base)

	scene2, cam2 := NewShowcaseScene()
	post := base
	post.Denoise = true
	post.HBAOPlus = 2
	post.Bloom = 2
	post.FXAA = 2
	post.TXAA = 2
	post.Upscale = UpscaleUltraQuality
	c1 := runBenchCase("Full multicore post stack", scene2, cam2, post)
	summary := []string{fmt.Sprintf("Baseline: %.2f Mpix/s", c0.MPixels), fmt.Sprintf("Post stack: %.2f Mpix/s", c1.MPixels)}
	return GigaBenchReport{Title: "Post-processing / Reconstruction", Cases: []GigaBenchCase{c0, c1}, Summary: summary, Duration: time.Since(started)}
}

func benchmarkIntegrators() GigaBenchReport {
	started := time.Now()
	cases := []GigaBenchCase{}
	modes := []IntegratorMode{IntegratorPath, IntegratorRecursive, IntegratorPhoton}
	for _, mode := range modes {
		scene, cam := NewCornellScene()
		q := benchmarkSettings(384, 216, 2, 5, 0)
		q.Integrator = mode
		if mode == IntegratorPhoton {
			q.PhotonMapping = 1
		}
		cases = append(cases, runBenchCase(string(mode), scene, cam, q))
	}
	return GigaBenchReport{Title: "Integrator Comparison", Cases: cases, Duration: time.Since(started)}
}

func benchmarkGPU() GigaBenchReport {
	started := time.Now()
	scene, cam := NewShowcaseScene()
	q := benchmarkSettings(640, 360, 4, 6, 0)
	q.Backend = BackendGPU
	q.CPUWorkers = 0
	q.MIS = false
	q.PowerLightSampling = false
	q.FireflyClamp = 0
	q.AdaptiveSampling = false
	c := runBenchCase("OpenCL GPU probe", scene, cam, q)
	if !strings.Contains(strings.ToLower(c.Backend), "opencl") {
		c.Note = "No OpenCL execution confirmed; result may be CPU fallback."
	} else {
		c.Note = "OpenCL execution confirmed by backend telemetry."
	}
	return GigaBenchReport{Title: "GPU / Backend Probe", Cases: []GigaBenchCase{c}, Duration: time.Since(started)}
}

func gigaScore(cases []GigaBenchCase) float64 {
	logs := 0.0
	n := 0
	for _, c := range cases {
		if strings.Contains(strings.ToLower(c.Name), "opencl gpu probe") && !strings.Contains(strings.ToLower(c.Backend), "opencl") {
			continue
		}
		if c.MRays > 0 {
			logs += math.Log(math.Max(c.MRays, 1e-9))
			n++
		}
	}
	if n == 0 {
		return 0
	}
	// Score is the geometric mean of measured Mray/s multiplied by 100.
	// It is intentionally transparent and only meaningful between identical suites/builds.
	return math.Exp(logs/float64(n)) * 100
}

func benchmarkFull() GigaBenchReport {
	started := time.Now()
	reports := []GigaBenchReport{benchmarkScaling(), benchmarkPost(), benchmarkIntegrators(), benchmarkGPU()}
	out := GigaBenchReport{Title: "FULL GIGA SUITE"}
	for _, r := range reports {
		out.Cases = append(out.Cases, r.Cases...)
	}
	score := gigaScore(out.Cases)
	out.Summary = append(out.Summary, fmt.Sprintf("GIGA Score: %.0f", score))
	out.Summary = append(out.Summary, "GIGA Score = geometric mean of measured Mray/s x 100.")
	out.Summary = append(out.Summary, fmt.Sprintf("Logical CPUs visible to runtime: %d", runtime.GOMAXPROCS(0)))
	for _, c := range out.Cases {
		if strings.Contains(strings.ToLower(c.Backend), "opencl") {
			out.Summary = append(out.Summary, "OpenCL GPU backend was exercised in this run.")
			break
		}
	}
	out.Duration = time.Since(started)
	return out
}

func RunGigaBenchmark(kind string) string {
	switch strings.ToLower(kind) {
	case "quick":
		return benchmarkQuick().String()
	case "scaling":
		return benchmarkScaling().String()
	case "post":
		return benchmarkPost().String()
	case "integrators":
		return benchmarkIntegrators().String()
	case "gpu":
		return benchmarkGPU().String()
	default:
		return benchmarkFull().String()
	}
}
