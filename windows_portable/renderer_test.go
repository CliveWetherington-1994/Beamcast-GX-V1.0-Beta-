package main

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSceneBVHAndTinyRender(t *testing.T) {
	s, c := NewShowcaseScene()
	if len(s.Primitives) < 5 || len(s.Nodes) == 0 {
		t.Fatal("showcase scene failed")
	}
	st := NewRenderState()
	q := Quality("Draft")
	q.Width = 48
	q.Height = 32
	q.SPP = 1
	q.Bounces = 2
	q.Denoise = false
	st.Start(s, c, q)
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, _, _, _, busy, status, _, _ := st.Snapshot()
		if !busy {
			if status != "Render complete" {
				t.Fatalf("render status: %s", status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("render timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	w, h, pix, _, _, _, _, _ := st.Snapshot()
	if w != 48 || h != 32 || len(pix) != 48*32 {
		t.Fatal("bad framebuffer")
	}
	p, rw, rh := RotatePixels(pix, w, h, 1)
	if rw != 32 || rh != 48 || len(p) != len(pix) {
		t.Fatal("rotation failed")
	}
}

func TestOBJ(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tri.obj")
	if err := os.WriteFile(p, []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s, _, err := LoadOBJ(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Primitives) != 1 {
		t.Fatalf("got %d primitives", len(s.Primitives))
	}
}

func TestSaveFormats(t *testing.T) {
	s, c := NewShowcaseScene()
	st := NewRenderState()
	q := Quality("Draft")
	q.Width, q.Height, q.SPP, q.Bounces, q.Denoise = 24, 16, 1, 1, false
	st.Start(s, c, q)
	deadline := time.Now().Add(5 * time.Second)
	for st.Meta().Rendering {
		if time.Now().After(deadline) {
			t.Fatal("render timeout")
		}
		time.Sleep(time.Millisecond)
	}
	w, h, pix := st.PixelsCopy()
	ppm := filepath.Join(t.TempDir(), "x.ppm")
	bmp := filepath.Join(t.TempDir(), "x.bmp")
	if err := SavePPM(ppm, pix, w, h, 1); err != nil {
		t.Fatal(err)
	}
	if err := SaveBMP(bmp, pix, w, h, 3); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(ppm); err != nil || fi.Size() <= 32 {
		t.Fatal("bad PPM")
	}
	if fi, err := os.Stat(bmp); err != nil || fi.Size() <= 54 {
		t.Fatal("bad BMP")
	}
	_, _, linear := st.LinearCopy()
	pfm := filepath.Join(t.TempDir(), "x.pfm")
	if err := SavePFM(pfm, linear, w, h, 0); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(pfm); err != nil || fi.Size() <= 64 {
		t.Fatal("bad PFM")
	}
}

func TestUpscaleAndGuideDenoise(t *testing.T) {
	src := make([]Vec3, 16)
	guides := make([]Guide, 16)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			i := y*4 + x
			v := float64((x+y)%2) * 0.8
			src[i] = V(0.2+v, 0.25+v*0.5, 0.3)
			guides[i] = Guide{P: V(float64(x), float64(y), -3), N: V(0, 0, 1), Depth: 3, Valid: true}
		}
	}
	filtered := atrousDenoise(src, guides, 4, 4, 2)
	if len(filtered) != len(src) {
		t.Fatal("denoiser changed image size")
	}
	for _, mode := range []UpscaleMode{UpscaleUltraQuality, UpscaleBalanced, UpscalePerformance} {
		out := upscaleDLSSBasic(filtered, 4, 4, 11, 7, mode)
		if len(out) != 77 {
			t.Fatalf("upscale %s produced %d pixels", mode, len(out))
		}
		for _, c := range out {
			if c.X < 0 || c.Y < 0 || c.Z < 0 {
				t.Fatalf("upscale %s produced negative radiance", mode)
			}
		}
	}
}

func TestTemporalReprojection(t *testing.T) {
	const w, h = 8, 6
	cam := Camera{Position: V(0, 0, 2), Target: V(0, 0, -3), Up: V(0, 1, 0), FOV: 45}
	current := make([]Vec3, w*h)
	previous := make([]Vec3, w*h)
	guides := make([]Guide, w*h)
	prevGuides := make([]Guide, w*h)
	origin, lower, horiz, vert := cam.basis(float64(w) / float64(h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			u := float64(x) / float64(w-1)
			v := float64(h-1-y) / float64(h-1)
			dir := lower.Add(horiz.Mul(u)).Add(vert.Mul(v)).Sub(origin).Unit()
			p := origin.Add(dir.Mul(5))
			guides[i] = Guide{P: p, N: V(0, 0, 1), Depth: 5, Valid: true}
			prevGuides[i] = guides[i]
			base := 0.8 + 0.05*float64(x)
			current[i] = V(base, 0.45+0.02*float64(y), 0.25)
			previous[i] = V(base*0.92, 0.43+0.02*float64(y), 0.24)
		}
	}
	out := temporalAccumulate(current, guides, w, h, previous, prevGuides, cam, 0.7)
	if len(out) != w*h {
		t.Fatal("temporal pass changed image size")
	}
	mixed := false
	for i := range out {
		if out[i].X < current[i].X && out[i].X > previous[i].X {
			mixed = true
			break
		}
	}
	if !mixed {
		t.Fatal("temporal reprojection did not reuse any valid history")
	}
}

func TestAdvancedPostStack(t *testing.T) {
	const w, h = 12, 8
	img := make([]Vec3, w*h)
	guides := make([]Guide, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			v := float64(x+y) / float64(w+h)
			img[i] = V(0.2+v*2.5, 0.15+v, 0.1+0.5*v)
			guides[i] = Guide{P: V(float64(x)*0.1, float64(y)*0.1, -3-v), N: V(0, 0, 1), Depth: 3 + v, Valid: true}
		}
	}
	ao := applyHBAO(img, guides, w, h, 2, true)
	bloomed := applyBloom(ao, w, h, 2)
	fx := applyFXAA(bloomed, w, h, 2)
	tx := applyTXAAResolve(fx, w, h, 2)
	if len(tx) != w*h {
		t.Fatal("post stack size changed")
	}
	for _, c := range tx {
		if math.IsNaN(c.X) || math.IsNaN(c.Y) || math.IsNaN(c.Z) || c.X < 0 || c.Y < 0 || c.Z < 0 {
			t.Fatal("post stack produced invalid radiance")
		}
	}
	for _, hdr := range []int{0, 1, 2, 3} {
		_ = toRGBA(V(4, 2, 1), 0, 2.2, hdr)
	}
}

func TestSamplingLevels(t *testing.T) {
	q := Quality("Preview")
	q.SPP = 2
	q.MSAA = 4
	q.TSAA = 2
	if got := effectiveSPP(&q); got != 24 {
		t.Fatalf("effective spp = %d, want 24", got)
	}
	q.MSAA = 8
	q.TSAA = 3
	q.SPP = 200
	if effectiveSPP(&q) > 4096 {
		t.Fatal("effective spp cap failed")
	}
}

func TestPresetObjectPickAndMove(t *testing.T) {
	s, c := NewShowcaseScene()
	idx := s.AddPresetObject("Metal Sphere", c.Target)
	if idx < 0 || len(s.Objects) == 0 {
		t.Fatal("preset object add failed")
	}
	ray := Ray{c.Position, c.Target.Sub(c.Position)}
	picked, _, ok := s.PickObject(ray)
	if !ok || picked < 0 {
		t.Fatal("preset object pick failed")
	}
	before := s.Objects[picked].Pivot
	if !s.TranslateObject(picked, V(1, 0, 0)) {
		t.Fatal("object translation failed")
	}
	after := s.Objects[picked].Pivot
	if math.Abs((after.X-before.X)-1) > 1e-9 {
		t.Fatal("object pivot did not move")
	}
	s.Build()
}

func TestResearchIntegrators(t *testing.T) {
	s, c := NewCornellScene()
	for _, integrator := range []IntegratorMode{IntegratorPath, IntegratorRecursive, IntegratorPhoton} {
		st := NewRenderState()
		q := Quality("Draft")
		q.Width, q.Height = 32, 24
		q.SPP, q.Bounces = 1, 3
		q.Denoise = false
		q.Backend = BackendCPU
		q.Integrator = integrator
		if integrator == IntegratorPhoton {
			q.PhotonMapping = 1
		}
		st.Start(s, c, q)
		deadline := time.Now().Add(5 * time.Second)
		for st.Meta().Rendering {
			if time.Now().After(deadline) {
				t.Fatalf("render timeout for %s", integrator)
			}
			time.Sleep(2 * time.Millisecond)
		}
		m := st.Meta()
		if m.Status != "Render complete" {
			t.Fatalf("integrator %s status %q", integrator, m.Status)
		}
		w, h, pix := st.PixelsCopy()
		if w != 32 || h != 24 || len(pix) != 32*24 {
			t.Fatalf("integrator %s bad framebuffer %dx%d len=%d", integrator, w, h, len(pix))
		}
	}
}

func TestDebugViewModes(t *testing.T) {
	const w, h = 6, 4
	img := make([]Vec3, w*h)
	guides := make([]Guide, w*h)
	for i := range img {
		img[i] = V(0.1+float64(i)*0.01, 0.2, 0.3)
		guides[i] = Guide{P: V(float64(i%w), float64(i/w), -2), N: V(0, 0, 1), Albedo: V(0.3, 0.4, 0.5), Depth: 1 + float64(i%w), Valid: true}
	}
	for _, mode := range []DebugViewMode{DebugBeauty, DebugAlbedo, DebugNormal, DebugDepth, DebugAO, DebugHeat} {
		q := Quality("Preview")
		q.DebugView = mode
		out := makeDebugView(img, guides, w, h, q)
		if len(out) != len(img) {
			t.Fatalf("debug mode %d changed output size", mode)
		}
		for _, c := range out {
			if math.IsNaN(c.X) || math.IsNaN(c.Y) || math.IsNaN(c.Z) {
				t.Fatalf("debug mode %d produced NaN", mode)
			}
		}
	}
}

func TestSpecialistSamplingAndClamp(t *testing.T) {
	q := Quality("Preview")
	q.Sampler = SamplerHalton
	for _, mode := range []SamplingMode{SamplerRandom, SamplerHalton, SamplerR2} {
		q.Sampler = mode
		rng := rand.New(rand.NewSource(42))
		for i := 0; i < 32; i++ {
			x, y := sampleJitter(q, 3, 7, i, rng)
			if x < 0 || x >= 1 || y < 0 || y >= 1 {
				t.Fatalf("sampler %s out of range: %f %f", mode, x, y)
			}
		}
	}
	c := clampFirefly(V(100, 50, 25), 10)
	if c.MaxComp() > 10.000001 {
		t.Fatalf("firefly clamp failed: %v", c)
	}
}

func TestPowerWeightedLightPDF(t *testing.T) {
	s, _ := NewShowcaseScene()
	if len(s.LightSpheres) == 0 {
		t.Fatal("expected emissive sphere")
	}
	sum := 0.0
	for _, pi := range s.LightSpheres {
		p := lightSelectionPDF(s, pi, true)
		if p <= 0 {
			t.Fatalf("non-positive selection pdf for light %d", pi)
		}
		sum += p
	}
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("light selection PDFs sum to %f", sum)
	}
	if powerHeuristic(0.5, 0.5) < 0.49 || powerHeuristic(0.5, 0.5) > 0.51 {
		t.Fatal("power heuristic sanity failed")
	}
}

func TestSpecialistMISRender(t *testing.T) {
	s, c := NewShowcaseScene()
	st := NewRenderState()
	q := Quality("Draft")
	q.Width, q.Height = 40, 24
	q.SPP, q.Bounces = 2, 4
	q.Backend = BackendCPU
	q.MIS = true
	q.PowerLightSampling = true
	q.Sampler = SamplerHalton
	q.FireflyClamp = 12
	q.AdaptiveSampling = true
	q.AdaptiveThreshold = 0.002
	st.Start(s, c, q)
	deadline := time.Now().Add(5 * time.Second)
	for st.Meta().Rendering {
		if time.Now().After(deadline) {
			t.Fatal("specialist MIS render timeout")
		}
		time.Sleep(2 * time.Millisecond)
	}
	if st.Meta().Status != "Render complete" {
		t.Fatalf("specialist MIS render status: %s", st.Meta().Status)
	}
	_, _, pix := st.PixelsCopy()
	if len(pix) != 40*24 {
		t.Fatal("specialist MIS render bad framebuffer")
	}
}

func TestFastAABBMatchesReference(t *testing.T) {
	boxes := []AABB{
		{V(-1, -1, -1), V(1, 1, 1)},
		{V(2, -3, -4), V(6, 5, 2)},
		{V(-10, 0, -2), V(-2, 0.1, 8)},
	}
	rays := []Ray{
		{V(0, 0, 5), V(0, 0, -1)},
		{V(0, 0, 0), V(1, 0, 0)},
		{V(4, 10, 0), V(0, -1, 0)},
		{V(20, 20, 20), V(1, 1, 1)},
		{V(-5, 0.05, 10), V(0, 0, -1)},
	}
	for _, box := range boxes {
		for _, ray := range rays {
			ref := func() bool {
				tmin, tmax := 1e-4, 1e30
				valsO := [3]float64{ray.O.X, ray.O.Y, ray.O.Z}
				valsD := [3]float64{ray.D.X, ray.D.Y, ray.D.Z}
				mn := [3]float64{box.Min.X, box.Min.Y, box.Min.Z}
				mx := [3]float64{box.Max.X, box.Max.Y, box.Max.Z}
				for i := 0; i < 3; i++ {
					if math.Abs(valsD[i]) < 1e-15 {
						if valsO[i] < mn[i] || valsO[i] > mx[i] {
							return false
						}
						continue
					}
					inv := 1 / valsD[i]
					t0, t1 := (mn[i]-valsO[i])*inv, (mx[i]-valsO[i])*inv
					if inv < 0 {
						t0, t1 = t1, t0
					}
					if t0 > tmin {
						tmin = t0
					}
					if t1 < tmax {
						tmax = t1
					}
					if tmax <= tmin {
						return false
					}
				}
				return true
			}()
			ar := prepareRay(ray)
			got, _ := box.HitAccel(&ar, 1e-4, 1e30)
			if got != ref {
				t.Fatalf("fast AABB mismatch: got %v want %v", got, ref)
			}
		}
	}
}

func TestSAHAndMedianAgree(t *testing.T) {
	s, c := NewStressScene()
	rays := make([]Ray, 400)
	rng := rand.New(rand.NewSource(42))
	for i := range rays {
		dir := c.Target.Add(V((rng.Float64()-.5)*14, (rng.Float64()-.5)*5, (rng.Float64()-.5)*14)).Sub(c.Position)
		rays[i] = Ray{c.Position, dir}
	}
	s.BVHMode, s.BVHLeafSize = "Median", 4
	s.Build()
	medianHits := make([]Hit, len(rays))
	medianOK := make([]bool, len(rays))
	for i, r := range rays {
		medianHits[i], medianOK[i] = s.Hit(r, 1e-4, 1e30)
	}
	s.BVHMode, s.BVHBins, s.BVHLeafSize = "SAH", 16, 4
	s.Build()
	for i, r := range rays {
		h, ok := s.Hit(r, 1e-4, 1e30)
		if ok != medianOK[i] {
			t.Fatalf("ray %d hit mismatch", i)
		}
		if ok && math.Abs(h.T-medianHits[i].T) > 1e-7 {
			t.Fatalf("ray %d distance mismatch %.9g %.9g", i, h.T, medianHits[i].T)
		}
	}
}

func benchmarkBVHMode(b *testing.B, mode string, bins int) {
	s, c := NewStressScene()
	s.BVHMode, s.BVHBins, s.BVHLeafSize = mode, bins, 4
	s.Build()
	rng := rand.New(rand.NewSource(99))
	rays := make([]Ray, 2048)
	for i := range rays {
		t := c.Target.Add(V((rng.Float64()-.5)*18, (rng.Float64()-.5)*6, (rng.Float64()-.5)*18))
		rays[i] = Ray{c.Position, t.Sub(c.Position)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, r := range rays {
			_, _ = s.Hit(r, 1e-4, 1e30)
		}
	}
}

func BenchmarkBVHMedian(b *testing.B) { benchmarkBVHMode(b, "Median", 16) }
func BenchmarkBVHSAH16(b *testing.B)  { benchmarkBVHMode(b, "SAH", 16) }

func TestBVHRefitMatchesRebuild(t *testing.T) {
	s, c := NewStressScene()
	s.BVHMode, s.BVHBins, s.BVHLeafSize = "SAH", 16, 4
	s.Build()
	if len(s.Primitives) < 4 {
		t.Fatal("stress scene too small")
	}
	translatePrimitive(&s.Primitives[3], V(0.75, 0.1, -0.25))
	s.Refit()
	rays := []Ray{
		{c.Position, c.Target.Sub(c.Position)},
		{c.Position, c.Target.Add(V(3, 1, -2)).Sub(c.Position)},
		{c.Position, c.Target.Add(V(-4, 0.5, 3)).Sub(c.Position)},
	}
	refitHit := make([]Hit, len(rays))
	refitOK := make([]bool, len(rays))
	for i, r := range rays {
		refitHit[i], refitOK[i] = s.Hit(r, 1e-4, 1e30)
	}
	s.Build()
	for i, r := range rays {
		h, ok := s.Hit(r, 1e-4, 1e30)
		if ok != refitOK[i] {
			t.Fatalf("ray %d refit hit mismatch", i)
		}
		if ok && math.Abs(h.T-refitHit[i].T) > 1e-7 {
			t.Fatalf("ray %d refit distance mismatch", i)
		}
	}
}

func BenchmarkBVHRefit(b *testing.B) {
	s, _ := NewStressScene()
	s.BVHMode, s.BVHBins, s.BVHLeafSize = "SAH", 16, 4
	s.Build()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		translatePrimitive(&s.Primitives[3], V(1e-7, 0, 0))
		s.Refit()
	}
}

func BenchmarkBVHRebuildSAH16(b *testing.B) {
	s, _ := NewStressScene()
	s.BVHMode, s.BVHBins, s.BVHLeafSize = "SAH", 16, 4
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		translatePrimitive(&s.Primitives[3], V(1e-7, 0, 0))
		s.Build()
	}
}

