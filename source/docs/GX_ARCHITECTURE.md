# Beamcast GX 4.1 architecture

## Design goal

GX treats rendering as a resource-allocation problem: rays, memory bandwidth, queue coherence, temporal history and reconstruction work should be spent where they produce measurable image quality. The architecture therefore separates transport from scheduling and makes reuse, streaming and quality control explicit.

The system retains the Classic and HX renderers. GX layers research-oriented scheduling and reconstruction systems on top of HX's packed scene representation rather than replacing every working component merely to make the version number feel important.

## Major data flow

A typical interactive frame can use the following pipeline:

1. Build or reuse an HX packed scene and flattened binary BVH.
2. Optionally derive a quantized BVH8 for bandwidth-oriented traversal.
3. Generate primary paths into structure-of-arrays wavefront queues.
4. Intersect/shade active paths in persistent worker chunks or a GPU backend.
5. Perform direct illumination through alias-MIS, local reservoir RIS, or the spatiotemporal ReSTIR-DI reference pass.
6. Compact live paths with prefix-sum/scatter.
7. Estimate queue coherence and radix-sort only when predicted divergence savings exceed the measured sort cost.
8. Reuse indirect information through a radiance cache and/or path-guide grid.
9. Fetch virtualized geometry pages through the cluster BVH and residency manager when a streaming integration is active.
10. Produce first-hit world position, normal, albedo and depth guides.
11. Reconstruct with a-trous filtering, temporal accumulation, motion-vector TAA, or optional neural residual inference.
12. Feed observed frame time into the frame-budget controller for the next frame's samples, bounce limit, foveation and history length.
13. Record time, memory, energy when available, and objective image-quality metrics for price/performance comparisons.

Not every application needs every stage. The point is that each stage has an explicit contract and can be benchmarked independently.

## Wavefront transport

Recursive path tracing is readable but exposes severe control-flow divergence on massively parallel hardware. GX stores path state in structure-of-arrays queues. Each bounce consumes one active queue and writes survivors to the next queue.

Advantages:

- terminated paths disappear immediately;
- compaction cost is observable;
- queues can be bucketed or radix-ordered;
- material/traversal work can be scheduled independently;
- CPU and GPU implementations can share scene and queue semantics.

The CPU wavefront renderer uses persistent workers. It is not expected to beat the packed HX loop on every CPU workload; the organization is primarily intended to expose GPU-friendly work units and controllable work reduction.

## Queue compaction and ordering

GX defines an exclusive prefix-sum/scatter contract. The OpenCL backend provides block scan, block-offset addition and alive-index scatter kernels. A CPU reference implementation is regression-tested against the same semantics.

Sorting is conditional rather than doctrinal. `evaluate_queue_ordering()`:

1. measures current key coherence;
2. performs a four-pass 32-bit radix ordering;
3. measures sort cost;
4. estimates divergence savings from improved coherence;
5. recommends the sorted order only when the estimated savings exceed measured cost.

This makes queue ordering a policy decision rather than an unconditional tax.

## Acceleration structures

### HX binary BVH

The primary packed scene uses HX's flattened SAH or Morton hierarchy. This remains a strong CPU baseline and the correctness reference for GX acceleration experiments.

### Quantized binary export

GX can export binary nodes with conservative 16-bit bounds relative to the root scene extent. This lowers device payload while preserving conservative containment.

### Quantized BVH8

`WideBvh8` collapses the binary hierarchy into up to eight child slots per node. Each 160-byte node uses structure-of-arrays quantized bounds:

- 8 child minima for X/Y/Z;
- 8 child maxima for X/Y/Z;
- child/ref indices;
- leaf counts;
- child kind tags.

Both CPU and OpenCL reference traversal exist. Randomized tests require closest-hit agreement with HX's binary BVH.

The current CPU benchmark demonstrates an important non-result: the wide representation is smaller but slower on that CPU workload. The experiment exists for GPU bandwidth/occupancy tradeoffs, where fewer node fetches and compact bounds may matter differently.

## Direct illumination

GX exposes three levels of direct-light experimentation.

### Alias MIS

A power-weighted emissive alias table proposes lights. Diffuse BSDF and light PDFs use the power heuristic, including complementary weighting for emissive hits reached through BSDF continuation.

### Local reservoir RIS

The wavefront renderer can evaluate several cheap light candidates and choose one through reservoir importance sampling before issuing a shadow ray.

### Spatiotemporal ReSTIR DI reference

`restir_di()` adds the roadmap's complete reference reuse path:

- per-pixel reservoirs;
- power-weighted fresh candidates;
- current-to-previous motion vectors;
- temporal reprojection;
- reservoir candidate re-evaluation at the current surface;
- normal/depth compatibility rejection;
- spatial neighbor reuse;
- visibility-aware validation;
- final visibility tests;
- explicit history update and reuse counters.

