#pragma once
#include <algorithm>
#include <cmath>
#include <cstddef>
#include <cstdint>
#include <limits>
#include <stdexcept>
#include <vector>
#include "vec3.hpp"

namespace beamcast {

inline std::size_t checked_pixel_count(int width, int height) {
    if (width < 0 || height < 0)
        throw std::invalid_argument("Beamcast: framebuffer dimensions must be non-negative");
    if (width == 0 || height == 0) return 0;
    const auto w = static_cast<std::size_t>(width);
    const auto h = static_cast<std::size_t>(height);
    if (w > std::numeric_limits<std::size_t>::max() / h)
        throw std::length_error("Beamcast: framebuffer dimensions overflow addressable memory");
    const std::size_t count = w * h;
    if (count > std::vector<Color>{}.max_size())
        throw std::length_error("Beamcast: framebuffer is too large");
    return count;
}

struct Framebuffer {
    int width{0}, height{0};
    std::vector<Color> pixels;
    Framebuffer() = default;
    Framebuffer(int w, int h) : width(w), height(h), pixels(checked_pixel_count(w, h)) {}

    bool empty() const noexcept { return pixels.empty(); }
    std::size_t size() const noexcept { return pixels.size(); }

    // Hot-path access. Coordinates must be in range.
    Color& at(int x, int y) { return pixels[static_cast<std::size_t>(y) * static_cast<std::size_t>(width) + static_cast<std::size_t>(x)]; }
    const Color& at(int x, int y) const { return pixels[static_cast<std::size_t>(y) * static_cast<std::size_t>(width) + static_cast<std::size_t>(x)]; }
    Color& checked_at(int x, int y) {
        if(x<0||y<0||x>=width||y>=height) throw std::out_of_range("Beamcast: framebuffer coordinates out of range");
        return at(x,y);
    }
    const Color& checked_at(int x, int y) const {
        if(x<0||y<0||x>=width||y>=height) throw std::out_of_range("Beamcast: framebuffer coordinates out of range");
        return at(x,y);
    }

    std::vector<std::uint8_t> rgba8() const {
        if (pixels.empty()) return {};
        if (pixels.size() > std::numeric_limits<std::size_t>::max() / 4)
            throw std::length_error("Beamcast: RGBA conversion would overflow");
        std::vector<std::uint8_t> out(pixels.size() * 4);
        auto enc=[](Scalar c){
            if (!std::isfinite(c)) c = Scalar(0);
            c=std::sqrt(std::max(Scalar(0),c));
            return static_cast<std::uint8_t>(256*std::clamp(c,Scalar(0),Scalar(0.999)));
        };
        for (std::size_t i=0;i<pixels.size();++i) {
            out[4*i+0]=enc(pixels[i].x); out[4*i+1]=enc(pixels[i].y); out[4*i+2]=enc(pixels[i].z); out[4*i+3]=255;
        }
        return out;
    }
};

} // namespace beamcast
