#pragma once

#include <array>
#include <atomic>
#include <cstddef>
#include <cstdint>
#include <functional>
#include <limits>
#include <vector>
#include "aabb.hpp"
#include "camera.hpp"
#include "framebuffer.hpp"
#include "random.hpp"
#include "ray.hpp"

namespace beamcast::hx {

// High-throughput, data-oriented path for large CPU renders.
enum class MaterialType : std::uint8_t { Lambertian, Metal, Dielectric, Emissive };
enum class BuildMode : std::uint8_t { SAH, Morton };

struct PackedMaterial {
    MaterialType type{MaterialType::Lambertian};
    Color albedo{0.8f,0.8f,0.8f};
    Color emission{0,0,0};
    Scalar roughness{0};
    Scalar ior{1.5f};
};

struct PackedSphere {
    Point3 center{};
    Scalar radius{1};
    std::uint32_t material{0};
};

struct PackedTriangle {
    Point3 v0{};
    Vec3 e1{};
    Vec3 e2{};
    Vec3 normal{};
    std::uint32_t material{0};
};

struct PackedHit {
    Scalar t{std::numeric_limits<Scalar>::infinity()};
    Point3 p{};
    Vec3 normal{};
    std::uint32_t material{0};
    std::uint32_t primitive_index{0};
    std::uint32_t primitive_type{0}; // 0 sphere, 1 triangle
    bool front_face{true};
};


struct GpuBvhNode {
    Vec3 bmin{};
    Vec3 bmax{};
    std::uint32_t left{0};
    std::uint32_t right{0};
    std::uint32_t first{0};
    std::uint32_t count{0};
    std::uint32_t leaf{0};
    std::uint32_t axis{0};
};

struct GpuPrimitiveRef {
    std::uint32_t index{0};
    std::uint32_t type{0}; // 0 sphere, 1 triangle
};

struct GpuSceneData {
    std::vector<GpuBvhNode> nodes;
    std::vector<GpuPrimitiveRef> refs;
    std::vector<PackedSphere> spheres;
    std::vector<PackedTriangle> triangles;
    std::vector<PackedMaterial> materials;
};

struct QuantizedGpuBvhNode {
    std::uint16_t bmin[3]{};
    std::uint16_t bmax[3]{};
    std::uint32_t left{0};
    std::uint32_t right{0};
    std::uint32_t first{0};
    std::uint32_t count{0};
    std::uint32_t leaf{0};
    std::uint32_t axis{0};
};

struct QuantizedGpuSceneData {
    Point3 world_min{};
    Vec3 world_extent{1,1,1};
    std::vector<QuantizedGpuBvhNode> nodes;
    std::vector<GpuPrimitiveRef> refs;
    std::vector<PackedSphere> spheres;
    std::vector<PackedTriangle> triangles;
    std::vector<PackedMaterial> materials;
};

struct TraceStats {
    std::uint64_t rays{0};
    std::uint64_t box_tests{0};
    std::uint64_t primitive_tests{0};
    std::uint64_t hits{0};
};

class PackedScene {
public:
    struct BuildOptions {
        BuildMode mode{BuildMode::SAH};
        std::uint32_t leaf_size{4};
        std::uint32_t sah_bins{16};
    };

private:
    enum class PrimitiveType : std::uint8_t { Sphere, Triangle };
    struct PrimitiveRef {
        AABB box{};
        Point3 centroid{};
        std::uint32_t index{0};
        PrimitiveType type{PrimitiveType::Sphere};
        std::uint32_t morton{0};
    };
    struct alignas(64) Node {
        AABB box{};
        std::uint32_t left{0};
        std::uint32_t right{0};
        std::uint32_t first{0};
        std::uint32_t count{0};
        std::uint32_t axis{0};
        std::uint32_t leaf{0};
    };

