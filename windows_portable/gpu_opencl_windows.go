//go:build windows

package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	clSuccess                       = 0
	clDeviceTypeGPU                 = 1 << 2
	clMemReadOnly                   = 1 << 2
	clMemWriteOnly                  = 1 << 1
	clMemCopyHostPtr                = 1 << 5
	clFalse                         = 0
	clTrue                          = 1
	clDeviceName                    = 0x102B
	clBuildLog                      = 0x1183
	clKernelWorkGroupSize           = 0x11B0
	clKernelPreferredWGSizeMultiple = 0x11B3
)

var openclDLL = syscall.NewLazyDLL("OpenCL.dll")
var (
	clGetPlatformIDs          = openclDLL.NewProc("clGetPlatformIDs")
	clGetDeviceIDs            = openclDLL.NewProc("clGetDeviceIDs")
	clGetDeviceInfo           = openclDLL.NewProc("clGetDeviceInfo")
	clCreateContext           = openclDLL.NewProc("clCreateContext")
	clCreateCommandQueue      = openclDLL.NewProc("clCreateCommandQueue")
	clCreateProgramWithSource = openclDLL.NewProc("clCreateProgramWithSource")
	clBuildProgram            = openclDLL.NewProc("clBuildProgram")
	clGetProgramBuildInfo     = openclDLL.NewProc("clGetProgramBuildInfo")
	clCreateKernel            = openclDLL.NewProc("clCreateKernel")
	clGetKernelWorkGroupInfo  = openclDLL.NewProc("clGetKernelWorkGroupInfo")
	clCreateBuffer            = openclDLL.NewProc("clCreateBuffer")
	clSetKernelArg            = openclDLL.NewProc("clSetKernelArg")
	clEnqueueNDRangeKernel    = openclDLL.NewProc("clEnqueueNDRangeKernel")
	clEnqueueReadBuffer       = openclDLL.NewProc("clEnqueueReadBuffer")
	clEnqueueWriteBuffer      = openclDLL.NewProc("clEnqueueWriteBuffer")
	clReleaseMemObject        = openclDLL.NewProc("clReleaseMemObject")
	clReleaseKernel           = openclDLL.NewProc("clReleaseKernel")
	clReleaseProgram          = openclDLL.NewProc("clReleaseProgram")
	clReleaseCommandQueue     = openclDLL.NewProc("clReleaseCommandQueue")
	clReleaseContext          = openclDLL.NewProc("clReleaseContext")
)

type clNode struct {
	MinX, MinY, MinZ float32
	MaxX, MaxY, MaxZ float32
	Escape           int32
	Meta             uint32
}

type clSphere struct {
	Material                    int32
	Cx, Cy, Cz, Radius2, InvRad float32
	Pad0, Pad1                  float32
}

type clTriangle struct {
	Material      int32
	Ax, Ay, Az    float32
	E1x, E1y, E1z float32
	E2x, E2y, E2z float32
}

type clMaterial struct {
	ColorX, ColorY, ColorZ, Roughness     float32
	Code, EmissionX, EmissionY, EmissionZ float32
}

type clFloat4 struct{ X, Y, Z, W float32 }

// Keep host/device layouts deterministic. Negative array sizes make the Windows
// cross-build fail immediately if Go padding ever changes these ABI-critical records.
var (
	_ [32 - unsafe.Sizeof(clNode{})]byte
	_ [unsafe.Sizeof(clNode{}) - 32]byte
	_ [32 - unsafe.Sizeof(clSphere{})]byte
	_ [unsafe.Sizeof(clSphere{}) - 32]byte
	_ [40 - unsafe.Sizeof(clTriangle{})]byte
	_ [unsafe.Sizeof(clTriangle{}) - 40]byte
	_ [32 - unsafe.Sizeof(clMaterial{})]byte
	_ [unsafe.Sizeof(clMaterial{}) - 32]byte
)

type openCLRuntime struct {
	mu sync.Mutex

	device         uintptr
	ctx            uintptr
	queue          uintptr
	program        uintptr
	kernel         uintptr
	guideKernel    uintptr
	deviceName     string
	localSize      uintptr
	guideLocalSize uintptr

	scene       *Scene
	topologyRev uint64
	geometryRev uint64
	nodeMem     uintptr
	refMem      uintptr
	sphereMem   uintptr
	triMem      uintptr
	matMem      uintptr
	nnode       int32

	nodes   []clNode
	refs    []uint32
	spheres []clSphere
	tris    []clTriangle
	mats    []clMaterial

	outMem        uintptr
	outCapacity   int
	guideMem      uintptr
	guideCapacity int
	raw           []float32
	rawGuide      []uint32
}

var persistentCL openCLRuntime

