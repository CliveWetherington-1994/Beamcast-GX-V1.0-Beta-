# Beamcast GX 5.3

Beamcast GX is the research/high-throughput branch of Beamcast, a C++17 ray/path-tracing library with three layers:

- **Classic**: readable object-oriented renderer for learning and experiments.
- **HX**: packed multi-core CPU renderer with SAH and Morton BVHs.
- **GX**: wavefront/data-oriented rendering plus compressed acceleration structures, spatiotemporal sampling, streaming, reconstruction, frame-budget control, and portable/native GPU experiments.

GX 5.3 packages the established C++ research engine alongside the hyper-optimized portable Studio path. The C++ research architecture remains intentionally inspectable and testable. The implementations are deliberately inspectable and testable rather than disguised behind a magnificent pile of adjectives.

## New in 5.0: production desktop path + accelerated portable frontend

Beamcast Studio can now rotate the displayed final render in 90-degree steps for easier inspection of portrait, tall, or awkwardly framed outputs. The rotation is available from the **View** menu, the right-side control panel, and the `[` / `]` keys.

Beamcast still ships real application targets by default. GX 5.0 provides two application targets out of the box:

- **Beamcast Studio** - a native desktop GUI with a render viewport, menus, scene presets, OBJ import, render/save commands, quality and resolution controls, denoising/VRS toggles, camera orbit/dolly controls, progress, scene statistics, and render telemetry. The Windows frontend uses Win32 directly; Linux uses X11.
- **beamcast-cli** - a headless/batch renderer for scripts, benchmarks and render farms.

On Windows, `BUILD_WINDOWS.bat` builds `BeamcastStudio.exe` and `beamcast-cli.exe` with MinGW-w64. `RUN_STUDIO.bat` builds on first use and launches the GUI. See `docs/STUDIO.md`.

OBJ imports are triangulated and automatically framed by the camera. Scene edits cancel and join active renders before geometry is replaced, so the GUI does not race against the renderer.

The friendly C++ API from 4.1 remains available:

```cpp
#include <beamcast/beamcast.hpp>

int main() {
    using namespace beamcast;

    easy::Scene scene;
    auto ground = scene.diffuse({0.72f,0.72f,0.72f});
    auto glass  = scene.glass(1.5f);
    auto lamp   = scene.light({15,12,9});

    scene.sphere({0,-1001,-3},1000,ground)
         .sphere({0,0,-3},1,glass)
         .sphere({0,5,-3},0.8f,lamp);

    easy::Camera camera;
    camera.position = {6,3,5};
    camera.target = {0,0,-3};

    auto result = easy::render(scene, camera, easy::Options::preview(1280,720));
    result.save_ppm("render.ppm");
}
```

Quality presets are `Draft`, `Preview`, `Balanced`, `High`, and `Ultra`.

### 4.4 application and practicality work

- native Win32 Studio frontend with standard menus, controls and Windows open/save dialogs;
- native X11 Studio frontend with the same renderer, viewport, menus and inspector;
- asynchronous rendering with progress reporting and safe cancellation between sample batches;
- built-in showcase, Cornell-like and stress-grid scenes;
- OBJ import with polygon triangulation and automatic camera framing;
- headless CLI with quality, resolution, denoise, VRS and output controls;
- GUI/core integration test performs a real render plus OBJ import;
- previous 4.1 allocation, traversal, camera, PPM, callback and stale-BVH fixes are retained.

## GX 4.2 research systems

| Area | Implementation |
|---|---|
| Native backend discovery | CPU/OpenCL/Vulkan/CUDA/HIP runtime probing and device counts |
| Portable GPU reference | OpenCL 1.2 binary/quantized/BVH8 traversal, shading, scan and scatter |
| Vulkan specialization | Compute compaction GLSL, SPIR-V build when `glslc` exists |
| CUDA/HIP specializations | Queue-compaction kernels, auto-built when vendor compilers exist |
| Queue management | Prefix scan, scatter, radix ordering, coherence/cost decision policy |
| Wide acceleration | 160-byte quantized BVH8 nodes, CPU + OpenCL traversal |
| ReSTIR DI | Per-pixel reservoirs, temporal reprojection, re-evaluation, spatial reuse, visibility validation |
| Indirect reuse | World-space radiance cache + directional path-guiding grid |
| Virtual geometry | Paged triangles, top-level cluster BVH, async bounded host/device residency |
| Reconstruction | A-trous, temporal accumulation, motion-vector TAA, optional neural residual inference |
| Frame control | Feedback controller adjusts samples, bounces, foveation and history to a target frame time |
| Price/performance | Frame time, memory, optional RAPL energy, MSE/PSNR/SSIM, FPS/$ and quality/J CSV output |
| Validation | Deterministic tests, randomized ray stress, concurrent paging stress, ASan/UBSan/TSan and `-Werror` builds |

## Build

A normal desktop build produces the Beamcast library, Studio GUI, and CLI:

```bash
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
./build/beamcast-studio   # Linux/X11
./build/beamcast-cli --help
```

