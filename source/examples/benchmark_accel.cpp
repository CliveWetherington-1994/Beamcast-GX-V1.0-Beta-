#include <chrono>
#include <iostream>
#include <memory>
#include <vector>
#include "beamcast/beamcast.hpp"

int main() {
    using namespace beamcast;
    auto mat=std::make_shared<Lambertian>(Color{0.5f,0.5f,0.5f});
    std::vector<std::shared_ptr<Hittable>> objects;
    for (int z=0;z<12;++z) for (int y=0;y<8;++y) for (int x=0;x<12;++x)
        objects.push_back(std::make_shared<Sphere>(Point3{(x-6)*0.38f,(y-4)*0.38f,-1.5f-z*0.42f},0.15f,mat));

    World linear; for (auto& o:objects) linear.add(o);
    World bvh; for (auto& o:objects) bvh.add(o); bvh.build_acceleration(Acceleration::BVH_SAH);
    World grid; for (auto& o:objects) grid.add(o); grid.build_acceleration(Acceleration::UniformGrid);

    std::vector<Ray> rays; rays.reserve(120000);
    RNG rng(12345);
    for (int i=0;i<120000;++i) rays.emplace_back(Point3{0,0,3}, unit_vector(Vec3{rng.uniform(-0.9f,0.9f),rng.uniform(-0.65f,0.65f),-1}));

    auto run=[&](const char* name,const World& w){
        auto start=std::chrono::high_resolution_clock::now(); std::size_t hits=0; HitRecord rec;
        for (const auto& r:rays) hits += w.hit(r,0.001f,1000,rec)?1:0;
        auto ms=std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::high_resolution_clock::now()-start).count();
        std::cout<<name<<": "<<ms<<" ms, hits="<<hits<<'\n';
    };
    run("linear",linear); run("SAH BVH",bvh); run("uniform grid",grid);
}
