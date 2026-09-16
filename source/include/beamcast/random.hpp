#pragma once
#include <cstdint>
#include <limits>
#include "vec3.hpp"

namespace beamcast {

class RNG {
    std::uint64_t state_;
public:
    explicit RNG(std::uint64_t seed = 0x853c49e6748fea9bULL) : state_(seed ? seed : 1ULL) {}

    std::uint64_t next_u64() {
        std::uint64_t x = state_;
        x ^= x >> 12; x ^= x << 25; x ^= x >> 27;
        state_ = x;
        return x * 2685821657736338717ULL;
    }
    Scalar uniform() {
        const std::uint32_t v = static_cast<std::uint32_t>(next_u64() >> 40);
        return static_cast<Scalar>(v) / static_cast<Scalar>(1u << 24);
    }
    Scalar uniform(Scalar lo, Scalar hi) { return lo + (hi - lo) * uniform(); }
    Vec3 vec(Scalar lo = 0, Scalar hi = 1) { return {uniform(lo,hi), uniform(lo,hi), uniform(lo,hi)}; }
    Vec3 in_unit_sphere() {
        for (;;) {
            const Vec3 p = vec(-1,1);
            if (p.length_squared() < 1) return p;
        }
    }
    Vec3 unit_vector() { return beamcast::unit_vector(in_unit_sphere()); }
    Vec3 in_unit_disk() {
        for (;;) {
            const Vec3 p{uniform(-1,1), uniform(-1,1), 0};
            if (p.length_squared() < 1) return p;
        }
    }
};

inline std::uint64_t hash_seed(std::uint64_t base, std::uint64_t a, std::uint64_t b = 0) {
    std::uint64_t x = base ^ (a + 0x9e3779b97f4a7c15ULL + (base<<6) + (base>>2));
    x ^= b + 0x9e3779b97f4a7c15ULL + (x<<6) + (x>>2);
    x ^= x >> 30; x *= 0xbf58476d1ce4e5b9ULL;
    x ^= x >> 27; x *= 0x94d049bb133111ebULL;
    return x ^ (x >> 31);
}

} // namespace beamcast