On Windows with MinGW-w64, run `BUILD_WINDOWS.bat`, then `RUN_STUDIO.bat`. For a library-only integration use `-DBEAMCAST_BUILD_STUDIO=OFF -DBEAMCAST_BUILD_CLI=OFF`.

For development, examples, and the complete test suite:

```bash
cmake -S . -B build-dev -DCMAKE_BUILD_TYPE=Release \
  -DBEAMCAST_BUILD_EXAMPLES=ON \
  -DBEAMCAST_BUILD_TESTS=ON
cmake --build build-dev -j
ctest --test-dir build-dev --output-on-failure
```

For CPU-specific HX/GX optimization on supported GCC/Clang machines:

```bash
cmake -S . -B build-native \
  -DCMAKE_BUILD_TYPE=Release \
  -DBEAMCAST_HX_NATIVE=ON \
  -DBEAMCAST_HX_ENABLE_AVX2=ON
cmake --build build-native -j
```

Do not distribute that binary to CPUs lacking the selected ISA unless `Illegal instruction` is part of the desired aesthetic.

Optional native GPU kernel builds are opt-in so ordinary library builds stay predictable. Enable them with `-DBEAMCAST_BUILD_NATIVE_GPU_KERNELS=ON`; CMake then detects `glslc`, CUDA and HIP toolchains and skips unavailable specializations without breaking the portable build.

## Minimal GX render

```cpp
#include <beamcast/beamcast.hpp>

int main() {
    using namespace beamcast;

    hx::PackedScene scene;
    auto diffuse = scene.add_lambertian({0.72f, 0.72f, 0.72f});
    auto glass   = scene.add_dielectric(1.5f);
    auto lamp    = scene.add_emissive({18.0f, 15.0f, 11.0f});

    scene.add_sphere({0, -1001, 0}, 1000.0f, diffuse);
    scene.add_sphere({0, 0, -3}, 1.0f, glass);
    scene.add_sphere({0, 5, -3}, 1.0f, lamp);

    hx::PackedScene::BuildOptions build;
    build.mode = hx::BuildMode::SAH;
    build.leaf_size = 4;
    build.sah_bins = 24;
    scene.build(build);

    gx::Settings settings;
    settings.width = 1280;
    settings.height = 720;
    settings.samples_per_pixel = 8;
    settings.max_bounces = 10;
    settings.direct_lighting = gx::DirectLightingMode::ReservoirRIS;
    settings.light_candidates = 6;
    settings.denoise = true;

    Camera camera({6,3,5}, {0,0,-3}, {0,1,0}, 38.0f,
                  float(settings.width) / float(settings.height), 0.02f, 8.0f);

    gx::Result result = gx::render_wavefront(scene, camera, settings);
    return result.image.pixels.empty();
}
```

## Full spatiotemporal ReSTIR DI reference

The original single-pixel RIS path remains available in the wavefront renderer. GX 4.2 additionally exposes a full reference ReSTIR-DI pass:

```cpp
using namespace beamcast;

gx::RestirDIHistory previous;
gx::RestirDIConfig cfg;
cfg.initial_candidates = 8;
cfg.spatial_neighbors = 5;
cfg.temporal_reuse = true;
cfg.spatial_reuse = true;
cfg.visibility_validation = true;

// `frame.guides.position` is generated by GX at the first visible hit.
auto surfaces = gx::surfaces_from_guides(frame.guides);
auto motion = gx::motion_vectors_from_positions(
    frame.guides.position,
    settings.width,
    settings.height,
    current_camera,
    previous_camera);

auto direct = gx::restir_di(
    scene,
    settings.width,
    settings.height,
    surfaces,
    motion,
    &previous,
    cfg);

gx::update_restir_history(
    previous,
    settings.width,
    settings.height,
    surfaces,
    direct.reservoirs);
```

The pass performs fresh light proposals, temporal reservoir reprojection and re-evaluation, spatial reservoir reuse, normal/depth compatibility checks, and visibility validation.

## Quantized BVH8

```cpp
gx::WideBvh8 wide;
wide.build(scene);

hx::PackedHit hit;
if (wide.hit(ray, 0.001f, 1e30f, hit)) {
    // same closest-hit contract as HX traversal
}
```

The layout stores eight conservative quantized child AABBs in a 160-byte structure-of-arrays node. The included benchmark reports build time, occupancy, total bytes, box tests, trace time, and hit equivalence against the binary BVH.

On the current CPU reference benchmark, BVH8 used less storage but traversed more slowly than the binary BVH. That result is preserved rather than cosmetically deleted. Wide quantized BVHs are primarily being investigated for GPU memory bandwidth and occupancy.

## GPU queue compaction and ordering

The portable OpenCL source contains:

- block prefix scan;
- block-offset addition;
- alive-path scatter;
- binary, quantized-binary, and BVH8 traversal;
- material shading and path-state helpers.

The CPU reference exposes the exact compaction contract for testing. `evaluate_queue_ordering()` measures radix-sort cost and estimates divergence savings before recommending a sort, avoiding the surprisingly common optimization strategy of spending 2 ms to save 1 ms.

## Native backend discovery

