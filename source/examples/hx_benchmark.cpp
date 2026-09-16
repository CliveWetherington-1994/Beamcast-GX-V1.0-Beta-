#include <beamcast/beamcast.hpp>
#include <chrono>
#include <cstdint>
#include <iostream>
#include <random>
#include <vector>

using Clock=std::chrono::steady_clock;

static beamcast::hx::PackedScene make_scene(beamcast::hx::BuildMode mode) {
    using namespace beamcast; using namespace beamcast::hx;
    PackedScene s; auto m=s.add_lambertian({0.7f,0.7f,0.7f});
    std::mt19937 gen(123); std::uniform_real_distribution<float> p(-50,50),r(0.08f,0.35f);
    for(int i=0;i<25000;++i){ float rr=r(gen); s.add_sphere({p(gen),p(gen)*0.3f,p(gen)-55.0f},rr,m); }
    PackedScene::BuildOptions o; o.mode=mode; o.leaf_size=4; o.sah_bins=24; s.build(o); return s;
}

int main(){
    using namespace beamcast; using namespace beamcast::hx;
    std::vector<Ray> rays; rays.reserve(200000); RNG rng(77);
    for(int i=0;i<200000;++i) rays.push_back({{0,0,20},unit_vector(Vec3{rng.uniform()*2-1,rng.uniform()*0.8f-0.4f,-1.0f})});
    for(auto mode:{BuildMode::SAH,BuildMode::Morton}){
        auto t0=Clock::now(); auto s=make_scene(mode); auto t1=Clock::now();
        std::uint64_t hitn=0; TraceStats st{}; PackedHit h;
        auto q0=Clock::now(); for(const auto& r:rays) hitn+=s.hit(r,0.001f,1e30f,h,&st); auto q1=Clock::now();
        std::cout<<(mode==BuildMode::SAH?"SAH":"Morton")
                 <<" build_ms="<<std::chrono::duration<double,std::milli>(t1-t0).count()
                 <<" trace_ms="<<std::chrono::duration<double,std::milli>(q1-q0).count()
                 <<" nodes="<<s.node_count()<<" hits="<<hitn
                 <<" box_tests="<<st.box_tests<<" prim_tests="<<st.primitive_tests<<"\n";
    }
}