func BenchmarkHaltonLegacyInnerLoop(b *testing.B) {
	q := Quality("High")
	q.Sampler = SamplerHalton
	rng := rand.New(rand.NewSource(7))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for s := 0; s < 64; s++ {
			_, _ = sampleJitter(q, i&1023, (i>>10)&1023, s, rng)
		}
	}
}

func BenchmarkHaltonPreparedInnerLoop(b *testing.B) {
	q := Quality("High")
	q.Sampler = SamplerHalton
	q.SPP, q.MSAA, q.TSAA = 64, 1, 0
	seq := buildSampleSequence(&q)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rx, ry := pixelSampleRotation(i&1023, (i>>10)&1023)
		for s := 0; s < 64; s++ {
			_, _ = fract(seq.X[s]+rx), fract(seq.Y[s]+ry)
		}
	}
}

func TestCenterTileOrderIsPermutation(t *testing.T) {
	for _, mode := range []string{"Center", "Scanline"} {
		order := makeTileOrder(7, 5, mode)
		if len(order) != 35 {
			t.Fatalf("%s order len %d", mode, len(order))
		}
		seen := make([]bool, 35)
		for _, v := range order {
			if v < 0 || v >= 35 || seen[v] {
				t.Fatalf("%s invalid tile %d", mode, v)
			}
			seen[v] = true
		}
	}
	center := makeTileOrder(7, 5, "Center")
	first := center[0]
	if first%7 != 3 || first/7 != 2 {
		t.Fatalf("center-first began at %d", first)
	}
}