```cpp
for (const auto& backend : gx::discover_native_backends()) {
    // backend.runtime_library_present
    // backend.compiled_support
    // backend.device_count
    // backend.compute_queue_count
    // backend.detail
}
```

Discovery dynamically probes installed OpenCL, Vulkan, CUDA and HIP runtimes. It does not require all vendor SDK headers merely to report what the host can expose.

On the package-development container, OpenCL and Vulkan loader libraries were installed but no usable physical GPU/platform was exposed. CUDA and HIP runtimes/toolchains were absent. Therefore the portable CPU/reference paths and OpenCL syntax were validated, but end-to-end vendor GPU execution cannot honestly be claimed from that environment.

## Virtualized geometry

`PagedTriangleStore` remains the binary storage layer. GX 4.2 includes `ClusterBvh` and `PageResidencyManager`:

```cpp
gx::PagedTriangleStore store("city.gxp");

gx::ClusterBvh clusters;
clusters.build(store);

gx::PageResidencyManager residency(
    store,
    256ull * 1024 * 1024,  // host budget
    512ull * 1024 * 1024,  // device budget
    upload_callback,
    eviction_callback);

for (std::size_t page : clusters.query(ray, 0.001f, 1e30f))
    residency.request(page);
```

Loading runs asynchronously, residency is bounded, concurrent requests are synchronized, and upload/eviction callbacks let a real Vulkan/CUDA/HIP backend own its device allocator.

## Reconstruction

Available paths:

- edge-aware a-trous filtering;
- bounded temporal accumulation;
- motion-vector `TemporalAA` with normal/depth rejection and neighborhood clamping;
- optional `TinyNeuralDenoiser` residual inference.

The neural path intentionally ships **without fake pretrained weights**. Applications can provide a trained `TinyNeuralModel`; otherwise the deterministic reconstruction paths remain dependency-free.

## Closed-loop frame budgeting

```cpp
gx::FrameBudgetConfig fc;
fc.target_ms = 16.667;
fc.min_samples = 1;
fc.max_samples = 32;

gx::FrameBudgetController controller(fc);

// once per frame
auto decision = controller.update(observed_frame_ms);
controller.apply(settings, decision);
```

The controller adjusts sample count, bounce depth, foveation strength and reconstruction history to recover from over-budget frames and spend more quality when the renderer is comfortably under budget.

## Price/performance harness

Build the examples and run:

```bash
./build/beamcast_price_perf
```

The harness renders a higher-sample reference and a cheaper test, then reports:

- frame time;
- resident process memory;
- optional Linux RAPL energy when the host exposes it;
- MSE;
- PSNR;
- SSIM;
- FPS per purchase dollar;
- quality per joule.

CSV output is designed for repeated runs across integrated graphics, older low-cost discrete GPUs, and current hardware. A sample is stored in `benchmarks/PRICE_PERF_SAMPLE.csv`. Energy is zero/unavailable in that sample because the container did not expose RAPL, rather than because Beamcast has discovered free energy and neglected to notify physics.

## Validation

GX 4.1 includes eight CTest executables:

```text
beamcast_smoke
beamcast_hx_test
beamcast_gx_test
beamcast_gx_abi_test
beamcast_gx_stream_test
beamcast_gx_research_test
beamcast_gx_stress_test
beamcast_easy_test
```

The research/stress coverage includes:

- binary-BVH versus quantized-BVH8 randomized ray equivalence;
- ReSTIR fresh/temporal/spatial reuse;
- generated camera motion vectors;
- radiance cache and path-guide behavior;
- prefix scan/scatter contracts;
- asynchronous bounded page residency;
- concurrent page requests;
- TAA and neural identity behavior;
- frame-controller feedback;
- objective quality metrics;
- GPU ABI/layout invariants.

`benchmarks/VALIDATION.txt` records the final release, warning-clean, sanitizer, ThreadSanitizer, OpenCL syntax, native-backend probe, and clean-archive results used for this package.

## Important limit on the word “bug-free”

No nontrivial renderer can be proven to contain no bugs by running a finite test suite. What this package can honestly claim is narrower: the exercised paths pass deterministic regression tests, randomized stress comparisons, concurrency stress, compiler warnings-as-errors, ASan, UBSan and ThreadSanitizer. Backend code that cannot be compiled or executed because the required vendor hardware/toolchain is absent is explicitly marked as unvalidated rather than granted imaginary immunity.

## Research status

All nine items from the GX 3.0 research roadmap now have concrete implementations. See:

- `docs/RESEARCH_ROADMAP.md` for item-by-item status;
- `docs/GX_ARCHITECTURE.md` for the system design;
- `benchmarks/GX_V4_RESEARCH_RESULTS.txt` for wide-BVH/queue measurements;
- `benchmarks/PRICE_PERF_SAMPLE.csv` for the reference price/performance output;
- `benchmarks/VALIDATION.txt` for the final validation matrix.

A real claim that Beamcast revolutionizes rendering cost would still require independent hardware comparisons against strong baselines at equal image quality. GX 4.1 supplies much more of the machinery required to conduct that experiment, which is considerably more useful than declaring victory over a benchmark nobody else has seen.
