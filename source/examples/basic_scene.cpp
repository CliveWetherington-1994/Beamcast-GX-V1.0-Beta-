#include <fstream>
#include <iostream>
#include <memory>
#include "beamcast/beamcast.hpp"

int main(int argc, char** argv) {
    using namespace beamcast;
    World world;

    auto checker = std::make_shared<CheckerTexture>(1.5f, Color{0.12f,0.16f,0.12f}, Color{0.75f,0.75f,0.72f});
    auto ground = std::make_shared<Lambertian>(checker);
    auto red = std::make_shared<Lambertian>(Color{0.72f,0.12f,0.10f});
    auto metal = std::make_shared<Metal>(Color{0.82f,0.85f,0.90f}, 0.08f);
    auto glass = std::make_shared<Dielectric>(1.5f);
    auto light = std::make_shared<DiffuseLight>(Color{5.0f,4.2f,3.2f});

    world.add(std::make_shared<Sphere>(Point3{0,-100.5f,-1},100,ground));
    world.add(std::make_shared<Sphere>(Point3{-1.1f,0.05f,-1.4f},0.55f,glass));
    world.add(std::make_shared<Sphere>(Point3{0.15f,0.05f,-1.0f},0.55f,red));
    world.add(std::make_shared<Sphere>(Point3{1.35f,0.10f,-1.6f},0.60f,metal));
    world.add(std::make_shared<Sphere>(Point3{0.2f,3.0f,-1.0f},0.65f,light));

    const char* obj = argc > 1 ? argv[1] : "examples/assets/pyramid.obj";
    try {
        auto mesh_mat=std::make_shared<Lambertian>(Color{0.16f,0.38f,0.78f});
        auto mesh=Mesh::load_obj(obj,mesh_mat,true);
        if (mesh->triangle_count()) world.add(mesh);
    } catch (const std::exception& e) {
        std::cerr << "OBJ skipped: " << e.what() << '\n';
    }

    world.build_acceleration(Acceleration::BVH_SAH);

    RenderSettings s;
    s.width=640; s.height=360; s.samples_per_pixel=32; s.max_bounces=12; s.tile_size=16;
    Camera camera(Point3{4.6f,2.4f,4.2f}, Point3{0,0.35f,-1.0f}, Vec3{0,1,0}, 36.0f,
                  static_cast<Scalar>(s.width)/s.height, 0.08f, 6.0f);

    std::cerr << "Beamcast rendering with " << world.size() << " top-level objects...\n";
    Framebuffer fb=render(world,camera,s,[](int done,int total){
        if (done==total || done%(std::max(1,total/10))==0) std::cerr << "  " << (100*done/total) << "%\n";
    });
    std::ofstream out("beamcast_render.ppm");
    write_ppm(out,fb);
    std::cout << "Wrote beamcast_render.ppm\n";
}
