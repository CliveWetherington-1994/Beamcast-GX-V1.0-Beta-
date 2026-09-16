#pragma once
#include <array>
#include <cmath>
#include <memory>
#include <stdexcept>
#include "hittable.hpp"
#include "material.hpp"

namespace beamcast {
struct UV { Scalar u{0}, v{0}; };

class Triangle final : public Hittable {
    std::array<Point3,3> p_;
    std::array<Vec3,3> n_{};
    std::array<UV,3> uv_{};
    bool has_normals_{false}, has_uv_{false};
    std::shared_ptr<Material> material_;
public:
    Triangle(Point3 a, Point3 b, Point3 c, std::shared_ptr<Material> m)
        : p_{a,b,c}, material_(std::move(m)) {
        if(!material_) throw std::invalid_argument("Beamcast: triangle material must not be null");
        if(cross(b-a,c-a).length_squared()<=1e-20f) throw std::invalid_argument("Beamcast: triangle must have non-zero area");
    }
    Triangle(std::array<Point3,3> p, std::array<Vec3,3> n, std::array<UV,3> uv,
             bool has_normals, bool has_uv, std::shared_ptr<Material> m)
        : p_(p), n_(n), uv_(uv), has_normals_(has_normals), has_uv_(has_uv), material_(std::move(m)) {
        if(!material_) throw std::invalid_argument("Beamcast: triangle material must not be null");
        if(cross(p_[1]-p_[0],p_[2]-p_[0]).length_squared()<=1e-20f) throw std::invalid_argument("Beamcast: triangle must have non-zero area");
    }

    bool hit(const Ray& ray, Scalar t_min, Scalar t_max, HitRecord& rec) const override {
        constexpr Scalar eps=1e-7f;
        const Vec3 e1=p_[1]-p_[0], e2=p_[2]-p_[0];
        const Vec3 h=cross(ray.direction,e2);
        const Scalar det=dot(e1,h);
        if (std::fabs(det)<eps) return false;
        const Scalar inv=1/det;
        const Vec3 s=ray.origin-p_[0];
        const Scalar b1=inv*dot(s,h);
        if (b1<0 || b1>1) return false;
        const Vec3 q=cross(s,e1);
        const Scalar b2=inv*dot(ray.direction,q);
        if (b2<0 || b1+b2>1) return false;
        const Scalar t=inv*dot(e2,q);
        if (t<=t_min || t>=t_max) return false;
        const Scalar b0=1-b1-b2;
        const Vec3 geometric=unit_vector(cross(e1,e2));
        Vec3 outward = has_normals_ ? unit_vector(b0*n_[0]+b1*n_[1]+b2*n_[2]) : geometric;
        if(outward.near_zero()) outward=geometric;
        rec.t=t; rec.point=ray.at(t); rec.material=material_; rec.set_face_normal(ray,outward);
        if (has_uv_) { rec.u=b0*uv_[0].u+b1*uv_[1].u+b2*uv_[2].u; rec.v=b0*uv_[0].v+b1*uv_[1].v+b2*uv_[2].v; }
        else { rec.u=b1; rec.v=b2; }
        return true;
    }
    AABB bounding_box() const override {
        constexpr Scalar e=1e-5f;
        Point3 mn=min_components(p_[0],min_components(p_[1],p_[2]));
        Point3 mx=max_components(p_[0],max_components(p_[1],p_[2]));
        mn -= Vec3{e,e,e}; mx += Vec3{e,e,e}; return {mn,mx};
    }
};
} // namespace beamcast
