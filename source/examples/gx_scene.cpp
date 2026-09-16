#include <beamcast/beamcast.hpp>
#include <fstream>
#include <iostream>

int main() {
    using namespace beamcast;
    hx::PackedScene scene;
    const auto white=scene.add_lambertian({0.72f,0.72f,0.72f});
    const auto red=scene.add_lambertian({0.75f,0.08f,0.05f});
    const auto blue=scene.add_lambertian({0.06f,0.16f,0.75f});
    const auto metal=scene.add_metal({0.94f,0.95f,0.98f},0.03f);
    const auto glass=scene.add_dielectric(1.5f);
    const auto light=scene.add_emissive({18,15,11});
    scene.add_sphere({0,-1000,0},999.0f,white);
    scene.add_sphere({-1.2f,0,-3.4f},1.0f,glass);
    scene.add_sphere({1.25f,0,-3.1f},1.0f,metal);
    scene.add_sphere({0.1f,-0.35f,-1.2f},0.45f,red);
    scene.add_sphere({-2.6f,-0.3f,-2.0f},0.5f,blue);
    scene.add_sphere({0,5,-3},1.0f,light);
    hx::PackedScene::BuildOptions bo; bo.mode=hx::BuildMode::SAH; bo.leaf_size=4; bo.sah_bins=24; scene.build(bo);

    gx::Settings rs; rs.width=960; rs.height=540; rs.samples_per_pixel=12; rs.max_bounces=12;
    rs.light_candidates=6; rs.denoise=true; rs.denoise_iterations=4;
    Camera camera({6,3.2f,5.5f},{0,0,-2.8f},{0,1,0},38.0f,static_cast<float>(rs.width)/rs.height,0.02f,8.0f);
    const auto r=gx::render_wavefront(scene,camera,rs,[](int d,int n){ if(d==n) std::cerr << "samples complete\n"; });
    std::ofstream out("beamcast_gx.ppm",std::ios::binary);
    out << "P6\n" << r.image.width << ' ' << r.image.height << "\n255\n";
    const auto rgba=r.image.rgba8();
    for(std::size_t i=0;i<rgba.size();i+=4){out.put(static_cast<char>(rgba[i]));out.put(static_cast<char>(rgba[i+1]));out.put(static_cast<char>(rgba[i+2]));}
    std::cout << "GX render " << r.report.total_seconds << " s, " << r.report.rays_per_second/1e6 << " Mray/s, peak queue " << r.report.peak_active_paths << "\n";
    std::cout << "GPU scene payload " << r.report.gpu_export_bytes/1024.0 << " KiB\n";
}