func firstOpenCLGPU() (uintptr, uintptr, string, error) {
	if err := openclDLL.Load(); err != nil {
		return 0, 0, "", fmt.Errorf("OpenCL.dll not available: %w", err)
	}
	var nplat uint32
	r, _, _ := clGetPlatformIDs.Call(0, 0, uintptr(unsafe.Pointer(&nplat)))
	if int32(r) != clSuccess || nplat == 0 {
		return 0, 0, "", errors.New("no OpenCL platform found")
	}
	plats := make([]uintptr, nplat)
	r, _, _ = clGetPlatformIDs.Call(uintptr(nplat), uintptr(unsafe.Pointer(&plats[0])), 0)
	if int32(r) != clSuccess {
		return 0, 0, "", fmt.Errorf("clGetPlatformIDs failed: %d", int32(r))
	}
	for _, p := range plats {
		var ndev uint32
		rc, _, _ := clGetDeviceIDs.Call(p, clDeviceTypeGPU, 0, 0, uintptr(unsafe.Pointer(&ndev)))
		if int32(rc) != clSuccess || ndev == 0 {
			continue
		}
		devs := make([]uintptr, ndev)
		rc, _, _ = clGetDeviceIDs.Call(p, clDeviceTypeGPU, uintptr(ndev), uintptr(unsafe.Pointer(&devs[0])), 0)
		if int32(rc) != clSuccess || len(devs) == 0 {
			continue
		}
		name := "OpenCL GPU"
		var bytes uintptr
		clGetDeviceInfo.Call(devs[0], clDeviceName, 0, 0, uintptr(unsafe.Pointer(&bytes)))
		if bytes > 1 && bytes < 4096 {
			buf := make([]byte, bytes)
			if rr, _, _ := clGetDeviceInfo.Call(devs[0], clDeviceName, bytes, uintptr(unsafe.Pointer(&buf[0])), 0); int32(rr) == clSuccess {
				name = strings.TrimRight(string(buf), "\x00")
			}
		}
		return p, devs[0], name, nil
	}
	return 0, 0, "", errors.New("no OpenCL GPU device found")
}

func openCLGPUAvailable() (bool, string) {
	_, _, name, err := firstOpenCLGPU()
	if err != nil {
		return false, err.Error()
	}
	return true, name
}

func clErr(code uintptr, what string) error {
	if int32(code) == clSuccess {
		return nil
	}
	return fmt.Errorf("%s failed with OpenCL error %d", what, int32(code))
}

func createCLBuffer(ctx uintptr, flags uintptr, size uintptr, host unsafe.Pointer) (uintptr, error) {
	var errcode int32
	mem, _, _ := clCreateBuffer.Call(ctx, flags, size, uintptr(host), uintptr(unsafe.Pointer(&errcode)))
	if errcode != clSuccess || mem == 0 {
		return 0, fmt.Errorf("clCreateBuffer failed: %d", errcode)
	}
	return mem, nil
}

func setKernelMem(kernel uintptr, index uint32, mem uintptr) error {
	rc, _, _ := clSetKernelArg.Call(kernel, uintptr(index), unsafe.Sizeof(mem), uintptr(unsafe.Pointer(&mem)))
	return clErr(rc, "clSetKernelArg(mem)")
}
func setKernelI32(kernel uintptr, index uint32, value int32) error {
	rc, _, _ := clSetKernelArg.Call(kernel, uintptr(index), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&value)))
	return clErr(rc, "clSetKernelArg(i32)")
}
func setKernelU32(kernel uintptr, index uint32, value uint32) error {
	rc, _, _ := clSetKernelArg.Call(kernel, uintptr(index), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&value)))
	return clErr(rc, "clSetKernelArg(u32)")
}
func setKernelF4(kernel uintptr, index uint32, value clFloat4) error {
	rc, _, _ := clSetKernelArg.Call(kernel, uintptr(index), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&value)))
	return clErr(rc, "clSetKernelArg(float4)")
}

