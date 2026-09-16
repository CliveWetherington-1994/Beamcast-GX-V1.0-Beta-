#pragma once
#include <memory>
#include <string>
#include <vector>
#include "bvh.hpp"
#include "triangle.hpp"

namespace beamcast {

class Mesh final : public Hittable {
    std::vector<std::shared_ptr<Hittable>> triangles_;
    std::shared_ptr<BVH> bvh_;
    AABB box_;
public:
    Mesh() = default;
    static std::shared_ptr<Mesh> load_obj(const std::string& path, std::shared_ptr<Material> material, bool build_bvh=true);
    std::size_t triangle_count() const { return triangles_.size(); }
    bool hit(const Ray&, Scalar t_min, Scalar t_max, HitRecord&) const override;
    AABB bounding_box() const override { return box_; }
};

} // namespace beamcast