func TestCachedLightDistribution(t *testing.T) {
	s, _ := NewShowcaseScene()
	s.Build()
	if len(s.LightSpheres) == 0 || len(s.LightCDF) != len(s.LightSpheres) {
		t.Fatal("light distribution missing")
	}
	if math.Abs(s.LightCDF[len(s.LightCDF)-1]-1) > 1e-12 {
		t.Fatal("light CDF not normalized")
	}
	sum := 0.0
	for _, pi := range s.LightSpheres {
		if pi < 0 || pi >= len(s.LightPDFByPrim) {
			t.Fatal("bad light index")
		}
		sum += s.LightPDFByPrim[pi]
	}
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("light PMF sum %.12f", sum)
	}
}

func TestFinalOnlyThroughputRender(t *testing.T) {
	s, c := NewShowcaseScene()
	q := Quality("Draft")
	q.Width, q.Height, q.SPP, q.Bounces = 48, 32, 1, 2
	q.Backend = BackendCPU
	q.Denoise = false
	q.TAA, q.TSAA, q.TXAA = 0, 0, 0
	q.ProgressiveUpdates = false
	q.BVHMode, q.BVHBins, q.BVHLeafSize = "SAH", 16, 4
	s.BVHMode, s.BVHBins, s.BVHLeafSize = q.BVHMode, q.BVHBins, q.BVHLeafSize
	s.Build()
	st := NewRenderState()
	st.Start(s, c, q)
	deadline := time.Now().Add(5 * time.Second)
	for st.Meta().Rendering {
		if time.Now().After(deadline) {
			t.Fatal("final-only render timeout")
		}
		time.Sleep(time.Millisecond)
	}
	w, h, pix := st.PixelsCopy()
	if w != 48 || h != 32 || len(pix) != 48*32 {
		t.Fatalf("final-only framebuffer %dx%d len=%d", w, h, len(pix))
	}
}

