#pragma once

#include <cstddef>
#include <cstdint>
#include <functional>
#include <vector>

#include "camera.hpp"
#include "framebuffer.hpp"
#include "hx.hpp"

namespace beamcast::gx {

enum class DirectLightingMode : std::uint8_t {
    Off,
    AliasMIS,
    ReservoirRIS
};

struct Settings {
    int width{1280};
    int height{720};
    int samples_per_pixel{16};
    int sample_batch{1};
    int max_bounces{12};
    int thread_count{0};
    int work_chunk{1024};
    std::uint64_t seed{0x47584245414dULL};

    DirectLightingMode direct_lighting{DirectLightingMode::ReservoirRIS};
    int light_candidates{4};
    bool russian_roulette{true};
    int russian_roulette_start{4};

    bool variable_rate_sampling{false};
    Scalar foveation_center_x{0.5f};
    Scalar foveation_center_y{0.5f};
    Scalar foveation_strength{0.75f};
    int minimum_pixel_samples{1};

    bool direction_bucketing{true};

    bool denoise{true};
    int denoise_iterations{4};
    Scalar denoise_color_sigma{4.0f};
    Scalar denoise_normal_sigma{64.0f};
    Scalar denoise_depth_sigma{1.0f};

    Color background_bottom{0.015f, 0.02f, 0.035f};
    Color background_top{0.16f, 0.22f, 0.36f};
};

struct Report {
    std::uint64_t primary_paths{0};
    std::uint64_t path_rays{0};
    std::uint64_t shadow_rays{0};
    std::uint64_t box_tests{0};
    std::uint64_t primitive_tests{0};
    std::uint64_t hits{0};
    std::uint64_t light_candidates{0};
    std::uint64_t queue_pushes{0};
    std::uint64_t queue_compactions{0};
    std::size_t peak_active_paths{0};
    std::size_t gpu_export_bytes{0};
    std::size_t gpu_quantized_export_bytes{0};
    std::uint64_t pixels_sampled{0};
    std::uint64_t pixels_skipped_by_vrs{0};
    std::uint64_t direction_bucket_passes{0};
    double render_seconds{0};
    double denoise_seconds{0};
    double total_seconds{0};
    double rays_per_second{0};
};

struct Guides {
    int width{0};
    int height{0};
    std::vector<Vec3> normal;
    std::vector<Scalar> depth;
    std::vector<Color> albedo;
    std::vector<Point3> position;
};

struct Result {
    Framebuffer noisy;
    Framebuffer image;
    Guides guides;
    Report report;
};

using ProgressCallback = std::function<void(int completed_sample_batches, int total_sample_batches)>;

// GPU-friendly wavefront renderer implemented on the CPU. The queue/state layout mirrors
// what a CUDA/HIP/OpenCL/Vulkan compute backend would upload and compact per bounce.
Result render_wavefront(const hx::PackedScene& scene,
                        const Camera& camera,
                        const Settings& settings,
                        ProgressCallback progress = {});

Framebuffer atrous_denoise(const Framebuffer& input,
                            const Guides& guides,
                            int iterations,
                            Scalar color_sigma,
                            Scalar normal_sigma,
                            Scalar depth_sigma);

// Conservative byte count for the flattened scene payload returned by PackedScene::export_gpu().
std::size_t exported_scene_bytes(const hx::PackedScene& scene);
std::size_t exported_quantized_scene_bytes(const hx::PackedScene& scene);

} // namespace beamcast::gx
