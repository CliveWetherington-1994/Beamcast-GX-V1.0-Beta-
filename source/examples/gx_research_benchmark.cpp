#include <beamcast/beamcast.hpp>
#include <chrono>
#include <cmath>
#include <cstdint>
#include <iostream>
#include <vector>

int main(){
    using namespace beamcast;
    hx::PackedScene scene;const auto m=scene.add_lambertian({0.7f,0.7f,0.7f});
    for(int z=0;z<40;++z)for(int y=0;y<20;++y)for(int x=0;x<20;++x)
        scene.add_sphere({(x-10)*0.35f,(y-10)*0.35f,-2.0f-z*0.35f},0.12f,m);
    scene.build({hx::BuildMode::SAH,4,24});
    const auto b0=std::chrono::steady_clock::now();gx::WideBvh8 wide;wide.build(scene);const auto b1=std::chrono::steady_clock::now();
    std::vector<Ray> rays;for(int y=-120;y<=120;++y)for(int x=-160;x<=160;++x)rays.push_back({{0,0,4},unit_vector(Vec3{x*0.0035f,y*0.0035f,-1})});
    hx::TraceStats bs{},ws{};std::uint64_t bh=0,wh=0;
    const auto t0=std::chrono::steady_clock::now();for(const auto&r:rays){hx::PackedHit h;if(scene.hit(r,1e-4f,1000,h,&bs))++bh;}const auto t1=std::chrono::steady_clock::now();
    const auto t2=std::chrono::steady_clock::now();for(const auto&r:rays){hx::PackedHit h;if(wide.hit(r,1e-4f,1000,h,&ws))++wh;}const auto t3=std::chrono::steady_clock::now();
    std::vector<std::uint32_t> keys(rays.size());for(std::size_t i=0;i<keys.size();++i){const std::uint32_t h=static_cast<std::uint32_t>(i*2654435761u);keys[i]=h>>27;}const auto ordering=gx::evaluate_queue_ordering(keys,80.0);
    const double build_ms=std::chrono::duration<double,std::milli>(b1-b0).count();
    const double binary_ms=std::chrono::duration<double,std::milli>(t1-t0).count();
    const double wide_ms=std::chrono::duration<double,std::milli>(t3-t2).count();
    std::cout<<"primitives="<<scene.primitive_count()<<" rays="<<rays.size()<<" hits="<<bh<<'\n';
    std::cout<<"wide_build_ms="<<build_ms<<" wide_nodes="<<wide.nodes().size()<<" occupancy="<<wide.average_child_occupancy()<<" bytes="<<wide.byte_size()<<'\n';
    std::cout<<"binary_trace_ms="<<binary_ms<<" binary_box_tests="<<bs.box_tests<<"\n";
    std::cout<<"wide_trace_ms="<<wide_ms<<" wide_box_tests="<<ws.box_tests<<" hit_match="<<(bh==wh)<<"\n";
    std::cout<<"queue_coherence_before="<<ordering.before_coherence<<" after="<<ordering.after_coherence<<" sort_ns="<<ordering.measured_sort_ns<<" estimated_saved_ns="<<ordering.estimated_saved_ns<<" sort="<<ordering.sort<<'\n';
    return bh==wh?0:2;
}
