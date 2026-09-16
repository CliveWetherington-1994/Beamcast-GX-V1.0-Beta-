#pragma once
#include <memory>
#include "aabb.hpp"

namespace beamcast {
class Material;

struct HitRecord {
    Point3 point;
    Vec3 normal;
    std::shared_ptr<Material> material;
    Scalar t{0};
    Scalar u{0}, v{0};
    bool front_face{true};
    void set_face_normal(const Ray& ray, const Vec3& outward_normal) {
        front_face = dot(ray.direction, outward_normal) < 0;
        normal = front_face ? outward_normal : -outward_normal;
    }
};

class Hittable {
public:
    virtual ~Hittable() = default;
    virtual bool hit(const Ray&, Scalar t_min, Scalar t_max, HitRecord&) const = 0;
    virtual AABB bounding_box() const = 0;
};
} // namespace beamcast