func TestPreparedSampleSequenceBounds(t *testing.T) {
	for _, mode := range []SamplingMode{SamplerHalton, SamplerR2} {
		q := Quality("Preview")
		q.Sampler = mode
		q.SPP, q.MSAA, q.TSAA = 64, 1, 0
		seq := buildSampleSequence(&q)
		if seq == nil || len(seq.X) != 64 || len(seq.Y) != 64 {
			t.Fatalf("%s sequence missing", mode)
		}
		rx, ry := pixelSampleRotation(17, 29)
		for i := range seq.X {
			x, y := fract(seq.X[i]+rx), fract(seq.Y[i]+ry)
			if x < 0 || x >= 1 || y < 0 || y >= 1 {
				t.Fatalf("%s sample outside unit square", mode)
			}
		}
	}
}

func BenchmarkEndToEndSpecialistRender(b *testing.B) {
	s, c := NewShowcaseScene()
	q := Quality("Preview")
	q.Width, q.Height = 320, 180
	q.SPP, q.Bounces = 6, 6
	q.Backend = BackendCPU
	q.Integrator = IntegratorPath
	q.Sampler = SamplerRandom
	q.MIS = true
	q.PowerLightSampling = true
	q.Denoise = false
	q.FXAA, q.TAA, q.TSAA, q.TXAA = 0, 0, 0, 0
	q.HBAO, q.HBAOPlus, q.Bloom = 0, 0, 0
	q.ProgressiveUpdates = false
	q.AdaptiveSampling = false
	q.MSAA = 1
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := NewRenderState()
		st.Start(s, c, q)
		for st.Meta().Rendering {
			runtime.Gosched()
		}
		if st.Meta().Status != "Render complete" {
			b.Fatalf("status %q", st.Meta().Status)
		}
	}
}

