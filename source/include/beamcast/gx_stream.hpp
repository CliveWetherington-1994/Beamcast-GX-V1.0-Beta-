#pragma once
#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>
#include "gx.hpp"

namespace beamcast::gx {

struct MeshPageInfo {
    std::uint64_t file_offset{0};
    std::uint32_t triangle_count{0};
};

class PagedTriangleStore {
    std::string path_;
    std::vector<MeshPageInfo> pages_;
public:
    static void write(const std::string& path,
                      const std::vector<hx::PackedTriangle>& triangles,
                      std::uint32_t triangles_per_page = 65536);
    explicit PagedTriangleStore(std::string path);
    std::size_t page_count() const { return pages_.size(); }
    const MeshPageInfo& page_info(std::size_t page) const { return pages_.at(page); }
    std::vector<hx::PackedTriangle> load_page(std::size_t page) const;
    void append_page_to_scene(std::size_t page, hx::PackedScene& scene) const;
};

struct HybridSplit {
    int cpu_tiles{0};
    int gpu_tiles{0};
};

// Proportional work split from measured device throughput. This is intentionally backend-agnostic:
// CUDA/HIP/OpenCL/Vulkan implementations can all feed their observed Mray/s into the same planner.
HybridSplit plan_hybrid_tiles(int total_tiles, double cpu_mrays_per_second, double gpu_mrays_per_second);

class TemporalAccumulator {
    Framebuffer history_;
    Guides guides_;
    std::vector<std::uint16_t> history_length_;
public:
    void reset();
    bool valid() const { return !history_.pixels.empty(); }
    Framebuffer accumulate(const Framebuffer& current,
                           const Guides& guides,
                           int max_history = 32,
                           Scalar normal_threshold = 0.92f,
                           Scalar relative_depth_threshold = 0.03f);
};

} // namespace beamcast::gx
