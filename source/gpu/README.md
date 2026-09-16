# Beamcast GX native GPU backends

GX keeps one queue ABI across backends: paths are marked alive, prefix-scanned, scattered into a compact queue, intersected, shaded, and repeated per bounce.

## OpenCL reference

`beamcast_gx.cl` is the portable OpenCL 1.2 reference. It contains binary and quantized BVH traversal, wavefront shading, alive marking, a 256-lane block scan, block-offset addition, and scatter. The scan is hierarchical: recursively scan `block_sums`, then call `gx_add_block_offsets` and `gx_scatter_alive`.

## Vulkan compute

`vulkan/gx_compact.comp`, `gx_add_offsets.comp`, and `gx_scatter.comp` provide the same device-compaction contract in GLSL 450. If `glslc` is installed, CMake builds SPIR-V automatically. Production integration should record these passes into the same command buffer as traversal/shading so queues remain device resident.

## CUDA

`cuda/beamcast_gx.cu` exposes queue marking/scatter kernels. A CUDA runtime should use CUB `DeviceScan::ExclusiveSum` between them; this avoids maintaining a slower home-grown scan on NVIDIA hardware.

## HIP

`hip/beamcast_gx_hip.cpp` mirrors the CUDA ABI. A HIP runtime should use rocPRIM `exclusive_scan` between marking and scatter.

The CMake build probes CUDA/HIP toolchains and `glslc` without making them mandatory. Missing vendor SDKs never disable the CPU/OpenCL-reference research path.
