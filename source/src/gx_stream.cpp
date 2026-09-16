#include "beamcast/gx_stream.hpp"
#include "beamcast/gx.hpp"
#include <algorithm>
#include <cmath>
#include <fstream>
#include <limits>
#include <stdexcept>

namespace beamcast::gx {
namespace {
constexpr std::uint64_t kMagic=0x3145474150584742ULL; // "BGXPAGE1" little endian-ish tag
struct FileHeader { std::uint64_t magic; std::uint32_t version; std::uint32_t page_count; };
struct DiskPage { std::uint64_t offset; std::uint32_t count; std::uint32_t reserved; };
}

void PagedTriangleStore::write(const std::string& path,const std::vector<hx::PackedTriangle>& triangles,std::uint32_t per_page) {
    if(per_page==0) throw std::invalid_argument("triangles_per_page must be non-zero");
    const std::uint32_t pages=static_cast<std::uint32_t>((triangles.size()+per_page-1)/per_page);
    std::ofstream out(path,std::ios::binary|std::ios::trunc); if(!out) throw std::runtime_error("cannot create page store");
    FileHeader h{kMagic,1,pages}; out.write(reinterpret_cast<const char*>(&h),sizeof(h));
    const std::uint64_t table_start=sizeof(FileHeader);
    const std::uint64_t data_start=table_start+static_cast<std::uint64_t>(pages)*sizeof(DiskPage);
    std::vector<DiskPage> table; table.reserve(pages);
    std::uint64_t off=data_start;
    for(std::uint32_t p=0;p<pages;++p){
        const std::size_t begin=static_cast<std::size_t>(p)*per_page;
        const std::uint32_t count=static_cast<std::uint32_t>(std::min<std::size_t>(per_page,triangles.size()-begin));
        table.push_back({off,count,0}); off+=static_cast<std::uint64_t>(count)*sizeof(hx::PackedTriangle);
    }
    if(!table.empty()) out.write(reinterpret_cast<const char*>(table.data()),static_cast<std::streamsize>(table.size()*sizeof(DiskPage)));
    if(!triangles.empty()) out.write(reinterpret_cast<const char*>(triangles.data()),static_cast<std::streamsize>(triangles.size()*sizeof(hx::PackedTriangle)));
    if(!out) throw std::runtime_error("failed while writing page store");
}

PagedTriangleStore::PagedTriangleStore(std::string path):path_(std::move(path)) {
    std::ifstream in(path_,std::ios::binary); if(!in) throw std::runtime_error("cannot open page store");
    FileHeader h{}; in.read(reinterpret_cast<char*>(&h),sizeof(h));
    if(!in||h.magic!=kMagic||h.version!=1) throw std::runtime_error("invalid Beamcast GX page store");
    pages_.resize(h.page_count);
    for(std::uint32_t i=0;i<h.page_count;++i){DiskPage d{};in.read(reinterpret_cast<char*>(&d),sizeof(d));pages_[i]={d.offset,d.count};}
    if(!in) throw std::runtime_error("truncated Beamcast GX page table");
}

std::vector<hx::PackedTriangle> PagedTriangleStore::load_page(std::size_t page) const {
    const auto& p=pages_.at(page); std::vector<hx::PackedTriangle> out(p.triangle_count);
    std::ifstream in(path_,std::ios::binary); if(!in) throw std::runtime_error("cannot reopen page store");
    in.seekg(static_cast<std::streamoff>(p.file_offset));
    if(!out.empty()) in.read(reinterpret_cast<char*>(out.data()),static_cast<std::streamsize>(out.size()*sizeof(hx::PackedTriangle)));
    if(!in) throw std::runtime_error("truncated Beamcast GX page payload");
    return out;
}

void PagedTriangleStore::append_page_to_scene(std::size_t page,hx::PackedScene& scene) const {
    for(const auto& t:load_page(page)) scene.add_triangle(t.v0,t.v0+t.e1,t.v0+t.e2,t.material);
}

HybridSplit plan_hybrid_tiles(int total,double cpu,double gpu) {
    total=std::max(0,total); cpu=std::max(0.0,cpu); gpu=std::max(0.0,gpu);
    if(total==0) return {};
    if(cpu+gpu<=0) return {total,0};
    int gpu_tiles=static_cast<int>(std::llround(total*(gpu/(cpu+gpu)))); gpu_tiles=std::clamp(gpu_tiles,0,total);
    return {total-gpu_tiles,gpu_tiles};
}

void TemporalAccumulator::reset(){history_=Framebuffer{};guides_=Guides{};history_length_.clear();}

Framebuffer TemporalAccumulator::accumulate(const Framebuffer& current,const Guides& g,int max_history,Scalar normal_t,Scalar depth_t) {
    max_history=std::clamp(max_history,1,65535); normal_t=std::clamp(normal_t,-1.0f,1.0f); depth_t=std::max(0.0f,depth_t);
    const std::size_t n=current.pixels.size();
    const bool dimensions_match=history_.width==current.width&&history_.height==current.height&&history_.pixels.size()==n;
    if(!dimensions_match){history_=current;guides_=g;history_length_.assign(n,1);return history_;}
    Framebuffer out(current.width,current.height);
    for(std::size_t i=0;i<n;++i){
        bool accept=i<g.normal.size()&&i<guides_.normal.size()&&i<g.depth.size()&&i<guides_.depth.size();
        if(accept){
            const Vec3 cn=g.normal[i],hn=guides_.normal[i];
            if(cn.length_squared()>0&&hn.length_squared()>0) accept=dot(cn,hn)>=normal_t;
            const Scalar cd=g.depth[i],hd=guides_.depth[i];
            if(cd>0&&hd>0){const Scalar denom=std::max(1e-4f,std::max(cd,hd));accept=accept&&std::fabs(cd-hd)/denom<=depth_t;}
        }
        if(accept){
            const int h=std::min<int>(max_history,static_cast<int>(history_length_[i])+1);
            const Scalar a=1/static_cast<Scalar>(h); out.pixels[i]=history_.pixels[i]*(1-a)+current.pixels[i]*a; history_length_[i]=static_cast<std::uint16_t>(h);
        } else {out.pixels[i]=current.pixels[i];history_length_[i]=1;}
    }
    history_=out;guides_=g;return history_;
}

} // namespace beamcast::gx