func TestOccludedMatchesClosestHit(t *testing.T) {
	s, c := NewStressScene()
	rng := rand.New(rand.NewSource(424242))
	for i := 0; i < 3000; i++ {
		target := c.Target.Add(V((rng.Float64()-.5)*24, (rng.Float64()-.5)*8, (rng.Float64()-.5)*24))
		d := target.Sub(c.Position)
		tmax := 0.25 + rng.Float64()*30
		r := Ray{c.Position, d}
		_, hit := s.Hit(r, 1e-4, tmax)
		if got := s.Occluded(r, 1e-4, tmax); got != hit {
			t.Fatalf("ray %d occlusion mismatch got=%v want=%v", i, got, hit)
		}
	}
}

func TestCosineSamplerDistribution(t *testing.T) {
	rng := rand.New(rand.NewSource(123456789))
	n := V(0.2, 0.9, 0.38).Unit()
	mean := V(0, 0, 0)
	for i := 0; i < 20000; i++ {
		d := randCosHemisphere(n, rng)
		if math.Abs(d.Len2()-1) > 2e-12 {
			t.Fatalf("cosine sample not unit length: %.15g", d.Len2())
		}
		if d.Dot(n) < -1e-12 {
			t.Fatalf("cosine sample below hemisphere: %.15g", d.Dot(n))
		}
		mean = mean.Add(d)
	}
	if mean.Dot(n) <= 0 {
		t.Fatal("cosine sampler has invalid mean direction")
	}
}

