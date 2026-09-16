# Beamcast GX 4.1 research roadmap status

GX 4.1 includes every experiment that was listed in the GX 3.0 research roadmap as an inspectable C++17 reference subsystem, portable kernel path, or optional native specialization. This document records what exists and, just as importantly, what was actually validated on the build machine.

A green status here means the research item has a concrete implementation and regression coverage. It does **not** mean Beamcast has magically become a production replacement for OptiX, Embree, Unreal, or a film renderer. Humans have invented benchmarking precisely because adjectives eventually run out.

## 1. Native GPU runtime and backend specializations — IMPLEMENTED / HARDWARE-DEPENDENT VALIDATION

Implemented:

- runtime discovery for CPU, OpenCL, Vulkan, CUDA, and HIP;
- dynamic device/compute-queue interrogation without requiring vendor SDK headers for discovery;
- OpenCL 1.2 portable transport, BVH traversal, BVH8 traversal, shading, scan, and scatter kernels;
- Vulkan compute queue-compaction GLSL sources with automatic SPIR-V compilation when `glslc` is available;
- CUDA queue-compaction specialization compiled automatically when `nvcc` is available;
- HIP queue-compaction specialization compiled automatically when `hipcc` is available;
- CMake feature detection and backend capability reporting.

Validation on the development container:

- OpenCL and Vulkan loader libraries were present;
- no usable OpenCL platform or Vulkan physical GPU was exposed to the container;
- CUDA and HIP runtimes/toolchains were absent;
- the OpenCL 1.2 source passed Clang syntax validation;
- vendor-specific kernels therefore could not be executed on physical GPU hardware in this environment.

## 2. Device queue compaction and conditional sorting — IMPLEMENTED

Implemented:

- exclusive prefix-sum contract;
- block scan and block-offset addition kernels;
- alive-path scatter kernel;
- CPU reference implementation used for exact regression comparison;
- four-pass 32-bit radix ordering;
- coherence scoring;
- measured sort-cost versus estimated divergence-savings policy, so sorting is skipped when it is expected to cost more than it saves.

## 3. Wide compressed BVH — IMPLEMENTED

Implemented:

- quantized BVH8 representation;
- eight-child structure-of-arrays node layout;
- conservative 16-bit child bounds;
- 160-byte aligned node ABI;
- CPU reference traversal;
- OpenCL BVH8 traversal;
- build time, byte size, child occupancy, traversal work, and hit-equivalence metrics.

Current benchmark caveat: BVH8 is smaller but slower than the binary BVH on the tested CPU workload. It is intentionally retained as a GPU bandwidth/occupancy experiment rather than being marketed as a universal speedup.

## 4. Full spatiotemporal ReSTIR DI reference — IMPLEMENTED

Implemented:

- per-pixel reservoirs;
- power-weighted emissive candidates;
- camera-derived motion vectors;
- previous-frame reservoir reprojection;
- current-surface candidate re-evaluation;
- normal/depth temporal rejection;
- spatial neighbor reuse;
- visibility-aware validation;
- final visibility rays;
- overflow-resistant reservoir accounting;
- explicit history update API and telemetry for fresh, temporal, spatial, and visibility work.

This is a research/reference implementation intended for correctness and experimentation. Production engines normally add more aggressive visibility reuse, disocclusion handling, material IDs, and GPU-specific reservoir packing.

## 5. Indirect reuse — IMPLEMENTED

Two complementary experiments are included:

- a frame-aged hashed world-space radiance cache;
- an eight-octant directional path-guiding grid with update, sampling, and PDF evaluation.

The price/performance harness provides equal-reference MSE, PSNR, and SSIM so future indirect-reuse experiments can be compared at equal time or equal quality rather than by cherry-picked sample counts.

## 6. Virtualized geometry — IMPLEMENTED

Implemented:

- `PagedTriangleStore` fixed-size geometry pages;
- top-level `ClusterBvh` over page bounds;
- ray-to-page queries;
- asynchronous page loading on a worker thread;
- bounded host-memory residency;
- bounded device-memory residency accounting;
- upload and device-eviction callbacks;
- LRU-style residency stamps;
- concurrent request handling;
- stress tests for eviction and multi-threaded access.

The manager provides the host/device residency contract. Real device transfers are supplied through callbacks so Vulkan/CUDA/HIP integrations can use their own upload queues and allocators.

## 7. Reconstruction — IMPLEMENTED

Implemented:

- deterministic normal/depth-guided a-trous fallback;
- temporal accumulation;
- motion-vector TAA;
- normal/depth history rejection;
- neighborhood history clamping;
- bounded history length;
- optional tiny neural residual denoiser inference path with externally supplied weights.

No pretrained neural weights are bundled. An untrained model would be computationally expensive nonsense, which software repositories already contain in sufficient quantities.

## 8. Closed-loop frame-budget controller — IMPLEMENTED

Implemented:

- configurable target frame time, including 8.3/16.7/33.3 ms-style targets;
- proportional/integral/derivative-style feedback;
- dynamic sample count;
- dynamic maximum bounce depth;
- dynamic foveation strength;
- dynamic reconstruction history;
- direct application to GX render settings;
- regression tests showing quality falls after over-budget frames and recovers after under-budget frames.

## 9. Reproducible price/performance suite — IMPLEMENTED

Implemented metrics:

- wall-clock frame time;
- process resident memory;
- optional Linux RAPL energy measurements when exposed by the host;
- MSE;
- PSNR;
- SSIM;
- FPS per purchase dollar;
- quality per joule;
- CSV output for cross-machine runs;
- high-sample reference versus cheaper render example.

The repository includes a sample result from the development container. RAPL was unavailable there, so energy is reported as unavailable/zero rather than invented. The harness is designed to be rerun on integrated GPUs, used discrete GPUs, and modern cards with the same reference scene and quality metrics.

## Validation status

The release, warning-clean, sanitizer, randomized-stress, and clean-archive results are recorded in `benchmarks/VALIDATION.txt` in the packaged source tree.

A credible claim that Beamcast changes rendering economics still requires independent, reproducible hardware measurements against serious baselines. GX 4.1 contains the machinery needed to run those experiments instead of merely listing them as future work.
