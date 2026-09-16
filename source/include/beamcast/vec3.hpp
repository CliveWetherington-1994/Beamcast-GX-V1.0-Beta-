#pragma once
#include <algorithm>
#include <cmath>
#include <cstdint>
#include <ostream>

#if !defined(BEAMCAST_DISABLE_SIMD) && (defined(__SSE2__) || defined(_M_X64) || (defined(_M_IX86_FP) && _M_IX86_FP >= 2))
  #define BEAMCAST_HAS_SSE2 1
  #include <emmintrin.h>
  #include <xmmintrin.h>
#endif

namespace beamcast {

using Scalar = float;

struct alignas(16) Vec3 {
    Scalar x{0}, y{0}, z{0}, w{0};

    Vec3() = default;
    Vec3(Scalar x_, Scalar y_, Scalar z_) : x(x_), y(y_), z(z_), w(0) {}

    Vec3 operator-() const { return {-x, -y, -z}; }
    Vec3& operator+=(const Vec3& v) { x += v.x; y += v.y; z += v.z; return *this; }
    Vec3& operator-=(const Vec3& v) { x -= v.x; y -= v.y; z -= v.z; return *this; }
    Vec3& operator*=(Scalar s) { x *= s; y *= s; z *= s; return *this; }
    Vec3& operator/=(Scalar s) { return *this *= (Scalar(1) / s); }

    Scalar operator[](int i) const { return i == 0 ? x : (i == 1 ? y : z); }
    Scalar& operator[](int i) { return i == 0 ? x : (i == 1 ? y : z); }

    Scalar length_squared() const {
#if defined(BEAMCAST_HAS_SSE2)
        const __m128 v = _mm_load_ps(&x);
        const __m128 m = _mm_mul_ps(v, v);
        alignas(16) Scalar t[4];
        _mm_store_ps(t, m);
        return t[0] + t[1] + t[2];
#else
        return x*x + y*y + z*z;
#endif
    }
    Scalar length() const { return std::sqrt(length_squared()); }
    bool near_zero() const {
        constexpr Scalar e = 1e-8f;
        return std::fabs(x) < e && std::fabs(y) < e && std::fabs(z) < e;
    }
};

using Point3 = Vec3;
using Color = Vec3;

inline Vec3 operator+(Vec3 a, const Vec3& b) { return a += b; }
inline Vec3 operator-(Vec3 a, const Vec3& b) { return a -= b; }
inline Vec3 operator*(const Vec3& a, const Vec3& b) { return {a.x*b.x, a.y*b.y, a.z*b.z}; }
inline Vec3 operator*(Scalar s, Vec3 v) { return v *= s; }
inline Vec3 operator*(Vec3 v, Scalar s) { return v *= s; }
inline Vec3 operator/(Vec3 v, Scalar s) { return v /= s; }

inline Scalar dot(const Vec3& a, const Vec3& b) {
#if defined(BEAMCAST_HAS_SSE2)
    const __m128 va = _mm_load_ps(&a.x);
    const __m128 vb = _mm_load_ps(&b.x);
    const __m128 m = _mm_mul_ps(va, vb);
    alignas(16) Scalar t[4];
    _mm_store_ps(t, m);
    return t[0] + t[1] + t[2];
#else
    return a.x*b.x + a.y*b.y + a.z*b.z;
#endif
}
inline Vec3 cross(const Vec3& a, const Vec3& b) {
    return {a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x};
}
inline Vec3 unit_vector(Vec3 v) {
    const Scalar len = v.length();
    return len > 0 ? v / len : Vec3{};
}
inline Vec3 reflect(const Vec3& v, const Vec3& n) { return v - Scalar(2) * dot(v,n) * n; }
inline Vec3 refract(const Vec3& uv, const Vec3& n, Scalar eta) {
    const Scalar cos_theta = std::min(dot(-uv, n), Scalar(1));
    const Vec3 r_out_perp = eta * (uv + cos_theta * n);
    const Vec3 r_out_parallel = -std::sqrt(std::fabs(Scalar(1) - r_out_perp.length_squared())) * n;
    return r_out_perp + r_out_parallel;
}
inline Vec3 min_components(const Vec3& a, const Vec3& b) {
    return {std::min(a.x,b.x), std::min(a.y,b.y), std::min(a.z,b.z)};
}
inline Vec3 max_components(const Vec3& a, const Vec3& b) {
    return {std::max(a.x,b.x), std::max(a.y,b.y), std::max(a.z,b.z)};
}
inline std::ostream& operator<<(std::ostream& os, const Vec3& v) {
    return os << v.x << ' ' << v.y << ' ' << v.z;
}

} // namespace beamcast
