#pragma once
#include <memory>
#include <cmath>
#include <stdexcept>
#include "random.hpp"
#include "ray.hpp"
#include "texture.hpp"

namespace beamcast {
struct HitRecord;

class Material {
public:
    virtual ~Material() = default;
    virtual bool scatter(const Ray& in, const HitRecord& rec, Color& attenuation, Ray& scattered, RNG& rng) const = 0;
    virtual Color emitted(Scalar, Scalar, const Point3&) const { return {0,0,0}; }
};

class Lambertian final : public Material {
    std::shared_ptr<Texture> albedo_;
public:
    explicit Lambertian(Color c) : albedo_(std::make_shared<SolidColor>(c)) {}
    explicit Lambertian(std::shared_ptr<Texture> t) : albedo_(std::move(t)) { if (!albedo_) throw std::invalid_argument("Beamcast: Lambertian texture must not be null"); }
    bool scatter(const Ray&, const HitRecord&, Color&, Ray&, RNG&) const override;
};

class Metal final : public Material {
    Color albedo_;
    Scalar fuzz_;
public:
    Metal(Color c, Scalar fuzz=0) : albedo_(c), fuzz_(std::clamp(fuzz, Scalar(0), Scalar(1))) {}
    bool scatter(const Ray&, const HitRecord&, Color&, Ray&, RNG&) const override;
};

class Dielectric final : public Material {
    Scalar ior_;
    static Scalar reflectance(Scalar cosine, Scalar refraction_ratio);
public:
    explicit Dielectric(Scalar index_of_refraction=1.5f) : ior_(index_of_refraction) { if (!std::isfinite(ior_) || ior_ <= 0) throw std::invalid_argument("Beamcast: dielectric IOR must be positive and finite"); }
    bool scatter(const Ray&, const HitRecord&, Color&, Ray&, RNG&) const override;
};

class DiffuseLight final : public Material {
    std::shared_ptr<Texture> emit_;
public:
    explicit DiffuseLight(Color c) : emit_(std::make_shared<SolidColor>(c)) {}
    explicit DiffuseLight(std::shared_ptr<Texture> t) : emit_(std::move(t)) { if (!emit_) throw std::invalid_argument("Beamcast: light texture must not be null"); }
    bool scatter(const Ray&, const HitRecord&, Color&, Ray&, RNG&) const override { return false; }
    Color emitted(Scalar u, Scalar v, const Point3& p) const override { return emit_->value(u,v,p); }
};

} // namespace beamcast