func TestGPURevisionTracking(t *testing.T) {
	s, _ := NewShowcaseScene()
	top0, geom0 := s.GPUTopologyRevision, s.GPUGeometryRevision
	if top0 == 0 || geom0 == 0 {
		t.Fatal("initial scene build did not stamp GPU revisions")
	}
	if len(s.Objects) == 0 {
		idx := s.AddPresetObject("Diffuse Sphere", V(0, 0, -3))
		if idx < 0 {
			t.Fatal("failed to add preset object")
		}
	}
	top1, geom1 := s.GPUTopologyRevision, s.GPUGeometryRevision
	if top1 <= top0 || geom1 <= geom0 {
		t.Fatal("topology build did not advance GPU revisions")
	}
	if !s.TranslateObject(0, V(0.1, 0, 0)) {
		t.Fatal("translate failed")
	}
	s.Refit()
	if s.GPUTopologyRevision != top1 {
		t.Fatal("refit incorrectly changed topology revision")
	}
	if s.GPUGeometryRevision <= geom1 {
		t.Fatal("refit did not advance geometry revision")
	}
}

func TestParallelPostDeterminism(t *testing.T) {
	const w, h = 96, 64
	img := make([]Vec3, w*h)
	guides := make([]Guide, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			v := float64((x*13+y*7)%97) / 97.0
			img[i] = V(0.1+2*v, 0.2+v, 0.3+0.5*v)
			guides[i] = Guide{P: V(float64(x)*0.01, float64(y)*0.01, -2-v), N: V(0, 0, 1), Depth: 2 + v, Valid: true}
		}
	}
	old := runtime.GOMAXPROCS(1)
	serial := applyFXAA(applyBloom(applyHBAO(atrousDenoise(img, guides, w, h, 2), guides, w, h, 2, true), w, h, 2), w, h, 2)
	runtime.GOMAXPROCS(4)
	parallel := applyFXAA(applyBloom(applyHBAO(atrousDenoise(img, guides, w, h, 2), guides, w, h, 2, true), w, h, 2), w, h, 2)
	runtime.GOMAXPROCS(old)
	if len(serial) != len(parallel) {
		t.Fatal("parallel post changed output size")
	}
	for i := range serial {
		if serial[i] != parallel[i] {
			t.Fatalf("parallel post mismatch at %d: serial=%v parallel=%v", i, serial[i], parallel[i])
		}
	}
}

