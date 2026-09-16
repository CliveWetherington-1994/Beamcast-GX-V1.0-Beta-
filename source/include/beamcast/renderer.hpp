#pragma once
#include <cstdint>
#include <functional>
#include <iosfwd>
#include <string>
#include "camera.hpp"
#include "framebuffer.hpp"
#include "hittable.hpp"

namespace beamcast {

struct RenderSettings {
    int width{640};
    int height{360};
    int samples_per_pixel{16};
    int max_bounces{10};
    int thread_count{0};
    int tile_size{16};
    std::uint64_t seed{0xB34AC457ULL};
    Color background_bottom{1,1,1};
    Color background_top{0.5f,0.7f,1.0f};
    bool russian_roulette{true};
};

using ProgressCallback = std::function<void(int completed_tiles, int total_tiles)>;

Color trace_ray(Ray ray, const Hittable& scene, const RenderSettings& settings, RNG& rng);
Framebuffer render(const Hittable& scene, const Camera& camera, const RenderSettings& settings, ProgressCallback progress={});
void write_ppm(std::ostream& out, const Framebuffer& framebuffer);
void write_ppm_binary(std::ostream& out, const Framebuffer& framebuffer);
void save_ppm(const std::string& path, const Framebuffer& framebuffer, bool binary=true);
void render_ppm(std::ostream& out, const Hittable& scene, const Camera& camera, const RenderSettings& settings);

} // namespace beamcast
