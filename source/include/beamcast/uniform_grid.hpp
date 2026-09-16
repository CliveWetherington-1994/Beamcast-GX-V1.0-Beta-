#pragma once
#include <cstdint>
#include <memory>
#include <vector>
#include "hittable.hpp"

namespace beamcast {

class UniformGrid final : public Hittable {
    std::vector<std::shared_ptr<Hittable>> objects_;
    std::vector<std::vector<std::uint32_t>> cells_;
    AABB bounds_;
    int nx_{0}, ny_{0}, nz_{0};
    Vec3 cell_size_{};
    std::size_t flatten(int x, int y, int z) const {
        return static_cast<std::size_t>(x) + static_cast<std::size_t>(nx_) *
               (static_cast<std::size_t>(y) + static_cast<std::size_t>(ny_) * static_cast<std::size_t>(z));
    }
public:
    UniformGrid() = default;
    explicit UniformGrid(std::vector<std::shared_ptr<Hittable>> objects, int max_axis_cells=64);
    bool hit(const Ray&, Scalar t_min, Scalar t_max, HitRecord&) const override;
    AABB bounding_box() const override { return bounds_; }
    std::size_t cell_count() const { return cells_.size(); }
};

} // namespace beamcast
