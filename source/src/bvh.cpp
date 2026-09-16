#include "beamcast/bvh.hpp"
#include <algorithm>
#include <array>
#include <limits>
#include <numeric>
#include <stdexcept>

namespace beamcast {

BVH::BVH(std::vector<std::shared_ptr<Hittable>> objects) : BVH(std::move(objects), BuildOptions{}) {}

BVH::BVH(std::vector<std::shared_ptr<Hittable>> objects, BuildOptions options)
    : objects_(std::move(objects)), options_(options) {
    options_.leaf_size=std::max(1u,options_.leaf_size);
    options_.sah_bins=std::clamp(options_.sah_bins,4u,32u);
    if(objects_.size()>static_cast<std::size_t>(std::numeric_limits<std::uint32_t>::max()))
        throw std::length_error("Beamcast: BVH supports at most UINT32_MAX objects");
    for(const auto& object:objects_) if(!object) throw std::invalid_argument("Beamcast: BVH cannot contain null objects");
    if(objects_.size()>nodes_.max_size()/2) throw std::length_error("Beamcast: BVH is too large");
    nodes_.reserve(objects_.empty()?0:objects_.size()*2);
    if (!objects_.empty()) build_node(0,static_cast<std::uint32_t>(objects_.size()));
}

std::uint32_t BVH::build_node(std::uint32_t start, std::uint32_t end) {
    const std::uint32_t idx=static_cast<std::uint32_t>(nodes_.size());
    nodes_.push_back(Node{});
    AABB bounds, centroids;
    for (std::uint32_t i=start;i<end;++i) { const AABB b=objects_[i]->bounding_box(); bounds.expand(b); centroids.expand(b.centroid()); }
    const std::uint32_t count=end-start;
    if (count<=options_.leaf_size) { nodes_[idx]={bounds,0,0,start,count,true}; return idx; }

    int best_axis=-1; std::uint32_t best_split=0; Scalar best_cost=std::numeric_limits<Scalar>::infinity();
    const Vec3 ce=centroids.extent();
    struct Bin { AABB box; std::uint32_t count{0}; };
    for (int axis=0;axis<3;++axis) {
        if (ce[axis] < 1e-7f) continue;
        std::vector<Bin> bins(options_.sah_bins);
        for (std::uint32_t i=start;i<end;++i) {
            const AABB b=objects_[i]->bounding_box();
            Scalar t=(b.centroid()[axis]-centroids.min[axis])/ce[axis];
            auto bi=std::min(options_.sah_bins-1, static_cast<std::uint32_t>(t*options_.sah_bins));
            bins[bi].count++; bins[bi].box.expand(b);
        }
        std::vector<AABB> left_box(options_.sah_bins), right_box(options_.sah_bins);
        std::vector<std::uint32_t> left_count(options_.sah_bins), right_count(options_.sah_bins);
        AABB lb, rb; std::uint32_t lc=0, rc=0;
        for (std::uint32_t i=0;i<options_.sah_bins;++i) { lc+=bins[i].count; lb.expand(bins[i].box); left_count[i]=lc; left_box[i]=lb; }
        for (std::uint32_t i=options_.sah_bins;i-->0;) { rc+=bins[i].count; rb.expand(bins[i].box); right_count[i]=rc; right_box[i]=rb; }
        for (std::uint32_t s=0;s+1<options_.sah_bins;++s) {
            if (!left_count[s] || !right_count[s+1]) continue;
            const Scalar cost=left_box[s].surface_area()*left_count[s]+right_box[s+1].surface_area()*right_count[s+1];
            if (cost<best_cost) { best_cost=cost; best_axis=axis; best_split=s; }
        }
    }

    std::uint32_t mid=start;
    if (best_axis>=0) {
        const Scalar cmin=centroids.min[best_axis], ext=centroids.extent()[best_axis];
        auto it=std::partition(objects_.begin()+start, objects_.begin()+end, [&](const std::shared_ptr<Hittable>& o){
            Scalar t=(o->bounding_box().centroid()[best_axis]-cmin)/ext;
            auto bi=std::min(options_.sah_bins-1, static_cast<std::uint32_t>(t*options_.sah_bins));
            return bi<=best_split;
        });
        mid=static_cast<std::uint32_t>(it-objects_.begin());
    }
    if (mid==start || mid==end) {
        int axis=0; if (ce.y>ce.x) axis=1; if (ce.z>ce[axis]) axis=2;
        mid=start+count/2;
        std::nth_element(objects_.begin()+start,objects_.begin()+mid,objects_.begin()+end,[axis](const auto& a,const auto& b){
            return a->bounding_box().centroid()[axis] < b->bounding_box().centroid()[axis];
        });
    }
    const std::uint32_t left=build_node(start,mid), right=build_node(mid,end);
    nodes_[idx]={surrounding_box(nodes_[left].box,nodes_[right].box),left,right,0,0,false};
    return idx;
}

bool BVH::hit(const Ray& ray, Scalar t_min, Scalar t_max, HitRecord& rec) const {
    if (nodes_.empty()) return false;
    struct StackItem { std::uint32_t node; Scalar enter; };
    std::vector<StackItem> stack; stack.reserve(64);
    Scalar root_enter;
    if (!nodes_[0].box.intersect(ray,t_min,t_max,&root_enter,nullptr)) return false;
    stack.push_back({0,root_enter});
    bool any=false; Scalar closest=t_max; HitRecord temp;
    while (!stack.empty()) {
        const StackItem si=stack.back(); stack.pop_back();
        if (si.enter>closest) continue;
        const Node& n=nodes_[si.node];
        if (n.leaf) {
            for (std::uint32_t i=0;i<n.count;++i) if (objects_[n.start+i]->hit(ray,t_min,closest,temp)) { any=true; closest=temp.t; rec=temp; }
            continue;
        }
        Scalar le, re; const bool lh=nodes_[n.left].box.intersect(ray,t_min,closest,&le,nullptr); const bool rh=nodes_[n.right].box.intersect(ray,t_min,closest,&re,nullptr);
        if (lh && rh) {
            if (le<re) { stack.push_back({n.right,re}); stack.push_back({n.left,le}); }
            else { stack.push_back({n.left,le}); stack.push_back({n.right,re}); }
        } else if (lh) stack.push_back({n.left,le}); else if (rh) stack.push_back({n.right,re});
    }
    return any;
}

} // namespace beamcast
