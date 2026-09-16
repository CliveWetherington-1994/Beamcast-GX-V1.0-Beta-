#include "studio_core.hpp"

#include <chrono>
#include <cstdlib>
#include <iostream>
#include <stdexcept>
#include <string>
#include <thread>

using namespace beamcast::studio;

namespace {
RenderPreset parse_quality(const std::string& s) {
    if (s=="draft") return RenderPreset::Draft;
    if (s=="preview") return RenderPreset::Preview;
    if (s=="balanced") return RenderPreset::Balanced;
    if (s=="high") return RenderPreset::High;
    if (s=="ultra") return RenderPreset::Ultra;
    throw std::runtime_error("Unknown quality: " + s);
}

void usage() {
    std::cout <<
        "Beamcast CLI 5.3\n"
        "Usage: beamcast-cli [options]\n\n"
        "  --scene showcase|cornell|stress|file.obj\n"
        "  --quality draft|preview|balanced|high|ultra\n"
        "  --width N --height N\n"
        "  --no-denoise\n"
        "  --vrs\n"
        "  --output render.ppm\n"
        "  --help\n";
}
}

int main(int argc, char** argv) {
    try {
        std::string scene="showcase", output="beamcast_render.ppm";
        StudioSettings settings;
        for (int i=1;i<argc;++i) {
            std::string a=argv[i];
            auto next=[&]()->std::string{if(i+1>=argc)throw std::runtime_error("Missing value after "+a);return argv[++i];};
            if(a=="--scene")scene=next();
            else if(a=="--quality")settings.preset=parse_quality(next());
            else if(a=="--width")settings.width=std::stoi(next());
            else if(a=="--height")settings.height=std::stoi(next());
            else if(a=="--output")output=next();
            else if(a=="--no-denoise")settings.denoise=false;
            else if(a=="--vrs")settings.variable_rate=true;
            else if(a=="--help"||a=="-h"){usage();return 0;}
            else throw std::runtime_error("Unknown option: "+a);
        }

        StudioCore core;
        if(scene=="showcase") core.load_preset(ScenePreset::Showcase);
        else if(scene=="cornell") core.load_preset(ScenePreset::CornellLike);
        else if(scene=="stress") core.load_preset(ScenePreset::StressGrid);
        else core.import_obj(scene);
        core.set_settings(settings);
        core.start_render();
        int last=-1;
        while(core.snapshot().rendering) {
            int p=static_cast<int>(core.snapshot().progress*100.0f);
            if(p!=last){std::cout<<"\rRendering "<<p<<"%"<<std::flush;last=p;}
            std::this_thread::sleep_for(std::chrono::milliseconds(50));
        }
        core.wait_for_render();
        auto snap=core.snapshot();
        std::cout<<"\rRendering 100%\n";
        if(!snap.has_image) throw std::runtime_error(snap.status);
        core.save_image(output);
        std::cout<<"Saved "<<output<<"\n"
                 <<"Time: "<<snap.seconds<<" s\n"
                 <<"Rays/s: "<<static_cast<long long>(snap.rays_per_second)<<"\n";
        return 0;
    } catch(const std::exception& e) {
        std::cerr<<"Beamcast CLI error: "<<e.what()<<"\n";
        usage();
        return 1;
    }
}
