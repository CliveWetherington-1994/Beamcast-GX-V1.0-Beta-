#pragma once
#include <memory>
#include <cmath>
#include <stdexcept>
#include <string>
#include <vector>
#include "vec3.hpp"

namespace beamcast {

class Texture {
public:
    virtual ~Texture() = default;
    virtual Color value(Scalar u, Scalar v, const Point3& p) const = 0;
};

class SolidColor final : public Texture {
    Color color_;
public:
    SolidColor() = default;
    explicit SolidColor(Color c) : color_(c) {}
    SolidColor(Scalar r, Scalar g, Scalar b) : color_(r,g,b) {}
    Color value(Scalar, Scalar, const Point3&) const override { return color_; }
};

class CheckerTexture final : public Texture {
    Scalar scale_;
    std::shared_ptr<Texture> even_, odd_;
public:
    CheckerTexture(Scalar scale, std::shared_ptr<Texture> even, std::shared_ptr<Texture> odd)
        : scale_(scale), even_(std::move(even)), odd_(std::move(odd)) {
        if (!std::isfinite(scale_) || scale_ == 0) throw std::invalid_argument("Beamcast: checker scale must be finite and non-zero");
        if (!even_ || !odd_) throw std::invalid_argument("Beamcast: checker textures must not be null");
    }
    CheckerTexture(Scalar scale, Color a, Color b)
        : CheckerTexture(scale, std::make_shared<SolidColor>(a), std::make_shared<SolidColor>(b)) {}
    Color value(Scalar u, Scalar v, const Point3& p) const override;
};

class ImageTexture final : public Texture {
    int width_{0}, height_{0};
    std::vector<unsigned char> rgb_;
public:
    ImageTexture() = default;
    explicit ImageTexture(const std::string& ppm_path) { load_ppm(ppm_path); }
    bool load_ppm(const std::string& ppm_path);
    bool valid() const {
        if (width_ <= 0 || height_ <= 0) return false;
        const auto w=static_cast<std::size_t>(width_), h=static_cast<std::size_t>(height_);
        return h <= rgb_.max_size()/3 && w <= (rgb_.max_size()/3)/h && rgb_.size() == w*h*3;
    }
    Color value(Scalar u, Scalar v, const Point3& p) const override;
};

} // namespace beamcast
