#pragma once
#include <cmath>
#include <memory>
#include <stdexcept>
#include <utility>
#include "hittable.hpp"
#include "material.hpp"

namespace beamcast {
class Sphere final : public Hittable {
    Point3 center_;
    Scalar radius_;
    std::shared_ptr<Material> material_;
    static void uv(const Point3& p, Scalar& u, Scalar& v) {
        constexpr Scalar pi = 3.14159265358979323846f;
        const Scalar theta = std::acos(std::clamp(-p.y, Scalar(-1), Scalar(1)));
        const Scalar phi = std::atan2(-p.z, p.x) + pi;
        u = phi / (2*pi); v = theta / pi;
    }
public:
    Sphere(Point3 c, Scalar r, std::shared_ptr<Material> m) : center_(c), radius_(r), material_(std::move(m)) {
        if (!std::isfinite(r) || r <= 0) throw std::invalid_argument("Beamcast: sphere radius must be positive and finite");
        if (!material_) throw std::invalid_argument("Beamcast: sphere material must not be null");
    }
    bool hit(const Ray& ray, Scalar t_min, Scalar t_max, HitRecord& rec) const override {
        const Vec3 oc = ray.origin - center_;
        const Scalar a = ray.direction.length_squared();
        if (!std::isfinite(a) || a <= 1e-20f) return false;
        const Scalar half_b = dot(oc, ray.direction);
        const Scalar c = oc.length_squared() - radius_*radius_;
        const Scalar d = half_b*half_b - a*c;
        if (d < 0) return false;
        const Scalar s = std::sqrt(d);
        Scalar root = (-half_b - s)/a;
        if (root <= t_min || root >= t_max) {
            root = (-half_b + s)/a;
            if (root <= t_min || root >= t_max) return false;
        }
        rec.t=root; rec.point=ray.at(root); rec.material=material_;
        const Vec3 outward=(rec.point-center_)/radius_;
        rec.set_face_normal(ray,outward); uv(outward,rec.u,rec.v);
        return true;
    }
    AABB bounding_box() const override {
        const Vec3 r{radius_,radius_,radius_}; return {center_-r, center_+r};
    }
};
} // namespace beamcast
