#pragma once
#include <cstdint>
#include <memory>
#include <vector>
#include "hittable.hpp"

namespace beamcast {

class BVH final : public Hittable {
public:
    struct BuildOptions {
        std::uint32_t leaf_size{4};
        std::uint32_t sah_bins{12};
    };
private:
    struct Node {
        AABB box;
        std::uint32_t left{0}, right{0}, start{0}, count{0};
        bool leaf{false};
    };
    std::vector<std::shared_ptr<Hittable>> objects_;
    std::vector<Node> nodes_;
    BuildOptions options_;
    std::uint32_t build_node(std::uint32_t start, std::uint32_t end);
public:
    BVH() = default;
    explicit BVH(std::vector<std::shared_ptr<Hittable>> objects);
    BVH(std::vector<std::shared_ptr<Hittable>> objects, BuildOptions options);
    bool empty() const { return objects_.empty(); }
    std::size_t node_count() const { return nodes_.size(); }
    bool hit(const Ray&, Scalar t_min, Scalar t_max, HitRecord&) const override;
    AABB bounding_box() const override { return nodes_.empty() ? AABB{} : nodes_.front().box; }
};

} // namespace beamcast
