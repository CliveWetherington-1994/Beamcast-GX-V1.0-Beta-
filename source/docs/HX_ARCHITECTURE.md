# Beamcast HX Architecture

## Fast path

`beamcast::hx::PackedScene` stores materials and primitive data in contiguous vectors. Traversal never calls virtual functions and never touches `shared_ptr`.

Each primitive receives a compact `PrimitiveRef` containing bounds, centroid, type and index. BVH leaves reference contiguous ranges of these records.

## SAH builder

The SAH builder uses binned centroid partitioning. Each axis is divided into configurable bins. Prefix/suffix bounds and counts are evaluated to choose a split that approximately minimizes traversal/intersection work. If the split degenerates, median partitioning is used as a fallback.

## Morton builder

The Morton builder normalizes primitive centroids into a 10-bit-per-axis grid, interleaves the bits into 30-bit Morton codes, sorts by code, then recursively partitions ranges by the highest differing bit. This is much cheaper to build than a full SAH hierarchy and is useful for rapidly rebuilt scenes.

## Traversal

Traversal is iterative. A 128-entry local stack avoids heap allocation. The ray direction reciprocal is computed once per scene query. Child boxes are tested before insertion, and the nearer child is traversed first so early primitive hits shrink `t_max` quickly.

## Renderer

The renderer uses fixed-size tiles distributed through an atomic index. Each worker keeps local traversal statistics and merges them at the end, avoiding atomic increments for every ray or box test.

Adaptive sampling uses online mean/variance estimation of pixel luminance. Once the estimated standard error drops below the configured relative threshold after `min_samples`, the pixel stops early rather than mechanically spending `max_samples` everywhere.

## Performance switches

`BEAMCAST_HX_ENABLE_AVX2=ON` enables AVX2/FMA compiler targeting. `BEAMCAST_HX_NATIVE=ON` enables host-specific instruction scheduling on GCC/Clang. For binaries that must run on unknown CPUs, leave both off.