    std::vector<PackedMaterial> materials_;
    std::vector<PackedSphere> spheres_;
    std::vector<PackedTriangle> triangles_;
    std::vector<PrimitiveRef> refs_;
    std::vector<Node> nodes_;
    BuildOptions build_options_{};

    std::uint32_t build_sah(std::uint32_t begin, std::uint32_t end);
    std::uint32_t build_morton(std::uint32_t begin, std::uint32_t end);
    bool hit_primitive(const PrimitiveRef&, const Ray&, Scalar t_min, Scalar t_max, PackedHit&, TraceStats*) const;

public:
    std::uint32_t add_material(const PackedMaterial& m);
    std::uint32_t add_lambertian(Color albedo);
    std::uint32_t add_metal(Color albedo, Scalar roughness=0);
    std::uint32_t add_dielectric(Scalar ior=1.5f);
    std::uint32_t add_emissive(Color emission);
    void add_sphere(Point3 center, Scalar radius, std::uint32_t material);
    void add_triangle(Point3 a, Point3 b, Point3 c, std::uint32_t material);
    void clear();
    void build();
    void build(BuildOptions options);

    bool hit(const Ray&, Scalar t_min, Scalar t_max, PackedHit&, TraceStats* stats=nullptr) const;
    const PackedMaterial& material(std::uint32_t i) const { return materials_[i]; }
    const std::vector<PackedMaterial>& materials() const { return materials_; }
    const std::vector<PackedSphere>& spheres() const { return spheres_; }
    const std::vector<PackedTriangle>& triangles() const { return triangles_; }
    GpuSceneData export_gpu() const;
    QuantizedGpuSceneData export_gpu_quantized() const;
    std::size_t gpu_export_bytes() const noexcept {
        return nodes_.size()*sizeof(GpuBvhNode) + refs_.size()*sizeof(GpuPrimitiveRef) +
               spheres_.size()*sizeof(PackedSphere) + triangles_.size()*sizeof(PackedTriangle) +
               materials_.size()*sizeof(PackedMaterial);
    }
    std::size_t gpu_quantized_export_bytes() const noexcept {
        return nodes_.size()*sizeof(QuantizedGpuBvhNode) + refs_.size()*sizeof(GpuPrimitiveRef) +
               spheres_.size()*sizeof(PackedSphere) + triangles_.size()*sizeof(PackedTriangle) +
               materials_.size()*sizeof(PackedMaterial) + sizeof(Point3) + sizeof(Vec3);
    }
    std::size_t primitive_count() const { return spheres_.size() + triangles_.size(); }
    bool built() const noexcept { return primitive_count()==0 || !nodes_.empty(); }
    std::size_t node_count() const { return nodes_.size(); }
    AABB bounds() const { return nodes_.empty() ? AABB{} : nodes_.front().box; }
};

struct RenderSettings {
    int width{1280};
    int height{720};
    int min_samples{8};
    int max_samples{128};
    int max_bounces{12};
    int thread_count{0};
    int tile_size{16};
    std::uint64_t seed{0xB34AC457A55AULL};
    bool adaptive_sampling{true};
    Scalar adaptive_threshold{0.015f};
    bool russian_roulette{true};
    Color background_bottom{0.02f,0.025f,0.04f};
    Color background_top{0.18f,0.24f,0.38f};
};

struct RenderReport {
    std::uint64_t camera_samples{0};
    std::uint64_t path_rays{0};
    std::uint64_t box_tests{0};
    std::uint64_t primitive_tests{0};
    std::uint64_t hits{0};
    double seconds{0};
    double primary_samples_per_second{0};
    double rays_per_second{0};
};

using ProgressCallback = std::function<void(int completed_tiles, int total_tiles)>;

Color trace(Ray ray, const PackedScene& scene, const RenderSettings&, RNG&, TraceStats* stats=nullptr);
Framebuffer render(const PackedScene&, const Camera&, const RenderSettings&, RenderReport* report=nullptr, ProgressCallback progress={});

} // namespace beamcast::hx