func TestGigaBenchmarkReportFormatting(t *testing.T) {
	r := GigaBenchReport{
		Title:    "Unit Test",
		Cases:    []GigaBenchCase{{Name: "CPU", Seconds: 0.1, Rays: 1000000, MRays: 10, MPixels: 2, Backend: "CPU", Workers: 4, Resolution: "64x64, 1 spp, 2 bounces"}},
		Summary:  []string{"Parallel efficiency: 90%"},
		Duration: 100 * time.Millisecond,
	}
	s := r.String()
	for _, want := range []string{"BEAMCAST GIGA BENCHMARK", "CPU", "10.00 Mray/s", "Parallel efficiency"} {
		if !strings.Contains(s, want) {
			t.Fatalf("report missing %q: %s", want, s)
		}
	}
}

func TestGigaBenchmarkQuickRuns(t *testing.T) {
	r := benchmarkQuick()
	if len(r.Cases) != 1 {
		t.Fatalf("quick benchmark cases = %d", len(r.Cases))
	}
	if r.Cases[0].Seconds <= 0 || r.Cases[0].Rays <= 0 {
		t.Fatalf("invalid quick benchmark result: %+v", r.Cases[0])
	}
}

func TestThreadedBVHEscapeTraversal(t *testing.T) {
	s, _ := NewStressScene()
	escape := threadedBVHEscapes(s.Nodes)
	if len(escape) != len(s.Nodes) || len(escape) == 0 || escape[0] != -1 {
		t.Fatal("bad threaded escape table")
	}
	// Every internal left subtree must escape into its right sibling, while each
	// right subtree inherits the parent escape. Walk the tree recursively and verify.
	var check func(int, int32)
	check = func(idx int, want int32) {
		if idx < 0 || idx >= len(s.Nodes) {
			t.Fatalf("invalid node %d", idx)
		}
		if escape[idx] != want {
			t.Fatalf("node %d escape=%d want=%d", idx, escape[idx], want)
		}
		n := s.Nodes[idx]
		if n.Count == 0 {
			check(n.Left, int32(n.Right))
			check(n.Right, want)
		}
	}
	check(0, -1)
}

