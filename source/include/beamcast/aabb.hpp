#pragma once
#include <algorithm>
#include <limits>
#include "ray.hpp"

namespace beamcast {

struct AABB {
    Point3 min{ std::numeric_limits<Scalar>::infinity(), std::numeric_limits<Scalar>::infinity(), std::numeric_limits<Scalar>::infinity() };
    Point3 max{ -std::numeric_limits<Scalar>::infinity(), -std::numeric_limits<Scalar>::infinity(), -std::numeric_limits<Scalar>::infinity() };

    AABB() = default;
    AABB(const Point3& a, const Point3& b) : min(a), max(b) {}

    Vec3 extent() const { return max - min; }
    Point3 centroid() const { return (min + max) * Scalar(0.5); }
    Scalar surface_area() const {
        const Vec3 e = extent();
        return Scalar(2) * (e.x*e.y + e.y*e.z + e.z*e.x);
    }
    bool valid() const { return min.x <= max.x && min.y <= max.y && min.z <= max.z; }

    void expand(const Point3& p) { min = min_components(min,p); max = max_components(max,p); }
    void expand(const AABB& b) { if (b.valid()) { expand(b.min); expand(b.max); } }

    bool intersect(const Ray& r, Scalar t_min, Scalar t_max, Scalar* t_enter = nullptr, Scalar* t_exit = nullptr) const {
        Scalar enter = t_min, exit = t_max;
        for (int axis=0; axis<3; ++axis) {
            const Scalar o = r.origin[axis];
            const Scalar d = r.direction[axis];
            if (std::fabs(d) < 1e-12f) {
                if (o < min[axis] || o > max[axis]) return false;
                continue;
            }
            Scalar t0 = (min[axis] - o) / d;
            Scalar t1 = (max[axis] - o) / d;
            if (t0 > t1) std::swap(t0,t1);
            enter = std::max(enter,t0);
            exit = std::min(exit,t1);
            if (exit < enter) return false;
        }
        if (t_enter) *t_enter = enter;
        if (t_exit) *t_exit = exit;
        return true;
    }
};

inline AABB surrounding_box(const AABB& a, const AABB& b) {
    if (!a.valid()) return b;
    if (!b.valid()) return a;
    return {min_components(a.min,b.min), max_components(a.max,b.max)};
}

} // namespace beamcast