func programBuildLog(program, device uintptr) string {
	var n uintptr
	clGetProgramBuildInfo.Call(program, device, clBuildLog, 0, 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 || n > 1<<20 {
		return ""
	}
	b := make([]byte, n)
	clGetProgramBuildInfo.Call(program, device, clBuildLog, n, uintptr(unsafe.Pointer(&b[0])), 0)
	return strings.TrimRight(string(b), "\x00")
}

func (rt *openCLRuntime) initLocked() error {
	if rt.ctx != 0 && rt.kernel != 0 {
		return nil
	}
	_, device, deviceName, err := firstOpenCLGPU()
	if err != nil {
		return err
	}
	var errcode int32
	ctx, _, _ := clCreateContext.Call(0, 1, uintptr(unsafe.Pointer(&device)), 0, 0, uintptr(unsafe.Pointer(&errcode)))
	if errcode != clSuccess || ctx == 0 {
		return fmt.Errorf("clCreateContext failed: %d", errcode)
	}
	queue, _, _ := clCreateCommandQueue.Call(ctx, device, 0, uintptr(unsafe.Pointer(&errcode)))
	if errcode != clSuccess || queue == 0 {
		clReleaseContext.Call(ctx)
		return fmt.Errorf("clCreateCommandQueue failed: %d", errcode)
	}
	src := []byte(openCLKernelSource + "\x00")
	srcPtr := uintptr(unsafe.Pointer(&src[0]))
	srcLen := uintptr(len(src) - 1)
	program, _, _ := clCreateProgramWithSource.Call(ctx, 1, uintptr(unsafe.Pointer(&srcPtr)), uintptr(unsafe.Pointer(&srcLen)), uintptr(unsafe.Pointer(&errcode)))
	if errcode != clSuccess || program == 0 {
		clReleaseCommandQueue.Call(queue)
		clReleaseContext.Call(ctx)
		return fmt.Errorf("clCreateProgramWithSource failed: %d", errcode)
	}
	// Rendering math tolerates relaxed IEEE semantics well; this allows fused MAD/native math paths on drivers that expose them.
	opts := []byte("-cl-fast-relaxed-math -cl-mad-enable\x00")
	rc, _, _ := clBuildProgram.Call(program, 1, uintptr(unsafe.Pointer(&device)), uintptr(unsafe.Pointer(&opts[0])), 0, 0)
	if int32(rc) != clSuccess {
		log := programBuildLog(program, device)
		clReleaseProgram.Call(program)
		clReleaseCommandQueue.Call(queue)
		clReleaseContext.Call(ctx)
		return fmt.Errorf("OpenCL kernel build failed: %d\n%s", int32(rc), log)
	}
	makeKernel := func(name string) (uintptr, error) {
		kname := append([]byte(name), 0)
		k, _, _ := clCreateKernel.Call(program, uintptr(unsafe.Pointer(&kname[0])), uintptr(unsafe.Pointer(&errcode)))
		if errcode != clSuccess || k == 0 {
			return 0, fmt.Errorf("clCreateKernel(%s) failed: %d", name, errcode)
		}
		return k, nil
	}
	kernel, err := makeKernel("beamcast_render")
	if err != nil {
		clReleaseProgram.Call(program)
		clReleaseCommandQueue.Call(queue)
		clReleaseContext.Call(ctx)
		return err
	}
	guideKernel, err := makeKernel("beamcast_render_guides")
	if err != nil {
		clReleaseKernel.Call(kernel)
		clReleaseProgram.Call(program)
		clReleaseCommandQueue.Call(queue)
		clReleaseContext.Call(ctx)
		return err
	}

	chooseLocal := func(k uintptr) uintptr {
		var preferred uintptr
		var maxWG uintptr
		clGetKernelWorkGroupInfo.Call(k, device, clKernelPreferredWGSizeMultiple, unsafe.Sizeof(preferred), uintptr(unsafe.Pointer(&preferred)), 0)
		clGetKernelWorkGroupInfo.Call(k, device, clKernelWorkGroupSize, unsafe.Sizeof(maxWG), uintptr(unsafe.Pointer(&maxWG)), 0)
		local := preferred
		if local == 0 {
			local = 32
		}
		if local > 64 {
			local = 64
		}
		if maxWG > 0 && local > maxWG {
			local = maxWG
		}
		if local == 0 {
			local = 1
		}
		return local
	}

	rt.device = device
	rt.deviceName = deviceName
	rt.ctx = ctx
	rt.queue = queue
	rt.program = program
	rt.kernel = kernel
	rt.guideKernel = guideKernel
	rt.localSize = chooseLocal(kernel)
	rt.guideLocalSize = chooseLocal(guideKernel)
	return nil
}

func (rt *openCLRuntime) releaseSceneLocked() {
	for _, mem := range []uintptr{rt.nodeMem, rt.refMem, rt.sphereMem, rt.triMem, rt.matMem} {
		if mem != 0 {
			clReleaseMemObject.Call(mem)
		}
	}
	rt.nodeMem, rt.refMem, rt.sphereMem, rt.triMem, rt.matMem = 0, 0, 0, 0, 0
	rt.scene = nil
	rt.topologyRev, rt.geometryRev = 0, 0
	rt.nnode = 0
}

func resizeSlice[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	return s[:n]
}

func (rt *openCLRuntime) packGeometryLocked(scene *Scene) {
	escapes := threadedBVHEscapes(scene.Nodes)
	rt.nodes = resizeSlice(rt.nodes, len(scene.Nodes))
	for i, n := range scene.Nodes {
		escape := int32(-1)
		if i < len(escapes) {
			escape = escapes[i]
		}
		var meta uint32
		if n.Count > 0 {
			meta = 0x80000000 | (uint32(n.Count-1)&0xf)<<27 | (uint32(n.Start) & 0x07ffffff)
		} else {
			meta = uint32(n.Left) & 0x07ffffff
		}
		rt.nodes[i] = clNode{float32(n.Box.Min.X), float32(n.Box.Min.Y), float32(n.Box.Min.Z), float32(n.Box.Max.X), float32(n.Box.Max.Y), float32(n.Box.Max.Z), escape, meta}
	}

	rt.refs = resizeSlice(rt.refs, len(scene.Order))
	rt.spheres = rt.spheres[:0]
	rt.tris = rt.tris[:0]
	for i, src := range scene.Order {
		if src < 0 || src >= len(scene.Primitives) {
			rt.refs[i] = 0
			continue
		}
		p := &scene.Primitives[src]
		if p.Kind == PrimTriangle {
			idx := len(rt.tris)
			rt.tris = append(rt.tris, clTriangle{int32(p.Material), float32(p.A.X), float32(p.A.Y), float32(p.A.Z), float32(p.E1.X), float32(p.E1.Y), float32(p.E1.Z), float32(p.E2.X), float32(p.E2.Y), float32(p.E2.Z)})
			rt.refs[i] = 0x80000000 | uint32(idx)
		} else {
			idx := len(rt.spheres)
			invRad := float32(0)
			if p.Radius > 0 {
				invRad = float32(1.0 / p.Radius)
			}
			rt.spheres = append(rt.spheres, clSphere{int32(p.Material), float32(p.Center.X), float32(p.Center.Y), float32(p.Center.Z), float32(p.Radius * p.Radius), invRad, 0, 0})
			rt.refs[i] = uint32(idx)
		}
	}
}

func (rt *openCLRuntime) uploadFullSceneLocked(scene *Scene) error {
	rt.releaseSceneLocked()
	rt.packGeometryLocked(scene)
	rt.mats = resizeSlice(rt.mats, len(scene.Materials))
	for i, m := range scene.Materials {
		code := float32(0)
		switch m.Kind {
		case MatMetal:
			code = -1
		case MatEmissive:
			code = -2
		case MatGlass:
			code = float32(m.IOR)
			if code <= 1 {
				code = 1.5
			}
		}
		rt.mats[i] = clMaterial{float32(m.Color.X), float32(m.Color.Y), float32(m.Color.Z), float32(m.Roughness), code, float32(m.Emission.X), float32(m.Emission.Y), float32(m.Emission.Z)}
	}
	makeRO := func(ptr unsafe.Pointer, bytes uintptr) (uintptr, error) {
		return createCLBuffer(rt.ctx, clMemReadOnly|clMemCopyHostPtr, bytes, ptr)
	}
	var err error
	rt.nodeMem, err = makeRO(unsafe.Pointer(&rt.nodes[0]), uintptr(len(rt.nodes))*unsafe.Sizeof(rt.nodes[0]))
	if err != nil {
		rt.releaseSceneLocked()
		return err
	}
	rt.refMem, err = makeRO(unsafe.Pointer(&rt.refs[0]), uintptr(len(rt.refs))*unsafe.Sizeof(rt.refs[0]))
	if err != nil {
		rt.releaseSceneLocked()
		return err
	}
	if len(rt.spheres) > 0 {
		rt.sphereMem, err = makeRO(unsafe.Pointer(&rt.spheres[0]), uintptr(len(rt.spheres))*unsafe.Sizeof(rt.spheres[0]))
	} else {
		dummy := clSphere{}
		rt.sphereMem, err = makeRO(unsafe.Pointer(&dummy), unsafe.Sizeof(dummy))
	}
	if err != nil {
		rt.releaseSceneLocked()
		return err
	}
	if len(rt.tris) > 0 {
		rt.triMem, err = makeRO(unsafe.Pointer(&rt.tris[0]), uintptr(len(rt.tris))*unsafe.Sizeof(rt.tris[0]))
	} else {
		dummy := clTriangle{}
		rt.triMem, err = makeRO(unsafe.Pointer(&dummy), unsafe.Sizeof(dummy))
	}
	if err != nil {
		rt.releaseSceneLocked()
		return err
	}
	if len(rt.mats) == 0 {
		return errors.New("OpenCL backend requires at least one material")
	}
	rt.matMem, err = makeRO(unsafe.Pointer(&rt.mats[0]), uintptr(len(rt.mats))*unsafe.Sizeof(rt.mats[0]))
	if err != nil {
		rt.releaseSceneLocked()
		return err
	}
	rt.scene = scene
	rt.topologyRev = scene.GPUTopologyRevision
	rt.geometryRev = scene.GPUGeometryRevision
	rt.nnode = int32(len(scene.Nodes))
	return nil
}

func (rt *openCLRuntime) updateGeometryLocked(scene *Scene) error {
	rt.packGeometryLocked(scene)
	write := func(mem uintptr, ptr unsafe.Pointer, bytes uintptr) error {
		rc, _, _ := clEnqueueWriteBuffer.Call(rt.queue, mem, clFalse, 0, bytes, uintptr(ptr), 0, 0, 0)
		return clErr(rc, "clEnqueueWriteBuffer")
	}
	if err := write(rt.nodeMem, unsafe.Pointer(&rt.nodes[0]), uintptr(len(rt.nodes))*unsafe.Sizeof(rt.nodes[0])); err != nil {
		return err
	}
	if len(rt.spheres) > 0 {
		if err := write(rt.sphereMem, unsafe.Pointer(&rt.spheres[0]), uintptr(len(rt.spheres))*unsafe.Sizeof(rt.spheres[0])); err != nil {
			return err
		}
	}
	if len(rt.tris) > 0 {
		if err := write(rt.triMem, unsafe.Pointer(&rt.tris[0]), uintptr(len(rt.tris))*unsafe.Sizeof(rt.tris[0])); err != nil {
			return err
		}
	}
	rt.geometryRev = scene.GPUGeometryRevision
	return nil
}

func validateGPUSceneEncoding(scene *Scene) error {
	const maxIndex = 1 << 27
	if len(scene.Nodes) >= maxIndex || len(scene.Order) >= maxIndex {
		return fmt.Errorf("OpenCL compact BVH supports fewer than %d nodes/primitives", maxIndex)
	}
	for i := range scene.Nodes {
		n := &scene.Nodes[i]
		if n.Count < 0 || n.Count > 16 {
			return fmt.Errorf("OpenCL compact BVH leaf %d has unsupported count %d", i, n.Count)
		}
	}
	return nil
}

func (rt *openCLRuntime) ensureSceneLocked(scene *Scene) error {
	if err := validateGPUSceneEncoding(scene); err != nil {
		return err
	}
	if rt.scene != scene || rt.topologyRev != scene.GPUTopologyRevision || rt.nodeMem == 0 || rt.refMem == 0 || rt.sphereMem == 0 || rt.triMem == 0 {
		return rt.uploadFullSceneLocked(scene)
	}
	if rt.geometryRev != scene.GPUGeometryRevision {
		return rt.updateGeometryLocked(scene)
	}
	return nil
}

func (rt *openCLRuntime) ensureOutputLocked(pixelCount int, needGuides bool) error {
	if pixelCount <= 0 {
		return errors.New("invalid OpenCL output size")
	}
	if rt.outMem == 0 || pixelCount > rt.outCapacity {
		if rt.outMem != 0 {
			clReleaseMemObject.Call(rt.outMem)
		}
		bytes := uintptr(pixelCount) * unsafe.Sizeof(clFloat4{})
		mem, err := createCLBuffer(rt.ctx, clMemWriteOnly, bytes, nil)
		if err != nil {
			rt.outMem = 0
			rt.outCapacity = 0
			return err
		}
		rt.outMem = mem
		rt.outCapacity = pixelCount
	}
	if needGuides && (rt.guideMem == 0 || pixelCount > rt.guideCapacity) {
		if rt.guideMem != 0 {
			clReleaseMemObject.Call(rt.guideMem)
		}
		// Compact guide payload: depth (float bits), packed oct-normal/material,
		// packed 16-bit primary-ray jitter. 12 bytes/pixel instead of 16.
		bytes := uintptr(pixelCount*3) * unsafe.Sizeof(uint32(0))
		mem, err := createCLBuffer(rt.ctx, clMemWriteOnly, bytes, nil)
		if err != nil {
			rt.guideMem = 0
			rt.guideCapacity = 0
			return err
		}
		rt.guideMem = mem
		rt.guideCapacity = pixelCount
	}
	if cap(rt.raw) < pixelCount*4 {
		rt.raw = make([]float32, pixelCount*4)
	} else {
		rt.raw = rt.raw[:pixelCount*4]
	}
	if needGuides {
		if cap(rt.rawGuide) < pixelCount*3 {
			rt.rawGuide = make([]uint32, pixelCount*3)
		} else {
			rt.rawGuide = rt.rawGuide[:pixelCount*3]
		}
	}
	return nil
}

func roundUp(v, multiple uintptr) uintptr {
	if multiple <= 1 {
		return v
	}
	return (v + multiple - 1) / multiple * multiple
}

func decodeOctNormal(qx, qy uint32) Vec3 {
	x := float64(qx)/1023.0*2 - 1
	y := float64(qy)/1023.0*2 - 1
	z := 1 - math.Abs(x) - math.Abs(y)
	if z < 0 {
		ox, oy := x, y
		x = (1 - math.Abs(oy)) * math.Copysign(1, ox)
		y = (1 - math.Abs(ox)) * math.Copysign(1, oy)
	}
	return V(x, y, z).Unit()
}

func renderOpenCL(scene *Scene, cam Camera, settings RenderSettings, cancel *atomic.Bool,
	onChunk func(y0, rows int, data []Vec3, progress float64)) ([]Vec3, []Guide, int64, string, error) {
	if scene == nil || len(scene.Primitives) == 0 || len(scene.Nodes) == 0 {
		return nil, nil, 0, "", errors.New("OpenCL backend requires a non-empty built scene")
	}
	rt := &persistentCL
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if err := rt.initLocked(); err != nil {
		return nil, nil, 0, "", err
	}
	if err := rt.ensureSceneLocked(scene); err != nil {
		return nil, nil, 0, rt.deviceName, err
	}

	w, h := internalSize(settings)
	needGuides := settings.Denoise || settings.HBAO > 0 || settings.HBAOPlus > 0 || temporalWeight(settings) > 0 || settings.DebugView != DebugBeauty
	if err := rt.ensureOutputLocked(w*h, needGuides); err != nil {
		return nil, nil, 0, rt.deviceName, err
	}
	aspect := float64(w) / float64(h)
	origin, lower, horiz, vert := cam.basis(aspect)
	f4 := func(v Vec3) clFloat4 { return clFloat4{float32(v.X), float32(v.Y), float32(v.Z), 0} }

	kernel := rt.kernel
	localSize := rt.localSize
	if needGuides {
		kernel = rt.guideKernel
		localSize = rt.guideLocalSize
	}
	staticArgs := []func() error{
		func() error { return setKernelMem(kernel, 0, rt.nodeMem) },
		func() error { return setKernelI32(kernel, 1, rt.nnode) },
		func() error { return setKernelMem(kernel, 2, rt.refMem) },
		func() error { return setKernelMem(kernel, 3, rt.sphereMem) },
		func() error { return setKernelMem(kernel, 4, rt.triMem) },
		func() error { return setKernelMem(kernel, 5, rt.matMem) },
		func() error { return setKernelI32(kernel, 6, int32(w)) },
		func() error { return setKernelI32(kernel, 7, int32(h)) },
		func() error { return setKernelI32(kernel, 8, int32(effectiveSPP(&settings))) },
		func() error { return setKernelI32(kernel, 9, int32(settings.Bounces)) },
		func() error { return setKernelU32(kernel, 10, uint32(0x9e3779b9)) },
		func() error { return setKernelF4(kernel, 11, f4(origin)) },
		func() error { return setKernelF4(kernel, 12, f4(lower)) },
		func() error { return setKernelF4(kernel, 13, f4(horiz)) },
		func() error { return setKernelF4(kernel, 14, f4(vert)) },
		func() error { return setKernelMem(kernel, 15, rt.outMem) },
	}
	if needGuides {
		staticArgs = append(staticArgs, func() error { return setKernelMem(kernel, 16, rt.guideMem) })
	}
	for _, fn := range staticArgs {
		if err := fn(); err != nil {
			return nil, nil, 0, rt.deviceName, err
		}
	}
	yArg, rowsArg := uint32(16), uint32(17)
	if needGuides {
		yArg, rowsArg = 17, 18
	}

	linear := make([]Vec3, w*h)
	var guides []Guide
	if needGuides {
		guides = make([]Guide, w*h)
	}
	chunkRows := 64
	if !settings.ProgressiveUpdates {
		chunkRows = h
	} else if settings.PublishHz <= 15 {
		chunkRows = 128
	} else if settings.PublishHz >= 60 {
		chunkRows = 32
	}
	if chunkRows < 1 {
		chunkRows = 1
	}
	var rays int64
	for y0 := 0; y0 < h; y0 += chunkRows {
		if cancel != nil && cancel.Load() {
			return nil, nil, 0, rt.deviceName, errors.New("render cancelled")
		}
		rows := chunkRows
		if y0+rows > h {
			rows = h - y0
		}
		if err := setKernelI32(kernel, yArg, int32(y0)); err != nil {
			return nil, nil, 0, rt.deviceName, err
		}
		if err := setKernelI32(kernel, rowsArg, int32(rows)); err != nil {
			return nil, nil, 0, rt.deviceName, err
		}
		global := [2]uintptr{roundUp(uintptr(w), localSize), uintptr(rows)}
		local := [2]uintptr{localSize, 1}
		rc, _, _ := clEnqueueNDRangeKernel.Call(rt.queue, kernel, 2, 0, uintptr(unsafe.Pointer(&global[0])), uintptr(unsafe.Pointer(&local[0])), 0, 0, 0)
		if err := clErr(rc, "clEnqueueNDRangeKernel"); err != nil {
			return nil, nil, 0, rt.deviceName, err
		}

		basePixel := y0 * w
		pixelN := rows * w
		offFloats := basePixel * 4
		sizeFloats := pixelN * 4
		if sizeFloats == 0 {
			continue
		}
		rc, _, _ = clEnqueueReadBuffer.Call(rt.queue, rt.outMem, clTrue, uintptr(offFloats)*unsafe.Sizeof(float32(0)), uintptr(sizeFloats)*unsafe.Sizeof(float32(0)), uintptr(unsafe.Pointer(&rt.raw[offFloats])), 0, 0, 0)
		if err := clErr(rc, "clEnqueueReadBuffer(color+telemetry)"); err != nil {
			return nil, nil, 0, rt.deviceName, err
		}
		for i := 0; i < pixelN; i++ {
			base := offFloats + i*4
			linear[basePixel+i] = V(float64(rt.raw[base]), float64(rt.raw[base+1]), float64(rt.raw[base+2]))
			rays += int64(rt.raw[base+3] + 0.5)
		}

		if needGuides {
			offGuide := basePixel * 3
			sizeGuide := pixelN * 3
			rc, _, _ = clEnqueueReadBuffer.Call(rt.queue, rt.guideMem, clTrue, uintptr(offGuide)*unsafe.Sizeof(uint32(0)), uintptr(sizeGuide)*unsafe.Sizeof(uint32(0)), uintptr(unsafe.Pointer(&rt.rawGuide[offGuide])), 0, 0, 0)
			if err := clErr(rc, "clEnqueueReadBuffer(compact guides)"); err != nil {
				return nil, nil, 0, rt.deviceName, err
			}
			invW := 1.0 / float64(iMax(w-1, 1))
			invH := 1.0 / float64(iMax(h-1, 1))
			for i := 0; i < pixelN; i++ {
				base := offGuide + i*3
				packed := rt.rawGuide[base+1]
				if packed&0x80000000 == 0 {
					continue
				}
				depth := float64(math.Float32frombits(rt.rawGuide[base]))
				qx := packed & 1023
				qy := (packed >> 10) & 1023
				mi := int((packed >> 20) & 2047)
				n := decodeOctNormal(qx, qy)
				jitter := rt.rawGuide[base+2]
				ju := float64(jitter&0xffff) / 65535.0
				jv := float64(jitter>>16) / 65535.0
				pix := basePixel + i
				y := pix / w
				x := pix - y*w
				u := (float64(x) + ju) * invW
				v := (float64(h-1-y) + jv) * invH
				dir := lower.Add(horiz.Mul(u)).Add(vert.Mul(v)).Sub(origin).Unit()
				pos := origin.Add(dir.Mul(depth))
				albedo := V(0.8, 0.8, 0.8)
				if mi >= 0 && mi < len(scene.Materials) {
					albedo = materialPreviewColor(&scene.Materials[mi])
				}
				guides[pix] = Guide{P: pos, N: n, Albedo: albedo, Depth: depth, Valid: true}
			}
		}
		if onChunk != nil {
			onChunk(y0, rows, linear[basePixel:basePixel+pixelN], float64(y0+rows)/float64(h))
		}
	}
	return linear, guides, rays, rt.deviceName, nil
}

const openCLKernelSource = `
typedef struct { float minx,miny,minz,maxx,maxy,maxz; int escape; uint meta; } Node;
typedef struct { int material; float cx,cy,cz,radius2,invrad,pad0,pad1; } Sphere;
typedef struct { int material; float ax,ay,az,e1x,e1y,e1z,e2x,e2y,e2z; } Tri;
typedef struct { float cr,cg,cb,rough,code,er,eg,eb; } Mat;

static inline uint hsh(uint x){ x ^= x>>16; x*=0x7feb352dU; x^=x>>15; x*=0x846ca68bU; x^=x>>16; return x; }
static inline float rnd(uint* s){ uint x=*s; x^=x<<13; x^=x>>17; x^=x<<5; *s=x; return (float)(x>>8)*(1.0f/16777216.0f); }
static inline float3 unitv(float3 v){ return v*native_rsqrt(fmax(dot(v,v),1e-20f)); }
static inline float3 bg(float3 d){ float t=.5f*(d.y+1.f); return mad((float3)(.26f,.40f,.60f),(float3)(t),(float3)(.06f,.08f,.12f)); }
static inline float safeinv(float d){ return fabs(d)<1e-8f ? copysign(1e30f,d) : native_recip(d); }
static inline int boxhit(__global const Node* n,float3 ro,float3 invd,float tmin,float tmax){
  float3 mn=(float3)(n->minx,n->miny,n->minz),mx=(float3)(n->maxx,n->maxy,n->maxz);
  float3 a=mad(mn-ro,invd,(float3)(0)),b=mad(mx-ro,invd,(float3)(0));
  float3 lo=fmin(a,b),hi=fmax(a,b);
  float enter=fmax(tmin,fmax(lo.x,fmax(lo.y,lo.z)));
  float exit=fmin(tmax,fmin(hi.x,fmin(hi.y,hi.z)));
  return exit>enter;
}
static inline int phit(uint ref,__global const Sphere* spheres,__global const Tri* tris,float3 ro,float3 rd,float tmin,float tmax,float* ot,float3* on,int* ofront,int* omat){
  if((ref&0x80000000U)==0){
    __global const Sphere* p=spheres+(ref&0x7fffffffU);float3 c=(float3)(p->cx,p->cy,p->cz),oc=ro-c;float hb=dot(oc,rd),cc=dot(oc,oc)-p->radius2;
    float disc=mad(hb,hb,-cc);if(disc<0)return 0;float sq=native_sqrt(disc),tt=-hb-sq;
    if(tt<tmin||tt>tmax){tt=-hb+sq;if(tt<tmin||tt>tmax)return 0;}
    float3 raw=(mad(rd,(float3)(tt),ro)-c)*p->invrad;int front=dot(rd,raw)<0;*ot=tt;*on=front?raw:-raw;*ofront=front;*omat=p->material;return 1;
  }
  __global const Tri* p=tris+(ref&0x7fffffffU);float3 a0=(float3)(p->ax,p->ay,p->az),e1=(float3)(p->e1x,p->e1y,p->e1z),e2=(float3)(p->e2x,p->e2y,p->e2z);
  float3 hh=cross(rd,e2);float det=dot(e1,hh);if(fabs(det)<1e-7f)return 0;
  float f=native_recip(det);float3 s=ro-a0;float u=f*dot(s,hh);if(u<0||u>1)return 0;
  float3 q=cross(s,e1);float v=f*dot(rd,q);if(v<0||u+v>1)return 0;float tt=f*dot(e2,q);if(tt<tmin||tt>tmax)return 0;
  float3 raw=unitv(cross(e1,e2));int front=dot(rd,raw)<0;*ot=tt;*on=front?raw:-raw;*ofront=front;*omat=p->material;return 1;
}

static inline int closest(__global const Node* restrict nodes,int nnode,__global const uint* restrict refs,__global const Sphere* restrict spheres,__global const Tri* restrict tris,float3 ro,float3 rd,float tmax,float* ot,float3* on,int* om,int* ofront){
  float3 invd=(float3)(safeinv(rd.x),safeinv(rd.y),safeinv(rd.z));
  float best=tmax;int ok=0,bm=-1,bf=1;float3 bn=(float3)(0);int ni=0;
  while(ni>=0&&ni<nnode){
    __global const Node* n=nodes+ni;
    if(!boxhit(n,ro,invd,1e-4f,best)){ni=n->escape;continue;}
    uint meta=n->meta;
    if(meta&0x80000000U){
      int count=(int)(((meta>>27)&15U)+1U);int start=(int)(meta&0x07ffffffU);
      for(int i=0;i<count;i++){
        uint ref=refs[start+i];float tt;float3 nn;int front,mat;
        if(phit(ref,spheres,tris,ro,rd,1e-4f,best,&tt,&nn,&front,&mat)){best=tt;bn=nn;bm=mat;bf=front;ok=1;}
      }
      ni=n->escape;
    }else{
      ni=(int)(meta&0x07ffffffU);
    }
  }
  *ot=best;*on=bn;*om=bm;*ofront=bf;return ok;
}

static inline float2 disk(uint* state){
  for(int i=0;i<12;i++){float2 p=(float2)(mad(2.f,rnd(state),-1.f),mad(2.f,rnd(state),-1.f));if(dot(p,p)<1)return p;}
  return (float2)(0,0);
}
static inline float3 cosineDir(float3 n,uint* state){
  float2 d=disk(state);float z=native_sqrt(fmax(0.f,1-dot(d,d)));float3 a=fabs(n.y)>.9f?(float3)(1,0,0):(float3)(0,1,0);float3 t=unitv(cross(a,n));float3 b=cross(n,t);return mad(t,(float3)(d.x),mad(b,(float3)(d.y),n*z));
}
static inline float3 randomUnit(uint* s){
  for(int i=0;i<12;i++){float x=mad(2.f,rnd(s),-1.f),y=mad(2.f,rnd(s),-1.f),q=mad(x,x,y*y);if(q>0&&q<1){float k=2*native_sqrt(1-q);return (float3)(x*k,y*k,1-2*q);}}
  return (float3)(0,0,1);
}
static inline float schlick(float c,float ref){float r=(1-ref)/(1+ref);r*=r;float x=1-c,x2=x*x;return mad(1-r,x2*x2*x,r);}
static inline uint packGuideNormalMat(float3 n,int mat){
  n*=native_recip(fabs(n.x)+fabs(n.y)+fabs(n.z)+1e-20f);float2 p=n.xy;
  if(n.z<0){float2 a=(float2)(1-fabs(p.y),1-fabs(p.x));p=(float2)(copysign(a.x,p.x),copysign(a.y,p.y));}
  p=clamp(mad(p,(float2)(.5f),(float2)(.5f)),0.f,1.f);uint qx=convert_uint_rte(p.x*1023.f),qy=convert_uint_rte(p.y*1023.f);uint m=(uint)clamp(mat,0,2047);
  return 0x80000000U|(m<<20)|(qy<<10)|qx;
}
static inline uint packJitter(float ju,float jv){uint a=convert_uint_sat_rte(clamp(ju,0.f,1.f)*65535.f),b=convert_uint_sat_rte(clamp(jv,0.f,1.f)*65535.f);return (b<<16)|a;}
static inline float3 refr(float3 uv,float3 n,float eta){float ct=fmin(dot(-uv,n),1.f);float3 perp=eta*mad((float3)(ct),n,uv);float3 para=-native_sqrt(fabs(1-dot(perp,perp)))*n;return perp+para;}

static inline void renderPixel(
  __global const Node* restrict nodes,int nnode,__global const uint* restrict refs,__global const Sphere* restrict spheres,__global const Tri* restrict tris,__global const Mat* restrict mats,
  int width,int height,int spp,int bounces,uint seed,float4 origin4,float4 lower4,float4 horiz4,float4 vert4,
  __global float4* restrict out,__global uint* restrict guideOut,int writeGuides,int yOffset,int rows){
  int x=get_global_id(0),ly=get_global_id(1);if(x>=width||ly>=rows)return;int y=yOffset+ly;if(y>=height)return;int idx=mad24(y,width,x);
  uint state=hsh(seed^(uint)(idx*747796405U+2891336453U))|1U;float3 sum=(float3)(0);uint rayCount=0;float3 origin=origin4.xyz,lower=lower4.xyz,horiz=horiz4.xyz,vert=vert4.xyz;
  float firstT=0.f;uint firstPacked=0,firstJitter=0;float invW=native_recip((float)max(width-1,1)),invH=native_recip((float)max(height-1,1));
  for(int s=0;s<spp;s++){
    float ju=rnd(&state),jv=rnd(&state);float u=((float)x+ju)*invW,v=((float)(height-1-y)+jv)*invH;float3 ro=origin;float3 rd=unitv(mad(horiz,(float3)(u),mad(vert,(float3)(v),lower-origin)));float3 throughput=(float3)(1),rad=(float3)(0);
    for(int bounce=0;bounce<bounces;bounce++){
      rayCount++;float tt;float3 n;int mi,front;if(!closest(nodes,nnode,refs,spheres,tris,ro,rd,1e30f,&tt,&n,&mi,&front)){rad=mad(throughput,bg(rd),rad);break;}
      float3 pos=mad(rd,(float3)(tt),ro);if(writeGuides&&s==0&&bounce==0){firstT=tt;firstPacked=packGuideNormalMat(n,mi);firstJitter=packJitter(ju,jv);}__global const Mat* m=mats+mi;float code=m->code;
      if(code==-2.f){rad=mad(throughput,(float3)(m->er,m->eg,m->eb),rad);break;}
      if(code==0.f){throughput*=(float3)(m->cr,m->cg,m->cb);rd=cosineDir(n,&state);ro=mad(n,(float3)(2e-4f),pos);}
      else if(code<0.f){float3 refl=mad((float3)(-2*dot(rd,n)),n,rd);rd=unitv(mad(randomUnit(&state),(float3)(clamp(m->rough,0.f,1.f)),refl));if(dot(rd,n)<=0)break;throughput*=(float3)(m->cr,m->cg,m->cb);ro=mad(n,(float3)(2e-4f),pos);}
      else{float eta=code;float ratio=front?native_recip(eta):eta;float ct=fmin(dot(-rd,n),1.f),st=native_sqrt(fmax(0.f,1-ct*ct));if(ratio*st>1||schlick(ct,ratio)>rnd(&state))rd=mad((float3)(-2*dot(rd,n)),n,rd);else rd=unitv(refr(rd,n,ratio));throughput*=(float3)(m->cr,m->cg,m->cb);ro=mad(rd,(float3)(2e-4f),pos);}
      if(bounce>=4){float p=clamp(fmax(throughput.x,fmax(throughput.y,throughput.z)),.08f,.95f);if(rnd(&state)>p)break;throughput*=native_recip(p);}
    }
    sum+=rad;
  }
  sum*=native_recip((float)spp);out[idx]=(float4)(sum.x,sum.y,sum.z,(float)rayCount);
  if(writeGuides){int g=idx*3;guideOut[g]=as_uint(firstT);guideOut[g+1]=firstPacked;guideOut[g+2]=firstJitter;}
}

__kernel void beamcast_render(
  __global const Node* restrict nodes,int nnode,__global const uint* restrict refs,__global const Sphere* restrict spheres,__global const Tri* restrict tris,__global const Mat* restrict mats,
  int width,int height,int spp,int bounces,uint seed,float4 origin4,float4 lower4,float4 horiz4,float4 vert4,
  __global float4* restrict out,int yOffset,int rows){
  renderPixel(nodes,nnode,refs,spheres,tris,mats,width,height,spp,bounces,seed,origin4,lower4,horiz4,vert4,out,(__global uint*)0,0,yOffset,rows);
}

__kernel void beamcast_render_guides(
  __global const Node* restrict nodes,int nnode,__global const uint* restrict refs,__global const Sphere* restrict spheres,__global const Tri* restrict tris,__global const Mat* restrict mats,
  int width,int height,int spp,int bounces,uint seed,float4 origin4,float4 lower4,float4 horiz4,float4 vert4,
  __global float4* restrict out,__global uint* restrict guideOut,int yOffset,int rows){
  renderPixel(nodes,nnode,refs,spheres,tris,mats,width,height,spp,bounces,seed,origin4,lower4,horiz4,vert4,out,guideOut,1,yOffset,rows);
}
`
