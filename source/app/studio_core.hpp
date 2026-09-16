#pragma once

#include <atomic>
#include <cstdint>
#include <memory>
#include <mutex>
#include <string>
#include <thread>

#include "beamcast/beamcast.hpp"

namespace beamcast::studio {

enum class ScenePreset : std::uint8_t { Showcase, CornellLike, StressGrid };

enum class RenderPreset : std::uint8_t { Draft, Preview, Balanced, High, Ultra };

struct StudioSettings {
    int width{960};
    int height{540};
    RenderPreset preset{RenderPreset::Preview};
    bool denoise{true};
    bool variable_rate{false};
};

struct Snapshot {
    bool rendering{false};
    bool has_image{false};
    float progress{0.0f};
    std::size_t primitive_count{0};
    std::string scene_name{"Untitled"};
    std::string status{"Ready"};
    double seconds{0.0};
    double rays_per_second{0.0};
};

class StudioCore {
public:
    StudioCore();
    ~StudioCore();

    StudioCore(const StudioCore&) = delete;
    StudioCore& operator=(const StudioCore&) = delete;

    void load_preset(ScenePreset preset);
    void clear_scene();
    void import_obj(const std::string& path);

    void start_render();
    void cancel_render();
    void wait_for_render();

    void save_image(const std::string& path) const;

    Snapshot snapshot() const;
    StudioSettings settings() const;
    void set_settings(const StudioSettings& settings);

    easy::Camera camera() const;
    void set_camera(const easy::Camera& camera);
    void reset_camera();
    void orbit_camera(float yaw_degrees, float pitch_degrees);
    void dolly_camera(float amount);

    std::shared_ptr<const Framebuffer> image() const;

private:
    mutable std::mutex mutex_;
    easy::Scene scene_;
    easy::Camera camera_{};
    StudioSettings settings_{};
    std::string scene_name_{"Material Showcase"};
    std::string status_{"Ready"};
    std::shared_ptr<Framebuffer> image_;
    gx::Report report_{};

    std::thread render_thread_;
    std::atomic<bool> rendering_{false};
    std::atomic<bool> cancel_requested_{false};
    std::atomic<float> progress_{0.0f};

    void stop_before_edit();
    easy::Options make_options() const;
};

std::string preset_name(RenderPreset preset);
RenderPreset next_preset(RenderPreset preset);

} // namespace beamcast::studio
