//go:build windows && beamcast_cli

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	sceneArg := flag.String("scene", "showcase", "showcase|cornell|stress|file.obj")
	quality := flag.String("quality", "Preview", "Draft|Preview|Balanced|High|Ultra|Ultra-Realism")
	width := flag.Int("width", 0, "override width")
	height := flag.Int("height", 0, "override height")
	output := flag.String("output", "beamcast_render.ppm", "output PPM/BMP/PFM path")
	rotate := flag.Int("rotate", 0, "display/output rotation: 0,90,180,270")
	noDenoise := flag.Bool("no-denoise", false, "disable post denoiser")
	backend := flag.String("backend", "auto", "auto|cpu|gpu|opencl")
	gpuThroughput := flag.Bool("gpu-throughput", false, "apply the GPU throughput / no-guide preset")
	upscale := flag.String("upscale", "off", "off|ultra|balanced|performance")
	integrator := flag.String("integrator", "path", "path|recursive|photon")
	photon := flag.Int("photon", -1, "photon mapping quality 0-4")
	debugView := flag.String("debug", "beauty", "beauty|albedo|normal|depth|ao|heat")
	tile := flag.Int("tile", 0, "CPU tile size 8|16|32")
	tileOrder := flag.String("tile-order", "center", "center|scanline")
	workers := flag.Int("workers", -1, "CPU workers 0=auto or explicit positive value")
	bvh := flag.String("bvh", "sah", "sah|median")
	bvhBins := flag.Int("bvh-bins", 16, "SAH bins 8|16|32")
	bvhLeaf := flag.Int("bvh-leaf", 4, "BVH leaf size 2|4|8")
	publish := flag.Int("publish", 30, "progressive publish batches 15|30|60")
	progressive := flag.Bool("progressive", true, "progressive framebuffer updates")
	adaptive := flag.Bool("adaptive", true, "adaptive sampling")
	adaptiveThreshold := flag.Float64("adaptive-threshold", -1, "adaptive variance threshold")
	sampler := flag.String("sampler", "halton", "random|halton|r2")
	mis := flag.Bool("mis", true, "power-heuristic multiple importance sampling")
	powerLights := flag.Bool("power-lights", true, "power-weighted emissive light selection")
	fireflyClamp := flag.Float64("firefly-clamp", -1, "radiance clamp; 0 disables")
	rrDepth := flag.Int("rr-depth", 0, "Russian roulette start bounce")
	fxaa := flag.Int("fxaa", -1, "FXAA level 0-3")
	taa := flag.Int("taa", -1, "TAA level 0-3")
	tsaa := flag.Int("tsaa", -1, "TSAA level 0-3")
	txaa := flag.Int("txaa", -1, "TXAA-style level 0-3")
	msaa := flag.Int("msaa", 0, "MSAA/stochastic supersample 1|2|4|8")
	hbao := flag.Int("hbao", -1, "HBAO level 0-3")
	hbaop := flag.Int("hbaoplus", -1, "HBAO+ level 0-3")
	bloom := flag.Int("bloom", -1, "bloom level 0-3")
	hdr := flag.Int("hdr", -1, "HDR tone map 0=off 1=Reinhard 2=filmic 3=ACES-style")
	exposure := flag.Float64("exposure", 999, "exposure in EV")
	gamma := flag.Float64("gamma", 0, "display gamma")
	flag.Parse()

	var scene *Scene
	var cam Camera
	var err error
	switch strings.ToLower(*sceneArg) {
	case "showcase":
		scene, cam = NewShowcaseScene()
	case "cornell":
		scene, cam = NewCornellScene()
	case "stress":
		scene, cam = NewStressScene()
	default:
		scene, cam, err = LoadOBJ(*sceneArg)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Beamcast CLI:", err)
		os.Exit(1)
	}
	qualityKey := strings.ToLower(strings.TrimSpace(*quality))
	var q RenderSettings
	switch qualityKey {
	case "ultra-realism", "ultra realism", "realism":
		q = Quality("Ultra Realism")
	default:
		q = Quality(strings.Title(qualityKey))
	}
	if *width > 0 {
		q.Width = *width
	}
	if *height > 0 {
		q.Height = *height
	}
	if *noDenoise {
		q.Denoise = false
	}
	switch strings.ToLower(*backend) {
	case "cpu":
		q.Backend = BackendCPU
	case "gpu", "opencl":
		q.Backend = BackendGPU
	default:
		q.Backend = BackendAuto
	}
	if q.Quality == "Ultra Realism" && q.Backend == BackendGPU {
		compat := UltraRealismSettings(BackendGPU)
		compat.Width, compat.Height = q.Width, q.Height
		compat.Exposure, compat.Gamma = q.Exposure, q.Gamma
		q = compat
	}
	switch strings.ToLower(*upscale) {
	case "ultra", "ultraquality", "ultra-quality":
		q.Upscale = UpscaleUltraQuality
	case "balanced":
		q.Upscale = UpscaleBalanced
	case "performance":
		q.Upscale = UpscalePerformance
	default:
		q.Upscale = UpscaleOff
	}
	switch strings.ToLower(*integrator) {
	case "recursive", "ray", "raytrace":
		q.Integrator = IntegratorRecursive
	case "photon", "photonmap", "photon-mapping":
		q.Integrator = IntegratorPhoton
	default:
		q.Integrator = IntegratorPath
	}
	switch strings.ToLower(*debugView) {
	case "albedo":
		q.DebugView = DebugAlbedo
	case "normal", "normals":
		q.DebugView = DebugNormal
	case "depth":
		q.DebugView = DebugDepth
	case "ao":
		q.DebugView = DebugAO
	case "heat", "heatmap":
		q.DebugView = DebugHeat
	default:
		q.DebugView = DebugBeauty
	}
	if *photon >= 0 {
		if *photon > 4 {
			*photon = 4
		}
		q.PhotonMapping = *photon
	}
	if *tile == 8 || *tile == 16 || *tile == 32 {
		q.TileSize = *tile
	}
	if strings.EqualFold(*tileOrder, "scanline") {
		q.TileOrder = "Scanline"
	} else {
		q.TileOrder = "Center"
	}
	if *workers >= 0 {
		q.CPUWorkers = *workers
	}
	if strings.EqualFold(*bvh, "median") {
		q.BVHMode = "Median"
	} else {
		q.BVHMode = "SAH"
	}
	if *bvhBins == 8 || *bvhBins == 16 || *bvhBins == 32 {
		q.BVHBins = *bvhBins
	}
	if *bvhLeaf == 2 || *bvhLeaf == 4 || *bvhLeaf == 8 {
		q.BVHLeafSize = *bvhLeaf
	}
	if *publish == 15 || *publish == 30 || *publish == 60 {
		q.PublishHz = *publish
	}
	q.ProgressiveUpdates = *progressive
	q.AdaptiveSampling = *adaptive
	if *adaptiveThreshold > 0 {
		q.AdaptiveThreshold = *adaptiveThreshold
	}
	switch strings.ToLower(*sampler) {
	case "random":
		q.Sampler = SamplerRandom
	case "r2", "quasirandom", "quasi-random":
		q.Sampler = SamplerR2
	default:
		q.Sampler = SamplerHalton
	}
	q.MIS = *mis
	q.PowerLightSampling = *powerLights
	if *fireflyClamp >= 0 {
		q.FireflyClamp = *fireflyClamp
	}
	if *rrDepth > 0 {
		q.RRDepth = *rrDepth
	}
	clampLevel := func(v int) int {
		if v < 0 {
			return v
		}
		if v > 3 {
			return 3
		}
		return v
	}
	if *fxaa >= 0 {
		q.FXAA = clampLevel(*fxaa)
	}
	if *taa >= 0 {
		q.TAA = clampLevel(*taa)
	}
	if *tsaa >= 0 {
		q.TSAA = clampLevel(*tsaa)
	}
	if *txaa >= 0 {
		q.TXAA = clampLevel(*txaa)
	}
	if *msaa == 1 || *msaa == 2 || *msaa == 4 || *msaa == 8 {
		q.MSAA = *msaa
	}
	if *hbao >= 0 {
		q.HBAO = clampLevel(*hbao)
		if q.HBAO > 0 {
			q.HBAOPlus = 0
		}
	}
	if *hbaop >= 0 {
		q.HBAOPlus = clampLevel(*hbaop)
		if q.HBAOPlus > 0 {
			q.HBAO = 0
		}
	}
	if *bloom >= 0 {
		q.Bloom = clampLevel(*bloom)
	}
	if *hdr >= 0 {
		q.HDR = clampLevel(*hdr)
	}
	if *exposure != 999 {
		q.Exposure = *exposure
	}
	if *gamma > 0 {
		q.Gamma = *gamma
	}
	if *gpuThroughput {
		q.Backend = BackendGPU
		q.Integrator = IntegratorPath
		q.Sampler = SamplerRandom
		q.MIS = false
		q.PowerLightSampling = false
		q.FireflyClamp = 0
		q.AdaptiveSampling = false
		q.ProgressiveUpdates = false
		q.Denoise = false
		q.HBAO = 0
		q.HBAOPlus = 0
		q.TAA = 0
		q.TSAA = 0
		q.TXAA = 0
		q.FXAA = 0
		q.Bloom = 0
		q.DebugView = DebugBeauty
	}
	scene.BVHMode = q.BVHMode
	scene.BVHBins = q.BVHBins
	scene.BVHLeafSize = q.BVHLeafSize
	scene.Build()
	state := NewRenderState()
	state.Start(scene, cam, q)
	last := -1
	for state.Meta().Rendering {
		m := state.Meta()
		p := int(m.Progress * 100)
		if p != last {
			fmt.Printf("\rRendering %d%%", p)
			last = p
		}
		time.Sleep(50 * time.Millisecond)
	}
	m := state.Meta()
	fmt.Printf("\rRendering 100%%\n")
	if m.Status != "Render complete" {
		fmt.Fprintln(os.Stderr, m.Status)
		os.Exit(2)
	}
	w, h, pix := state.PixelsCopy()
	rot := ((*rotate/90)%4 + 4) % 4
	lowerOutput := strings.ToLower(*output)
	if strings.HasSuffix(lowerOutput, ".bmp") {
		err = SaveBMP(*output, pix, w, h, rot)
	} else if strings.HasSuffix(lowerOutput, ".pfm") {
		lw, lh, linear := state.LinearCopy()
		err = SavePFM(*output, linear, lw, lh, rot)
	} else {
		err = SavePPM(*output, pix, w, h, rot)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(3)
	}
	rps := 0.0
	if m.Seconds > 0 {
		rps = float64(m.Rays) / m.Seconds
	}
	fmt.Printf("Saved %s\nIntegrator %s | Sampler %s | MIS %v | Adaptive %v\nTime %.3fs | Rays %d | %.2f Mray/s\n", *output, q.Integrator, q.Sampler, q.MIS, q.AdaptiveSampling, m.Seconds, m.Rays, rps/1e6)
}
