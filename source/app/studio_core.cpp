#include "studio_core.hpp"

#include <algorithm>
#include <cmath>
#include <fstream>
#include <limits>
#include <sstream>
#include <stdexcept>
#include <vector>

namespace beamcast::studio {
namespace {

constexpr float kPi = 3.14159265358979323846f;

struct ObjIndex {
    int v{0};
};

int parse_vertex_index(const std::string& token, std::size_t vertex_count) {
    const auto slash = token.find('/');
    const std::string head = token.substr(0, slash);
    if (head.empty()) throw std::runtime_error("OBJ face contains an empty vertex index");
    const int raw = std::stoi(head);
    if (raw == 0) throw std::runtime_error("OBJ indices are 1-based; index 0 is invalid");
    const long long idx = raw > 0 ? static_cast<long long>(raw - 1)
                                  : static_cast<long long>(vertex_count) + raw;
    if (idx < 0 || idx >= static_cast<long long>(vertex_count))
        throw std::runtime_error("OBJ face references a vertex outside the file");
    return static_cast<int>(idx);
}

struct ObjLoadResult { Point3 center{}; float radius{1.0f}; };

ObjLoadResult load_obj_into_easy(const std::string& path, easy::Scene& scene) {
    std::ifstream in(path);
    if (!in) throw std::runtime_error("Could not open OBJ: " + path);

    std::vector<Point3> vertices;
    Point3 bmin{std::numeric_limits<float>::infinity(),std::numeric_limits<float>::infinity(),std::numeric_limits<float>::infinity()};
    Point3 bmax{-std::numeric_limits<float>::infinity(),-std::numeric_limits<float>::infinity(),-std::numeric_limits<float>::infinity()};
    std::string line;
    std::size_t line_number = 0;
    auto material = scene.diffuse({0.72f, 0.76f, 0.82f});

    while (std::getline(in, line)) {
        ++line_number;
        std::istringstream ss(line);
        std::string tag;
        ss >> tag;
        if (tag.empty() || tag[0] == '#') continue;
        if (tag == "v") {
            float x = 0, y = 0, z = 0;
            if (!(ss >> x >> y >> z))
                throw std::runtime_error("Malformed OBJ vertex at line " + std::to_string(line_number));
            vertices.push_back({x, y, z});
            bmin.x=std::min(bmin.x,x); bmin.y=std::min(bmin.y,y); bmin.z=std::min(bmin.z,z);
            bmax.x=std::max(bmax.x,x); bmax.y=std::max(bmax.y,y); bmax.z=std::max(bmax.z,z);
        } else if (tag == "f") {
            std::vector<int> face;
            std::string token;
            while (ss >> token) face.push_back(parse_vertex_index(token, vertices.size()));
            if (face.size() < 3)
                throw std::runtime_error("OBJ face has fewer than three vertices at line " + std::to_string(line_number));
            for (std::size_t i = 1; i + 1 < face.size(); ++i)
                scene.triangle(vertices[static_cast<std::size_t>(face[0])],
                               vertices[static_cast<std::size_t>(face[i])],
                               vertices[static_cast<std::size_t>(face[i + 1])], material);
        }
    }
    if (scene.primitive_count() == 0)
        throw std::runtime_error("OBJ contained no renderable faces");
    const Point3 center=(bmin+bmax)*0.5f;
    const float radius=std::max(0.5f,(bmax-bmin).length()*0.5f);
    return {center,radius};
}

} // namespace

std::string preset_name(RenderPreset preset) {
    switch (preset) {
        case RenderPreset::Draft: return "Draft";
        case RenderPreset::Preview: return "Preview";
        case RenderPreset::Balanced: return "Balanced";
        case RenderPreset::High: return "High";
        case RenderPreset::Ultra: return "Ultra";
    }
    return "Preview";
}

RenderPreset next_preset(RenderPreset preset) {
    const int n = (static_cast<int>(preset) + 1) % 5;
    return static_cast<RenderPreset>(n);
}

StudioCore::StudioCore() { load_preset(ScenePreset::Showcase); }

StudioCore::~StudioCore() {
    cancel_render();
    wait_for_render();
}

void StudioCore::stop_before_edit() {
    cancel_render();
    wait_for_render();
}

void StudioCore::load_preset(ScenePreset preset) {
    stop_before_edit();
    easy::Scene fresh;

    if (preset == ScenePreset::Showcase) {
        auto ground = fresh.diffuse({0.55f, 0.57f, 0.60f});
        auto red = fresh.diffuse({0.72f, 0.12f, 0.10f});
        auto gold = fresh.metal({0.93f, 0.72f, 0.28f}, 0.08f);
        auto glass = fresh.glass(1.5f);
        auto lamp = fresh.light({18.0f, 15.0f, 11.0f});
        fresh.sphere({0,-1001,-4},1000,ground)
             .sphere({-1.8f,0,-4.2f},1,red)
             .sphere({0.25f,0,-3.2f},1,glass)
             .sphere({2.2f,0,-4.4f},1,gold)
             .sphere({0,5,-4},0.8f,lamp);
        scene_name_ = "Material Showcase";
        camera_.position = {6, 3.2f, 5.5f}; camera_.target = {0,0,-4}; camera_.vertical_fov_degrees = 40;
    } else if (preset == ScenePreset::CornellLike) {
        auto white = fresh.diffuse({0.72f,0.72f,0.72f});
        auto red = fresh.diffuse({0.70f,0.08f,0.06f});
        auto green = fresh.diffuse({0.08f,0.55f,0.12f});
        auto glass = fresh.glass(1.5f);
        auto light = fresh.light({22,20,16});
        fresh.triangle({-3,-2,-8},{3,-2,-8},{3,-2,-2},white)
             .triangle({-3,-2,-8},{3,-2,-2},{-3,-2,-2},white)
             .triangle({-3,-2,-8},{-3,-2,-2},{-3,4,-2},red)
             .triangle({-3,-2,-8},{-3,4,-2},{-3,4,-8},red)
             .triangle({3,-2,-2},{3,-2,-8},{3,4,-8},green)
             .triangle({3,-2,-2},{3,4,-8},{3,4,-2},green)
             .sphere({-0.9f,-1,-5.2f},1,glass)
             .sphere({1.1f,-1.2f,-4.2f},0.8f,white)
             .sphere({0,3.2f,-5},0.65f,light);
        scene_name_ = "Cornell-like Room";
        camera_.position = {0,0.5f,5}; camera_.target = {0,0,-5}; camera_.vertical_fov_degrees = 38;
    } else {
        auto ground = fresh.diffuse({0.45f,0.48f,0.52f});
        auto blue = fresh.metal({0.25f,0.52f,0.92f},0.12f);
        auto light = fresh.light({25,22,18});
        fresh.sphere({0,-1001,-10},1000,ground);
        for (int z=0; z<20; ++z) for (int x=-20; x<=20; ++x) {
            const float fx = static_cast<float>(x) * 0.58f;
            const float fz = -3.0f - static_cast<float>(z) * 0.58f;
            fresh.sphere({fx,-0.65f,fz},0.22f,blue);
        }
        fresh.sphere({0,8,-8},1.2f,light);
        scene_name_ = "Stress Grid";
        camera_.position = {9,5,8}; camera_.target = {0,-0.5f,-8}; camera_.vertical_fov_degrees = 42;
    }

    fresh.build();
    std::lock_guard<std::mutex> lock(mutex_);
    scene_ = std::move(fresh);
    image_.reset();
    report_ = {};
    progress_.store(0.0f);
    status_ = "Scene ready";
}

void StudioCore::clear_scene() {
    stop_before_edit();
    easy::Scene fresh;
    fresh.build();
    std::lock_guard<std::mutex> lock(mutex_);
    scene_ = std::move(fresh);
    scene_name_ = "Empty Scene";
    image_.reset();
    report_ = {};
    status_ = "Scene cleared";
}

void StudioCore::import_obj(const std::string& path) {
    stop_before_edit();
    easy::Scene fresh;
    const ObjLoadResult loaded=load_obj_into_easy(path, fresh);
    fresh.build();
    std::lock_guard<std::mutex> lock(mutex_);
    scene_ = std::move(fresh);
    scene_name_ = path;
    image_.reset();
    report_ = {};
    status_ = "OBJ loaded";
    camera_.target = loaded.center;
    camera_.position = loaded.center + Vec3{loaded.radius*1.4f,loaded.radius*0.9f,loaded.radius*2.6f};
    camera_.vertical_fov_degrees = 42; camera_.focus_distance = loaded.radius*3.0f;
}

easy::Options StudioCore::make_options() const {
    StudioSettings s;
    { std::lock_guard<std::mutex> lock(mutex_); s = settings_; }
    easy::Options o;
    switch (s.preset) {
        case RenderPreset::Draft: o = easy::Options::draft(s.width,s.height); break;
        case RenderPreset::Preview: o = easy::Options::preview(s.width,s.height); break;
        case RenderPreset::Balanced: o = easy::Options::balanced(s.width,s.height); break;
        case RenderPreset::High: o = easy::Options::high(s.width,s.height); break;
        case RenderPreset::Ultra: o = easy::Options::ultra(s.width,s.height); break;
    }
    o.denoise = s.denoise;
    o.variable_rate_sampling = s.variable_rate;
    return o;
}

void StudioCore::start_render() {
    if (rendering_.load()) return;
    wait_for_render();
    cancel_requested_.store(false);
    progress_.store(0.0f);
    rendering_.store(true);
    {
        std::lock_guard<std::mutex> lock(mutex_);
        status_ = "Rendering...";
    }
    render_thread_ = std::thread([this] {
        try {
            easy::Camera cam;
            easy::Options opts = make_options();
            {
                std::lock_guard<std::mutex> lock(mutex_);
                cam = camera_;
            }
            auto result = easy::render(scene_, cam, opts, [this](float p) {
                if (cancel_requested_.load()) throw std::runtime_error("Render cancelled");
                progress_.store(std::clamp(p,0.0f,1.0f));
            });
            auto image = std::make_shared<Framebuffer>(result.image());
            {
                std::lock_guard<std::mutex> lock(mutex_);
                image_ = std::move(image);
                report_ = result.report();
                status_ = "Render complete";
            }
            progress_.store(1.0f);
        } catch (const std::exception& e) {
            std::lock_guard<std::mutex> lock(mutex_);
            status_ = cancel_requested_.load() ? "Render cancelled" : std::string("Render failed: ") + e.what();
        }
        rendering_.store(false);
    });
}

void StudioCore::cancel_render() { cancel_requested_.store(true); }

void StudioCore::wait_for_render() {
    if (render_thread_.joinable()) render_thread_.join();
}

void StudioCore::save_image(const std::string& path) const {
    std::shared_ptr<Framebuffer> image;
    {
        std::lock_guard<std::mutex> lock(mutex_);
        image = image_;
    }
    if (!image) throw std::runtime_error("There is no rendered image to save");
    save_ppm(path, *image, true);
}

Snapshot StudioCore::snapshot() const {
    Snapshot s;
    s.rendering = rendering_.load();
    s.progress = progress_.load();
    std::lock_guard<std::mutex> lock(mutex_);
    s.has_image = static_cast<bool>(image_);
    s.primitive_count = scene_.primitive_count();
    s.scene_name = scene_name_;
    s.status = status_;
    s.seconds = report_.total_seconds;
    s.rays_per_second = report_.rays_per_second;
    return s;
}

StudioSettings StudioCore::settings() const { std::lock_guard<std::mutex> lock(mutex_); return settings_; }

void StudioCore::set_settings(const StudioSettings& settings) {
    if (settings.width <= 0 || settings.height <= 0 || settings.width > 8192 || settings.height > 8192)
        throw std::invalid_argument("Studio resolution must be between 1 and 8192 pixels per axis");
    std::lock_guard<std::mutex> lock(mutex_);
    settings_ = settings;
}

easy::Camera StudioCore::camera() const { std::lock_guard<std::mutex> lock(mutex_); return camera_; }
void StudioCore::set_camera(const easy::Camera& camera) { std::lock_guard<std::mutex> lock(mutex_); camera_ = camera; }

void StudioCore::reset_camera() {
    std::lock_guard<std::mutex> lock(mutex_);
    camera_ = easy::Camera{};
}

void StudioCore::orbit_camera(float yaw_degrees, float pitch_degrees) {
    std::lock_guard<std::mutex> lock(mutex_);
    Vec3 offset = camera_.position - camera_.target;
    float r = std::max(0.1f, offset.length());
    float yaw = std::atan2(offset.x, offset.z) + yaw_degrees * kPi / 180.0f;
    float pitch = std::asin(std::clamp(offset.y / r, -0.999f, 0.999f)) + pitch_degrees * kPi / 180.0f;
    pitch = std::clamp(pitch, -1.45f, 1.45f);
    camera_.position = camera_.target + Vec3{r*std::cos(pitch)*std::sin(yaw), r*std::sin(pitch), r*std::cos(pitch)*std::cos(yaw)};
}

void StudioCore::dolly_camera(float amount) {
    std::lock_guard<std::mutex> lock(mutex_);
    Vec3 delta = camera_.position - camera_.target;
    float d = std::max(0.1f, delta.length());
    float nd = std::max(0.2f, d + amount);
    camera_.position = camera_.target + unit_vector(delta) * nd;
}

std::shared_ptr<const Framebuffer> StudioCore::image() const {
    std::lock_guard<std::mutex> lock(mutex_);
    return image_;
}

} // namespace beamcast::studio
