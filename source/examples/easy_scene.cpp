#include <beamcast/beamcast.hpp>
#include <iostream>

int main() {
    using namespace beamcast;

    easy::Scene scene;
    auto ground=scene.diffuse({0.72f,0.72f,0.72f});
    auto red=scene.diffuse({0.80f,0.12f,0.08f});
    auto metal=scene.metal({0.92f,0.94f,0.98f},0.05f);
    auto glass=scene.glass(1.5f);
    auto light=scene.light({15.0f,12.0f,9.0f});

    scene.sphere({0,-1001,-3},1000,ground)
         .sphere({0,0,-3},1,glass)
         .sphere({-2,0,-4},1,red)
         .sphere({2,0,-4},1,metal)
         .sphere({0,5,-3},0.8f,light);

    easy::Camera camera;
    camera.position={6,3,5};
    camera.target={0,0,-3};
    camera.vertical_fov_degrees=38;
    camera.aperture=0.02f;

    auto options=easy::Options::preview(1280,720);
    auto result=easy::render(scene,camera,options,[](float p){
        std::cout << "\rRendering " << int(p*100.0f) << "%" << std::flush;
    });
    result.save_ppm("beamcast_easy.ppm");
    std::cout << "\nSaved beamcast_easy.ppm in " << result.report().total_seconds << " s\n";
}
