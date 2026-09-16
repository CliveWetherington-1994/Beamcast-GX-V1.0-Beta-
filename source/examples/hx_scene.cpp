#include <beamcast/beamcast.hpp>
#include <chrono>
#include <fstream>
#include <iostream>
#include <random>

int main() {
    using namespace beamcast;
    using namespace beamcast::hx;

    PackedScene scene;
    const auto ground = scene.add_lambertian({0.38f,0.40f,0.42f});
    const auto red = scene.add_lambertian({0.72f,0.12f,0.08f});
    const auto blue = scene.add_lambertian({0.08f,0.20f,0.74f});
    const auto metal = scene.add_metal({0.88f,0.90f,0.94f},0.04f);
    const auto glass = scene.add_dielectric(1.5f);
    const auto light = scene.add_emissive({10.0f,8.8f,6.0f});

    scene.add_sphere({0,-1001,0},1000,ground);
    scene.add_sphere({-1.35f,0.75f,-2.3f},0.75f,glass);
    scene.add_sphere({0.25f,0.65f,-2.0f},0.65f,metal);
    scene.add_sphere({1.5f,0.55f,-2.8f},0.55f,red);
    scene.add_sphere({0,5.5f,-2},1.0f,light);

    std::mt19937 gen(42);
    std::uniform_real_distribution<float> pos(-6.0f,6.0f), rad(0.08f,0.22f), col(0.1f,0.9f);
    for(int i=0;i<1200;++i){
        float x=pos(gen), z=pos(gen)-5.0f, r=rad(gen);
        auto m=scene.add_lambertian({col(gen),col(gen),col(gen)});
        scene.add_sphere({x,r,z},r,m);
    }

    PackedScene::BuildOptions bo; bo.mode=BuildMode::SAH; bo.leaf_size=4; bo.sah_bins=24;
    scene.build(bo);

    beamcast::hx::RenderSettings rs;
    rs.width=960; rs.height=540; rs.min_samples=8; rs.max_samples=96; rs.max_bounces=12;
    rs.adaptive_sampling=true; rs.adaptive_threshold=0.02f; rs.tile_size=16;
    Camera cam({5.5f,3.4f,6.5f},{0.0f,0.7f,-3.0f},{0,1,0},38.0f,float(rs.width)/rs.height,0.04f,10.5f);

    RenderReport report;
    auto fb=render(scene,cam,rs,&report);
    std::ofstream out("beamcast_hx.ppm",std::ios::binary);
    beamcast::write_ppm(out,fb);
    std::cout << "Beamcast HX\n"
              << "primitives: " << scene.primitive_count() << "\n"
              << "BVH nodes: " << scene.node_count() << "\n"
              << "camera samples: " << report.camera_samples << "\n"
              << "path rays: " << report.path_rays << "\n"
              << "box tests: " << report.box_tests << "\n"
              << "primitive tests: " << report.primitive_tests << "\n"
              << "seconds: " << report.seconds << "\n"
              << "rays/s: " << report.rays_per_second << "\n";
}
