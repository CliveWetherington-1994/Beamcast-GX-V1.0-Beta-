#pragma once
#include <cmath>
#include <stdexcept>
#include "random.hpp"
#include "ray.hpp"

namespace beamcast {
class Camera {
    Point3 origin_;
    Point3 lower_left_;
    Vec3 horizontal_, vertical_, u_, v_, w_;
    Scalar lens_radius_{0};
public:
    Camera(Point3 look_from={3,2,2}, Point3 look_at={0,0,-1}, Vec3 up={0,1,0},
           Scalar vertical_fov_degrees=40, Scalar aspect_ratio=16.0f/9.0f,
           Scalar aperture=0, Scalar focus_distance=1) {
        auto finite3=[](const Vec3& q){ return std::isfinite(q.x)&&std::isfinite(q.y)&&std::isfinite(q.z); };
        if(!finite3(look_from)||!finite3(look_at)||!finite3(up))
            throw std::invalid_argument("Beamcast: camera vectors must be finite");
        if(!std::isfinite(vertical_fov_degrees)||vertical_fov_degrees<=0||vertical_fov_degrees>=179)
            throw std::invalid_argument("Beamcast: camera vertical FOV must be between 0 and 179 degrees");
        if(!std::isfinite(aspect_ratio)||aspect_ratio<=0)
            throw std::invalid_argument("Beamcast: camera aspect ratio must be positive");
        if(!std::isfinite(aperture)||aperture<0)
            throw std::invalid_argument("Beamcast: camera aperture must be non-negative");
        if(!std::isfinite(focus_distance)||focus_distance<=0)
            throw std::invalid_argument("Beamcast: camera focus distance must be positive");
        const Vec3 view=look_from-look_at;
        if(view.length_squared()<=1e-16f)
            throw std::invalid_argument("Beamcast: camera position and target must differ");
        if(up.length_squared()<=1e-16f)
            throw std::invalid_argument("Beamcast: camera up vector must be non-zero");

        constexpr Scalar pi=3.14159265358979323846f;
        const Scalar theta=vertical_fov_degrees*pi/180;
        const Scalar h=std::tan(theta/2);
        const Scalar viewport_h=2*h, viewport_w=aspect_ratio*viewport_h;
        w_=unit_vector(view);
        const Vec3 right=cross(up,w_);
        if(right.length_squared()<=1e-16f)
            throw std::invalid_argument("Beamcast: camera up vector must not be parallel to the view direction");
        u_=unit_vector(right);
        v_=cross(w_,u_);
        origin_=look_from;
        horizontal_=focus_distance*viewport_w*u_;
        vertical_=focus_distance*viewport_h*v_;
        lower_left_=origin_-horizontal_/2-vertical_/2-focus_distance*w_;
        lens_radius_=aperture/2;
    }
    Ray ray(Scalar s, Scalar t, RNG& rng) const {
        const Vec3 rd=lens_radius_*rng.in_unit_disk();
        const Vec3 offset=u_*rd.x+v_*rd.y;
        return {origin_+offset, lower_left_+s*horizontal_+t*vertical_-origin_-offset};
    }
    // Project a world point through the pinhole center. Lens jitter is intentionally ignored
    // so motion vectors remain stable when depth-of-field is enabled.
    bool project(const Point3& p, Scalar& s, Scalar& t, Scalar* view_depth=nullptr) const {
        const Vec3 rel=p-origin_;
        const Scalar depth=-dot(rel,w_);
        if(view_depth) *view_depth=depth;
        if(depth<=1e-6f) return false;
        const Point3 center=lower_left_+horizontal_*Scalar(0.5)+vertical_*Scalar(0.5);
        const Scalar focus=dot(origin_-center,w_);
        if(focus<=1e-6f) return false;
        const Point3 on_plane=origin_+rel*(focus/depth);
        const Vec3 q=on_plane-lower_left_;
        const Scalar hw=horizontal_.length_squared(), vh=vertical_.length_squared();
        if(hw<=1e-12f||vh<=1e-12f) return false;
        s=dot(q,horizontal_)/hw;
        t=dot(q,vertical_)/vh;
        return true;
    }
    Point3 origin() const { return origin_; }
};
} // namespace beamcast
