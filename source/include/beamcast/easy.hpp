#pragma once

#include <cstdint>
#include <functional>
#include <string>

#include "gx.hpp"

namespace beamcast::easy {

enum class Quality : std::uint8_t { Draft, Preview, Balanced, High, Ultra };

struct Material {
    std::uint32_t id{0};
};

class Scene {
    hx::PackedScene scene_;
    bool dirty_{false};
    hx::PackedScene::BuildOptions build_{};
public:
    Scene();

    Material diffuse(Color color);
    Material metal(Color color, Scalar roughness=0.0f);
    Material glass(Scalar ior=1.5f);
    Material light(Color emission);

    Scene& sphere(Point3 center, Scalar radius, Material material);
    Scene& triangle(Point3 a, Point3 b, Point3 c, Material material);
    Scene& clear();

    Scene& acceleration(hx::BuildMode mode, std::uint32_t leaf_size=4, std::uint32_t sah_bins=24);
    Scene& build();
    const hx::PackedScene& packed();
    std::size_t primitive_count() const { return scene_.primitive_count(); }
};

struct Camera {
    Point3 position{5,3,5};
    Point3 target{0,0,-3};
    Vec3 up{0,1,0};
    Scalar vertical_fov_degrees{40};
    Scalar aperture{0};
    // <= 0 means "use distance(position,target)".
    Scalar focus_distance{0};

    beamcast::Camera make(Scalar aspect_ratio) const;
};

struct Options {
    int width{1280};
    int height{720};
    Quality quality{Quality::Balanced};
    int thread_count{0};
    bool denoise{true};
    bool variable_rate_sampling{false};
    Scalar foveation_strength{0.55f};
    gx::DirectLightingMode direct_lighting{gx::DirectLightingMode::ReservoirRIS};
    std::uint64_t seed{0x4245414d43415354ULL};

    static Options draft(int width=960, int height=540);
    static Options preview(int width=1280, int height=720);
    static Options balanced(int width=1280, int height=720);
    static Options high(int width=1920, int height=1080);
    static Options ultra(int width=1920, int height=1080);
};

struct Result {
    gx::Result frame;
    const Framebuffer& image() const { return frame.image; }
    const gx::Report& report() const { return frame.report; }
    void save_ppm(const std::string& path, bool binary=true) const;
};

using ProgressCallback = std::function<void(float progress_0_to_1)>;

Result render(Scene& scene,
              const Camera& camera=Camera{},
              const Options& options=Options{},
              ProgressCallback progress={});

} // namespace beamcast::easy
