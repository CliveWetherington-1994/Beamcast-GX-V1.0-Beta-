#pragma once
#include <memory>
#include <utility>
#include <stdexcept>
#include <vector>
#include "bvh.hpp"
#include "uniform_grid.hpp"

namespace beamcast {

enum class Acceleration { Linear, BVH_SAH, UniformGrid };

class World final : public Hittable {
    std::vector<std::shared_ptr<Hittable>> objects_;
    std::shared_ptr<Hittable> accelerator_;
    Acceleration acceleration_{Acceleration::Linear};
public:
    void clear() { objects_.clear(); accelerator_.reset(); acceleration_=Acceleration::Linear; }
    void add(std::shared_ptr<Hittable> object) {
        if (!object) throw std::invalid_argument("Beamcast: cannot add a null object to World");
        objects_.push_back(std::move(object)); accelerator_.reset(); acceleration_=Acceleration::Linear;
    }
    std::size_t size() const { return objects_.size(); }
    const std::vector<std::shared_ptr<Hittable>>& objects() const { return objects_; }
    void build_acceleration(Acceleration type=Acceleration::BVH_SAH);
    Acceleration acceleration() const { return acceleration_; }
    bool hit(const Ray&, Scalar t_min, Scalar t_max, HitRecord&) const override;
    AABB bounding_box() const override;
};

} // namespace beamcast
