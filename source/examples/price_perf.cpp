#include <beamcast/beamcast.hpp>
#include <chrono>
#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <string>

int main(int argc,char** argv){
    using namespace beamcast;
    std::string label="host-cpu";
    double price=0;
    if(argc>1)label=argv[1];
    if(argc>2)price=std::max(0.0,std::atof(argv[2]));

    hx::PackedScene scene;
    const auto ground=scene.add_lambertian({0.55f,0.55f,0.58f});
    const auto red=scene.add_lambertian({0.8f,0.15f,0.08f});
    const auto metal=scene.add_metal({0.85f,0.9f,0.95f},0.08f);
    const auto light=scene.add_emissive({14,12,10});
    scene.add_sphere({0,-100.5f,-2},100,ground);
    scene.add_sphere({-0.7f,0,-2.5f},0.5f,red);
    scene.add_sphere({0.7f,0,-2.8f},0.5f,metal);
    scene.add_sphere({0,3,-2.5f},0.65f,light);
    scene.build({hx::BuildMode::SAH,3,24});

    const int w=160,h=90;
    Camera camera({3,1.7f,3.5f},{0,0,-2.5f},{0,1,0},42.0f,static_cast<float>(w)/h,0,5);
    gx::Settings reference_s;reference_s.width=w;reference_s.height=h;reference_s.samples_per_pixel=24;reference_s.max_bounces=10;reference_s.denoise=false;reference_s.variable_rate_sampling=false;
    auto reference=gx::render_wavefront(scene,camera,reference_s);

    gx::Settings test_s=reference_s;test_s.samples_per_pixel=4;test_s.max_bounces=7;test_s.variable_rate_sampling=true;test_s.foveation_strength=0.55f;test_s.minimum_pixel_samples=1;test_s.denoise=true;test_s.denoise_iterations=3;
    const auto e0=gx::read_linux_rapl_joules();
    const auto t0=std::chrono::steady_clock::now();
    auto test=gx::render_wavefront(scene,camera,test_s);
    const auto t1=std::chrono::steady_clock::now();
    const auto e1=gx::read_linux_rapl_joules();
    const double ms=std::chrono::duration<double,std::milli>(t1-t0).count();
    double joules=0;if(e0&&e1&&*e1>=*e0)joules=*e1-*e0;
    const auto rec=gx::make_price_performance_record(label,gx::NativeBackend::CPU,price,ms,joules,gx::process_resident_bytes(),test.image,reference.image);
    std::cout<<gx::price_performance_csv_header()<<'\n'<<gx::price_performance_csv_row(rec)<<'\n';
    std::cerr<<"reference_ms="<<reference.report.total_seconds*1000.0<<" test_ms="<<ms
             <<" psnr="<<rec.quality.psnr_db<<" ssim="<<rec.quality.ssim
             <<" primary_paths="<<test.report.primary_paths<<'\n';
    return 0;
}
