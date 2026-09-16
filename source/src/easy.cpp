#include "beamcast/easy.hpp"
#include "beamcast/renderer.hpp"

#include <algorithm>
#include <cmath>
#include <stdexcept>

namespace beamcast::easy {

Scene::Scene() {
    build_.mode=hx::BuildMode::SAH;
    build_.leaf_size=4;
    build_.sah_bins=24;
}

Material Scene::diffuse(Color color) { return {scene_.add_lambertian(color)}; }
Material Scene::metal(Color color, Scalar roughness) { return {scene_.add_metal(color,roughness)}; }
Material Scene::glass(Scalar ior) { return {scene_.add_dielectric(ior)}; }
Material Scene::light(Color emission) { return {scene_.add_emissive(emission)}; }

Scene& Scene::sphere(Point3 center, Scalar radius, Material material) {
    scene_.add_sphere(center,radius,material.id); dirty_=true; return *this;
}
Scene& Scene::triangle(Point3 a, Point3 b, Point3 c, Material material) {
    scene_.add_triangle(a,b,c,material.id); dirty_=true; return *this;
}
Scene& Scene::clear() { scene_.clear(); dirty_=false; return *this; }
Scene& Scene::acceleration(hx::BuildMode mode,std::uint32_t leaf_size,std::uint32_t sah_bins) {
    build_.mode=mode; build_.leaf_size=leaf_size; build_.sah_bins=sah_bins; dirty_=true; return *this;
}
Scene& Scene::build() { scene_.build(build_); dirty_=false; return *this; }
const hx::PackedScene& Scene::packed() { if(dirty_) build(); return scene_; }

beamcast::Camera Camera::make(Scalar aspect_ratio) const {
    Scalar focus=focus_distance;
    if(focus<=0) focus=(position-target).length();
    return beamcast::Camera(position,target,up,vertical_fov_degrees,aspect_ratio,aperture,focus);
}

static Options preset(Quality q,int w,int h) {
    Options o; o.width=w; o.height=h; o.quality=q; return o;
}
Options Options::draft(int w,int h){ return preset(Quality::Draft,w,h); }
Options Options::preview(int w,int h){ return preset(Quality::Preview,w,h); }
Options Options::balanced(int w,int h){ return preset(Quality::Balanced,w,h); }
Options Options::high(int w,int h){ return preset(Quality::High,w,h); }
Options Options::ultra(int w,int h){ return preset(Quality::Ultra,w,h); }

static gx::Settings make_settings(const Options& o) {
    if(o.width<=0||o.height<=0) throw std::invalid_argument("Beamcast Easy: width and height must be positive");
    if(o.width>32768||o.height>32768) throw std::invalid_argument("Beamcast Easy: dimensions above 32768 require the advanced API");
    gx::Settings s;
    s.width=o.width; s.height=o.height; s.thread_count=o.thread_count; s.denoise=o.denoise;
    s.variable_rate_sampling=o.variable_rate_sampling;
    s.foveation_strength=std::clamp(o.foveation_strength,Scalar(0),Scalar(0.95));
    s.direct_lighting=o.direct_lighting; s.seed=o.seed;
    switch(o.quality) {
        case Quality::Draft:    s.samples_per_pixel=1;  s.sample_batch=1; s.max_bounces=4;  s.light_candidates=1; break;
        case Quality::Preview:  s.samples_per_pixel=4;  s.sample_batch=1; s.max_bounces=6;  s.light_candidates=2; break;
        case Quality::Balanced: s.samples_per_pixel=8;  s.sample_batch=1; s.max_bounces=10; s.light_candidates=4; break;
        case Quality::High:     s.samples_per_pixel=24; s.sample_batch=2; s.max_bounces=12; s.light_candidates=6; break;
        case Quality::Ultra:    s.samples_per_pixel=64; s.sample_batch=4; s.max_bounces=16; s.light_candidates=8; break;
    }
    return s;
}

void Result::save_ppm(const std::string& path,bool binary) const { beamcast::save_ppm(path,frame.image,binary); }

Result render(Scene& scene,const Camera& camera,const Options& options,ProgressCallback progress) {
    const gx::Settings settings=make_settings(options);
    const Scalar aspect=static_cast<Scalar>(settings.width)/static_cast<Scalar>(settings.height);
    const beamcast::Camera cam=camera.make(aspect);
    auto callback = progress ? gx::ProgressCallback([&](int done,int total){
        const float p=total>0?static_cast<float>(done)/static_cast<float>(total):1.0f;
        progress(std::clamp(p,0.0f,1.0f));
    }) : gx::ProgressCallback{};
    Result result;
    result.frame=gx::render_wavefront(scene.packed(),cam,settings,std::move(callback));
    return result;
}

} // namespace beamcast::easy