func TestThreadedBVHClosestHitEquivalence(t *testing.T) {
	s, c := NewStressScene()
	escape := threadedBVHEscapes(s.Nodes)
	threaded := func(ray Ray, tmin, tmax float64) (Hit, bool) {
		if len(s.Nodes) == 0 {
			return Hit{}, false
		}
		ar := prepareRay(ray)
		bestT := tmax
		var best Hit
		found := false
		for ni := 0; ni >= 0 && ni < len(s.Nodes); {
			n := &s.Nodes[ni]
			if ok, _ := n.Box.HitAccel(&ar, tmin, bestT); !ok {
				ni = int(escape[ni])
				continue
			}
			if n.Count > 0 {
				for j := 0; j < n.Count; j++ {
					pi := s.Order[n.Start+j]
					if h, ok := primHitAccel(&s.Primitives[pi], &ar, tmin, bestT); ok {
						h.Primitive = pi
						best, bestT, found = h, h.T, true
					}
				}
				ni = int(escape[ni])
			} else {
				ni = n.Left
			}
		}
		return best, found
	}
	origin := c.Position
	for y := 0; y < 31; y++ {
		for x := 0; x < 47; x++ {
			d := V(float64(x-23)*0.065, float64(y-15)*0.055, -1).Unit()
			r := Ray{O: origin, D: d}
			a, oka := s.Hit(r, 1e-4, 1e30)
			b, okb := threaded(r, 1e-4, 1e30)
			if oka != okb {
				t.Fatalf("hit mismatch at %d,%d", x, y)
			}
			if oka && math.Abs(a.T-b.T) > 1e-9 {
				t.Fatalf("distance mismatch at %d,%d: %.12f vs %.12f", x, y, a.T, b.T)
			}
		}
	}
}

func TestUltraRealismPreset(t *testing.T) {
	cpu := UltraRealismSettings(BackendCPU)
	if cpu.Quality != "Ultra Realism" || cpu.Width != 2560 || cpu.Height != 1440 {
		t.Fatalf("unexpected ultra realism dimensions/label: %+v", cpu)
	}
	if cpu.SPP < 256 || cpu.Bounces < 16 || !cpu.MIS || !cpu.PowerLightSampling || cpu.Sampler != SamplerHalton {
		t.Fatalf("CPU ultra realism transport was weakened: %+v", cpu)
	}
	if cpu.HBAO != 0 || cpu.HBAOPlus != 0 || cpu.FXAA != 0 || cpu.FireflyClamp != 0 {
		t.Fatalf("CPU ultra realism should avoid biased screen-space/clamp effects: %+v", cpu)
	}
	gpu := UltraRealismSettings(BackendGPU)
	if gpu.Backend != BackendGPU || gpu.Sampler != SamplerRandom || gpu.MIS || gpu.PowerLightSampling || gpu.AdaptiveSampling {
		t.Fatalf("GPU ultra realism is not OpenCL-compatible: %+v", gpu)
	}
	if gpu.SPP < 256 || gpu.Bounces < 16 || !gpu.Denoise {
		t.Fatalf("GPU ultra realism quality unexpectedly low: %+v", gpu)
	}
}
