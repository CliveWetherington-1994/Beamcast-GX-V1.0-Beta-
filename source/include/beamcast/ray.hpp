#pragma once
#include "vec3.hpp"
namespace beamcast {
struct Ray {
    Point3 origin;
    Vec3 direction;
    Ray() = default;
    Ray(const Point3& o, const Vec3& d) : origin(o), direction(d) {}
    Point3 at(Scalar t) const { return origin + t * direction; }
};
} // namespace beamcast
