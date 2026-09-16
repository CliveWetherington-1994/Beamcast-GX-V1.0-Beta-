#include <beamcast/beamcast.hpp>
#include <iostream>
#include <random>

int main(){
    using namespace beamcast;
    hx::PackedScene scene;
    auto matte=scene.add_lambertian({0.65f,0.68f,0.72f});
    auto light=scene.add_emissive({20,17,12});
    std::mt19937 rng(42); std::uniform_real_distribution<float> d(-40,40),r(0.08f,0.28f);
    for(int i=0;i<30000;++i) scene.add_sphere({d(rng),d(rng)*0.25f-5,d(rng)-40},r(rng),matte);
    scene.add_sphere({0,18,-40},3.0f,light);
    hx::PackedScene::BuildOptions bo;bo.mode=hx::BuildMode::Morton;bo.leaf_size=4;scene.build(bo);
    Camera c({0,1,8},{0,-2,-40},{0,1,0},50.0f,16.0f/9.0f,0,48.0f);

    gx::Settings fixed;fixed.width=320;fixed.height=180;fixed.samples_per_pixel=4;fixed.max_bounces=5;fixed.denoise=false;fixed.light_candidates=2;
    auto a=gx::render_wavefront(scene,c,fixed);

    gx::Settings vrs=fixed;vrs.variable_rate_sampling=true;vrs.foveation_strength=0.82f;vrs.minimum_pixel_samples=1;
    auto b=gx::render_wavefront(scene,c,vrs);

    std::cout << "primitives="<<scene.primitive_count()<<" nodes="<<scene.node_count()<<"\n";
    std::cout << "full_gpu_MiB="<<a.report.gpu_export_bytes/(1024.0*1024.0)
              <<" quantized_gpu_MiB="<<a.report.gpu_quantized_export_bytes/(1024.0*1024.0)<<"\n";
    std::cout << "fixed paths="<<a.report.primary_paths<<" rays="<<(a.report.path_rays+a.report.shadow_rays)
              <<" ms="<<a.report.render_seconds*1000.0<<" Mray/s="<<a.report.rays_per_second/1e6<<"\n";
    std::cout << "vrs   paths="<<b.report.primary_paths<<" rays="<<(b.report.path_rays+b.report.shadow_rays)
              <<" ms="<<b.report.render_seconds*1000.0<<" skipped="<<b.report.pixels_skipped_by_vrs<<"\n";
    const double path_save=100.0*(1.0-double(b.report.primary_paths)/double(a.report.primary_paths));
    const double time_save=100.0*(1.0-b.report.render_seconds/a.report.render_seconds);
    std::cout << "vrs_primary_path_reduction_pct="<<path_save<<" time_reduction_pct="<<time_save<<"\n";
}