The implementation favors inspectability and deterministic testing. A production implementation would normally pack reservoirs more aggressively and use device-specific visibility/reprojection optimizations.

## Indirect reuse

Two independent structures are supplied so experiments can compare their value rather than conflating them.

### Radiance cache

`RadianceCache` hashes world space into configurable cells, accumulates radiance, tracks frame age and supports pruning. It is suitable for coarse indirect-history reuse experiments.

### Directional path guide

`PathGuideGrid` stores eight directional octant weights per world-space cell. Samples update directional contribution; subsequent path choices can sample from the learned distribution and evaluate its PDF.

Both are lightweight reference structures, not claims of a finished production ReSTIR-GI implementation.

## Virtualized geometry

### Paged storage

`PagedTriangleStore` serializes fixed-size triangle pages and supports independent page reads.

### Cluster BVH

`ClusterBvh` builds a top-level hierarchy over page bounds. A ray query returns candidate page IDs rather than forcing every page into resident memory.

### Residency manager

`PageResidencyManager` adds:

- asynchronous worker-thread loads;
- synchronized concurrent requests;
- host byte budget;
- device byte budget accounting;
- LRU-style stamps;
- upload callback;
- device eviction callback;
- blocking wait and nonblocking residency queries.

Callbacks execute outside the residence lock, preventing allocator/upload callbacks from re-entering the manager while its internal mutex is held.

The abstraction deliberately leaves actual Vulkan/CUDA/HIP copies to the embedding backend, where command queues, pinned memory and allocator strategy are application-specific.

## Reconstruction

### A-trous

The deterministic dependency-free fallback uses color, surface normal and relative depth to suppress cross-edge blur.

### Temporal accumulation

The original temporal accumulator provides bounded history with normal/depth rejection for stable views.

### Motion-vector TAA

`TemporalAA` reprojects previous history using current-to-previous pixel motion, rejects incompatible normal/depth history, clamps history against a current 3x3 neighborhood and tracks bounded per-pixel history length.

### Optional neural residual inference

`TinyNeuralDenoiser` provides a small 12-input, 16-hidden-unit, 3-output residual inference path. Weights are supplied by the application. No untrained/random model is advertised as a denoiser.

## Native GPU backends

### OpenCL reference

`gpu/beamcast_gx.cl` is the most complete portable kernel source. It contains:

- full-precision binary traversal;
- quantized binary traversal;
- quantized BVH8 traversal;
- path/material shading helpers;
- alive marking;
- block prefix scan;
- block offset addition;
- scatter compaction.

The source targets OpenCL 1.2 and is syntax-validated in the package test procedure.

### Vulkan

`gpu/vulkan/` contains compute shaders for queue scan/offset/scatter. CMake compiles them to SPIR-V automatically when `glslc` is available. Runtime discovery directly interrogates the Vulkan loader for physical devices and compute-capable queue families.

### CUDA and HIP

`gpu/cuda/` and `gpu/hip/` contain native queue-compaction specializations. CMake probes the vendor compilers and enables the corresponding object targets when available. Runtime discovery separately checks whether CUDA/HIP devices are exposed.

The development container exposed no usable physical GPU and lacked CUDA/HIP compilers, so vendor execution is not presented as validated there.

## Frame-budget control

`FrameBudgetController` closes the loop around observed frame time. A configurable PID-like feedback calculation maps timing error into a quality scale, then derives:

- samples per pixel;
- maximum path bounces;
- foveation strength;
- reconstruction history length.

This allows the renderer to degrade gracefully after expensive frames and reinvest spare frame time in quality after cheap ones.

## Price/performance methodology

`beamcast_price_perf` compares a cheaper render to a higher-sample reference and emits a CSV record containing:

- device/backend label;
- purchase-price input;
- frame milliseconds;
- energy joules when Linux RAPL is exposed;
- resident process bytes;
- MSE;
- PSNR;
- SSIM;
- FPS per dollar;
- quality per joule.

This is intentionally designed for cross-hardware reproduction. An optimization that wins only by producing a substantially worse image should appear as such in the quality metrics rather than winning because the benchmark forgot to look at its pixels.

## Correctness strategy

The most dangerous GX systems have a simpler reference beside them:

- BVH8 traversal is compared against HX binary traversal;
- GPU compaction semantics are compared against a CPU exclusive-scan reference;
- ReSTIR counters and deterministic seeds expose reuse behavior;
- virtual residency is stress-tested under concurrent request pressure;
- GPU structures have ABI/layout tests;
- reconstruction has identity/history tests;
- image comparisons use objective metrics.

The final package is built and tested under normal Release, warnings-as-errors, ASan, UBSan and a concurrency-focused ThreadSanitizer run, followed by a clean build from the packaged archive.
