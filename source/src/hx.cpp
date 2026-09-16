#include "beamcast/hx.hpp"

#include <algorithm>
#include <array>
#include <chrono>
#include <exception>
#include <cmath>
#include <thread>
#include <stdexcept>
#include <mutex>

namespace beamcast::hx {
namespace {

constexpr Scalar kEps = 1e-4f;
constexpr Scalar kPi = 3.14159265358979323846f;

inline std::uint32_t expand_bits(std::uint32_t v) {
    v = (v * 0x00010001u) & 0xFF0000FFu;
    v = (v * 0x00000101u) & 0x0F00F00Fu;
    v = (v * 0x00000011u) & 0xC30C30C3u;
    v = (v * 0x00000005u) & 0x49249249u;
    return v;
}

inline std::uint32_t morton3(Point3 p, const AABB& centroid_bounds) {
    const Vec3 e = centroid_bounds.extent();
    auto q = [&](Scalar x, Scalar lo, Scalar ext) {
        if (ext <= 1e-12f) return 0u;
        Scalar t = std::clamp((x-lo)/ext, 0.0f, 0.999999f);
        return static_cast<std::uint32_t>(t * 1024.0f);
    };
    const std::uint32_t x=q(p.x,centroid_bounds.min.x,e.x);
    const std::uint32_t y=q(p.y,centroid_bounds.min.y,e.y);
    const std::uint32_t z=q(p.z,centroid_bounds.min.z,e.z);
    return (expand_bits(x)<<2u) | (expand_bits(y)<<1u) | expand_bits(z);
}

inline bool fast_box_hit(const AABB& b, const Ray& r, const Vec3& inv, Scalar tmin, Scalar tmax, Scalar* enter=nullptr) {
    Scalar tx1=(b.min.x-r.origin.x)*inv.x, tx2=(b.max.x-r.origin.x)*inv.x;
    Scalar lo=std::min(tx1,tx2), hi=std::max(tx1,tx2);
    Scalar ty1=(b.min.y-r.origin.y)*inv.y, ty2=(b.max.y-r.origin.y)*inv.y;
    lo=std::max(lo,std::min(ty1,ty2)); hi=std::min(hi,std::max(ty1,ty2));
    Scalar tz1=(b.min.z-r.origin.z)*inv.z, tz2=(b.max.z-r.origin.z)*inv.z;
    lo=std::max(lo,std::min(tz1,tz2)); hi=std::min(hi,std::max(tz1,tz2));
    lo=std::max(lo,tmin); hi=std::min(hi,tmax);
    if (enter) *enter=lo;
    return hi>=lo;
}

inline Vec3 random_in_unit_sphere(RNG& rng) {
    for (;;) {
        Vec3 p{2*rng.uniform()-1,2*rng.uniform()-1,2*rng.uniform()-1};
        if (p.length_squared()<1) return p;
    }
}

inline Vec3 cosine_hemisphere(const Vec3& n, RNG& rng) {
    Scalar r1=2*kPi*rng.uniform();
    Scalar r2=rng.uniform(), r2s=std::sqrt(r2);
    Vec3 w=n;
    Vec3 a=std::fabs(w.x)>0.1f ? Vec3{0,1,0} : Vec3{1,0,0};
    Vec3 v=unit_vector(cross(w,a));
    Vec3 u=cross(v,w);
    return unit_vector(u*(std::cos(r1)*r2s)+v*(std::sin(r1)*r2s)+w*std::sqrt(1-r2));
}

inline Scalar max_component(const Color& c) { return std::max({c.x,c.y,c.z}); }
inline Scalar luminance(const Color& c) { return 0.2126f*c.x + 0.7152f*c.y + 0.0722f*c.z; }

inline Scalar reflectance(Scalar cosine, Scalar eta) {
    Scalar r0=(1-eta)/(1+eta); r0*=r0;
    return r0+(1-r0)*std::pow(1-cosine,5.0f);
}

} // namespace

std::uint32_t PackedScene::add_material(const PackedMaterial& m) {
    auto finite3=[](const Vec3& v){ return std::isfinite(v.x)&&std::isfinite(v.y)&&std::isfinite(v.z); };
    if (!finite3(m.albedo) || !finite3(m.emission) || !std::isfinite(m.roughness) || !std::isfinite(m.ior))
        throw std::invalid_argument("Beamcast HX: material values must be finite");
    if (m.type==MaterialType::Dielectric && m.ior<=0)
        throw std::invalid_argument("Beamcast HX: dielectric IOR must be positive");
    if (materials_.size() >= static_cast<std::size_t>(std::numeric_limits<std::uint32_t>::max()))
        throw std::length_error("Beamcast HX: too many materials");
    materials_.push_back(m);
    return static_cast<std::uint32_t>(materials_.size()-1);
}
std::uint32_t PackedScene::add_lambertian(Color c) { return add_material({MaterialType::Lambertian,c,{0,0,0},0,1.5f}); }
std::uint32_t PackedScene::add_metal(Color c, Scalar r) { return add_material({MaterialType::Metal,c,{0,0,0},std::clamp(r,0.0f,1.0f),1.5f}); }
std::uint32_t PackedScene::add_dielectric(Scalar ior) { return add_material({MaterialType::Dielectric,{1,1,1},{0,0,0},0,ior}); }
std::uint32_t PackedScene::add_emissive(Color e) { return add_material({MaterialType::Emissive,{0,0,0},e,0,1.0f}); }

void PackedScene::add_sphere(Point3 center, Scalar radius, std::uint32_t material) {
    if (material>=materials_.size()) throw std::out_of_range("Beamcast HX: sphere material index is invalid");
    if (!std::isfinite(radius) || radius<=0) throw std::invalid_argument("Beamcast HX: sphere radius must be positive and finite");
    if (!std::isfinite(center.x)||!std::isfinite(center.y)||!std::isfinite(center.z)) throw std::invalid_argument("Beamcast HX: sphere center must be finite");
    spheres_.push_back({center,radius,material});
    refs_.clear(); nodes_.clear();
}
void PackedScene::add_triangle(Point3 a, Point3 b, Point3 c, std::uint32_t material) {
    if (material>=materials_.size()) throw std::out_of_range("Beamcast HX: triangle material index is invalid");
    auto finite3=[](const Vec3& v){ return std::isfinite(v.x)&&std::isfinite(v.y)&&std::isfinite(v.z); };
    if (!finite3(a)||!finite3(b)||!finite3(c)) throw std::invalid_argument("Beamcast HX: triangle vertices must be finite");
    Vec3 e1=b-a, e2=c-a; const Vec3 n=cross(e1,e2);
    if (n.length_squared()<=1e-20f) throw std::invalid_argument("Beamcast HX: triangle must have non-zero area");
    triangles_.push_back({a,e1,e2,unit_vector(n),material});
    refs_.clear(); nodes_.clear();
}
void PackedScene::clear() { materials_.clear(); spheres_.clear(); triangles_.clear(); refs_.clear(); nodes_.clear(); }

void PackedScene::build() { build(BuildOptions{}); }
void PackedScene::build(BuildOptions options) {
    build_options_=options;
    build_options_.leaf_size=std::clamp(build_options_.leaf_size,1u,16u);
    build_options_.sah_bins=std::clamp(build_options_.sah_bins,8u,64u);
    refs_.clear(); nodes_.clear();
    if(spheres_.size() > static_cast<std::size_t>(std::numeric_limits<std::uint32_t>::max()) - triangles_.size())
        throw std::length_error("Beamcast HX: too many primitives for 32-bit BVH indices");
    refs_.reserve(spheres_.size()+triangles_.size());
    for (std::uint32_t i=0;i<spheres_.size();++i) {
        const auto& s=spheres_[i]; Vec3 r{s.radius,s.radius,s.radius}; AABB b{s.center-r,s.center+r};
        refs_.push_back({b,b.centroid(),i,PrimitiveType::Sphere,0});
    }
    for (std::uint32_t i=0;i<triangles_.size();++i) {
        const auto& t=triangles_[i]; Point3 b=t.v0+t.e1, c=t.v0+t.e2;
        Point3 mn=min_components(t.v0,min_components(b,c)); Point3 mx=max_components(t.v0,max_components(b,c));
        constexpr Scalar e=1e-5f; Vec3 pad{e,e,e}; AABB box{mn-pad,mx+pad};
        refs_.push_back({box,box.centroid(),i,PrimitiveType::Triangle,0});
    }
    if (refs_.empty()) return;
    if(refs_.size()>nodes_.max_size()/2) throw std::length_error("Beamcast HX: BVH is too large");
    nodes_.reserve(refs_.size()*2);
    if (build_options_.mode==BuildMode::Morton) {
        AABB cb; for (const auto& r:refs_) cb.expand(r.centroid);
        for (auto& r:refs_) r.morton=morton3(r.centroid,cb);
        std::sort(refs_.begin(),refs_.end(),[](const auto& a,const auto& b){ return a.morton<b.morton; });
        build_morton(0,static_cast<std::uint32_t>(refs_.size()));
    } else build_sah(0,static_cast<std::uint32_t>(refs_.size()));
}

std::uint32_t PackedScene::build_sah(std::uint32_t begin, std::uint32_t end) {
    std::uint32_t ni=static_cast<std::uint32_t>(nodes_.size()); nodes_.push_back(Node{});
    AABB bounds, cb; for (std::uint32_t i=begin;i<end;++i){ bounds.expand(refs_[i].box); cb.expand(refs_[i].centroid); }
    std::uint32_t count=end-begin;
    if (count<=build_options_.leaf_size) { nodes_[ni].box=bounds; nodes_[ni].first=begin; nodes_[ni].count=count; nodes_[ni].leaf=1; return ni; }
    struct Bin { AABB b; std::uint32_t n{0}; };
    int best_axis=-1; std::uint32_t best_split=0; Scalar best_cost=std::numeric_limits<Scalar>::infinity();
    const Vec3 ext=cb.extent(); const Scalar parent_area=std::max(bounds.surface_area(),1e-12f);
    for (int axis=0;axis<3;++axis) {
        if (ext[axis]<=1e-8f) continue;
        std::vector<Bin> bins(build_options_.sah_bins);
        for (std::uint32_t i=begin;i<end;++i) {
            Scalar p=(refs_[i].centroid[axis]-cb.min[axis])/ext[axis];
            std::uint32_t bi=std::min(build_options_.sah_bins-1,static_cast<std::uint32_t>(p*build_options_.sah_bins));
            bins[bi].n++; bins[bi].b.expand(refs_[i].box);
        }
        std::vector<AABB> lb(build_options_.sah_bins), rb(build_options_.sah_bins);
        std::vector<std::uint32_t> ln(build_options_.sah_bins), rn(build_options_.sah_bins);
        AABB l,r; std::uint32_t nl=0,nr=0;
        for(std::uint32_t i=0;i<build_options_.sah_bins;++i){ nl+=bins[i].n; l.expand(bins[i].b); ln[i]=nl; lb[i]=l; }
        for(std::uint32_t i=build_options_.sah_bins;i-->0;){ nr+=bins[i].n; r.expand(bins[i].b); rn[i]=nr; rb[i]=r; }
        for(std::uint32_t i=0;i+1<build_options_.sah_bins;++i) {
            if(!ln[i]||!rn[i+1]) continue;
            Scalar cost=(lb[i].surface_area()*ln[i]+rb[i+1].surface_area()*rn[i+1])/parent_area;
            if(cost<best_cost){ best_cost=cost; best_axis=axis; best_split=i; }
        }
    }
    std::uint32_t mid=begin;
    if(best_axis>=0) {
        Scalar lo=cb.min[best_axis], ex=ext[best_axis];
        auto it=std::partition(refs_.begin()+begin,refs_.begin()+end,[&](const PrimitiveRef& r){
            Scalar p=(r.centroid[best_axis]-lo)/ex;
            std::uint32_t bi=std::min(build_options_.sah_bins-1,static_cast<std::uint32_t>(p*build_options_.sah_bins));
            return bi<=best_split;
        });
        mid=static_cast<std::uint32_t>(it-refs_.begin());
    }
    if(mid==begin||mid==end) {
        int axis=0; if(ext.y>ext.x) axis=1; if(ext.z>ext[axis]) axis=2;
        mid=begin+count/2;
        std::nth_element(refs_.begin()+begin,refs_.begin()+mid,refs_.begin()+end,[&](const auto&a,const auto&b){return a.centroid[axis]<b.centroid[axis];});
        best_axis=axis;
    }
    std::uint32_t l=build_sah(begin,mid), r=build_sah(mid,end);
    nodes_[ni].box=surrounding_box(nodes_[l].box,nodes_[r].box); nodes_[ni].left=l; nodes_[ni].right=r; nodes_[ni].axis=static_cast<std::uint32_t>(std::max(0,best_axis));
    return ni;
}

std::uint32_t PackedScene::build_morton(std::uint32_t begin, std::uint32_t end) {
    std::uint32_t ni=static_cast<std::uint32_t>(nodes_.size()); nodes_.push_back(Node{});
    AABB bounds; for(std::uint32_t i=begin;i<end;++i) bounds.expand(refs_[i].box);
    std::uint32_t count=end-begin;
    if(count<=build_options_.leaf_size){ nodes_[ni].box=bounds; nodes_[ni].first=begin; nodes_[ni].count=count; nodes_[ni].leaf=1; return ni; }
    std::uint32_t mid=begin+count/2;
    std::uint32_t first=refs_[begin].morton,last=refs_[end-1].morton;
    if(first!=last) {
#if defined(__GNUC__) || defined(__clang__)
        int bit=31-__builtin_clz(first^last);
#else
        int bit=31; while(bit>0 && (((first^last)>>bit)&1u)==0u) --bit;
#endif
        std::uint32_t mask=1u<<bit;
        auto it=std::partition_point(refs_.begin()+begin,refs_.begin()+end,[&](const PrimitiveRef& r){ return (r.morton&mask)==0; });
        std::uint32_t candidate=static_cast<std::uint32_t>(it-refs_.begin());
        if(candidate>begin&&candidate<end) mid=candidate;
    }
    std::uint32_t l=build_morton(begin,mid), r=build_morton(mid,end);
    nodes_[ni].box=surrounding_box(nodes_[l].box,nodes_[r].box); nodes_[ni].left=l; nodes_[ni].right=r;
    return ni;
}

bool PackedScene::hit_primitive(const PrimitiveRef& pr, const Ray& ray, Scalar t_min, Scalar t_max, PackedHit& out, TraceStats* st) const {
    if(st) st->primitive_tests++;
    if(pr.type==PrimitiveType::Sphere) {
        const auto& s=spheres_[pr.index]; Vec3 oc=ray.origin-s.center;
        Scalar a=ray.direction.length_squared(); if(!std::isfinite(a)||a<=1e-20f) return false;
        Scalar hb=dot(oc,ray.direction), c=oc.length_squared()-s.radius*s.radius;
        Scalar d=hb*hb-a*c; if(d<0) return false; Scalar root=(-hb-std::sqrt(d))/a;
        if(root<=t_min||root>=t_max){ root=(-hb+std::sqrt(d))/a; if(root<=t_min||root>=t_max) return false; }
        out.t=root; out.p=ray.at(root); Vec3 n=(out.p-s.center)/s.radius; out.front_face=dot(ray.direction,n)<0; out.normal=out.front_face?n:-n; out.material=s.material; out.primitive_index=pr.index; out.primitive_type=0; return true;
    }
    const auto& t=triangles_[pr.index]; Vec3 p=cross(ray.direction,t.e2); Scalar det=dot(t.e1,p);
    if(std::fabs(det)<1e-8f) return false;
    Scalar inv=1/det;
    Vec3 s=ray.origin-t.v0;
    Scalar u=dot(s,p)*inv;
    if(u<0||u>1) return false;
    Vec3 q=cross(s,t.e1); Scalar v=dot(ray.direction,q)*inv; if(v<0||u+v>1) return false; Scalar tt=dot(t.e2,q)*inv; if(tt<=t_min||tt>=t_max) return false;
    out.t=tt; out.p=ray.at(tt); out.front_face=dot(ray.direction,t.normal)<0; out.normal=out.front_face?t.normal:-t.normal; out.material=t.material; out.primitive_index=pr.index; out.primitive_type=1; return true;
}

bool PackedScene::hit(const Ray& ray, Scalar t_min, Scalar t_max, PackedHit& out, TraceStats* st) const {
    if(nodes_.empty()) return false;
    if(st) st->rays++;
    Vec3 inv{
        std::fabs(ray.direction.x)>1e-20f?1/ray.direction.x:std::copysign(1e30f,ray.direction.x),
        std::fabs(ray.direction.y)>1e-20f?1/ray.direction.y:std::copysign(1e30f,ray.direction.y),
        std::fabs(ray.direction.z)>1e-20f?1/ray.direction.z:std::copysign(1e30f,ray.direction.z)
    };
    struct Item { std::uint32_t node; Scalar enter; };
    std::array<Item,128> local_stack{}; int sp=0; std::vector<Item> overflow_stack; Scalar e;
    auto push=[&](Item item){ if(sp<static_cast<int>(local_stack.size()) && overflow_stack.empty()) local_stack[sp++]=item; else overflow_stack.push_back(item); };
    auto empty=[&](){ return sp==0 && overflow_stack.empty(); };
    auto pop=[&](){ if(!overflow_stack.empty()){ Item item=overflow_stack.back(); overflow_stack.pop_back(); return item; } return local_stack[--sp]; };
    if(st) st->box_tests++;
    if(!fast_box_hit(nodes_[0].box,ray,inv,t_min,t_max,&e)) return false;
    push({0,e});
    bool any=false; Scalar closest=t_max; PackedHit temp;
    while(!empty()) {
        Item it=pop(); if(it.enter>closest) continue; const Node& n=nodes_[it.node];
        if(n.leaf) {
            for(std::uint32_t i=0;i<n.count;++i) if(hit_primitive(refs_[n.first+i],ray,t_min,closest,temp,st)){ any=true; closest=temp.t; out=temp; }
            continue;
        }
        Scalar le=0,re=0; if(st) st->box_tests+=2;
        bool lh=fast_box_hit(nodes_[n.left].box,ray,inv,t_min,closest,&le), rh=fast_box_hit(nodes_[n.right].box,ray,inv,t_min,closest,&re);
        if(lh&&rh){ if(le<re){ push({n.right,re}); push({n.left,le}); } else { push({n.left,le}); push({n.right,re}); } }
        else if(lh){ push({n.left,le}); } else if(rh){ push({n.right,re}); }
    }
    if(any&&st) st->hits++;
    return any;
}

GpuSceneData PackedScene::export_gpu() const {
    GpuSceneData out;
    out.nodes.reserve(nodes_.size());
    for (const Node& n : nodes_) {
        out.nodes.push_back({n.box.min, n.box.max, n.left, n.right, n.first, n.count, n.leaf, n.axis});
    }
    out.refs.reserve(refs_.size());
    for (const PrimitiveRef& r : refs_) {
        out.refs.push_back({r.index, r.type == PrimitiveType::Sphere ? 0u : 1u});
    }
    out.spheres = spheres_;
    out.triangles = triangles_;
    out.materials = materials_;
    return out;
}

QuantizedGpuSceneData PackedScene::export_gpu_quantized() const {
    QuantizedGpuSceneData out;
    if(nodes_.empty()) {
        out.spheres=spheres_; out.triangles=triangles_; out.materials=materials_;
        return out;
    }
    out.world_min=nodes_.front().box.min;
    out.world_extent=nodes_.front().box.extent();
    for(int a=0;a<3;++a) if(out.world_extent[a]<1e-12f) out.world_extent[a]=1.0f;
    auto qmin=[&](Scalar x,int a){
        const Scalar t=std::clamp((x-out.world_min[a])/out.world_extent[a],0.0f,1.0f);
        return static_cast<std::uint16_t>(std::floor(t*65535.0f));
    };
    auto qmax=[&](Scalar x,int a){
        const Scalar t=std::clamp((x-out.world_min[a])/out.world_extent[a],0.0f,1.0f);
        return static_cast<std::uint16_t>(std::ceil(t*65535.0f));
    };
    out.nodes.reserve(nodes_.size());
    for(const Node& n:nodes_) {
        QuantizedGpuBvhNode q{};
        for(int a=0;a<3;++a){q.bmin[a]=qmin(n.box.min[a],a);q.bmax[a]=qmax(n.box.max[a],a);}
        q.left=n.left;q.right=n.right;q.first=n.first;q.count=n.count;q.leaf=n.leaf;q.axis=n.axis;
        out.nodes.push_back(q);
    }
    out.refs.reserve(refs_.size());
    for(const PrimitiveRef& r:refs_) out.refs.push_back({r.index,r.type==PrimitiveType::Sphere?0u:1u});
    out.spheres=spheres_; out.triangles=triangles_; out.materials=materials_;
    return out;
}

Color trace(Ray ray, const PackedScene& scene, const RenderSettings& s, RNG& rng, TraceStats* st) {
    Color L{0,0,0}, beta{1,1,1};
    for(int bounce=0;bounce<s.max_bounces;++bounce) {
        PackedHit h;
        if(!scene.hit(ray,kEps,std::numeric_limits<Scalar>::infinity(),h,st)) {
            Vec3 d=unit_vector(ray.direction); Scalar t=0.5f*(d.y+1); L+=beta*((1-t)*s.background_bottom+t*s.background_top); break;
        }
        const auto& m=scene.material(h.material);
        if(m.type==MaterialType::Emissive){ L+=beta*m.emission; break; }
        Vec3 dir; Color att=m.albedo;
        if(m.type==MaterialType::Lambertian) dir=cosine_hemisphere(h.normal,rng);
        else if(m.type==MaterialType::Metal) {
            dir=reflect(unit_vector(ray.direction),h.normal)+m.roughness*random_in_unit_sphere(rng); if(dot(dir,h.normal)<=0) break;
        } else {
            att={1,1,1}; Scalar eta=h.front_face?(1/m.ior):m.ior; Vec3 u=unit_vector(ray.direction); Scalar ct=std::min(dot(-u,h.normal),1.0f); Scalar stheta=std::sqrt(std::max(0.0f,1-ct*ct));
            bool cannot=eta*stheta>1; dir=(cannot||reflectance(ct,eta)>rng.uniform())?reflect(u,h.normal):refract(u,h.normal,eta);
        }
        beta=beta*att;
        if(s.russian_roulette&&bounce>=4){ Scalar p=std::clamp(max_component(beta),0.05f,0.98f); if(rng.uniform()>p) break; beta/=p; }
        const Vec3 off = dot(dir,h.normal)>=0 ? h.normal*kEps : -h.normal*kEps;
        ray={h.p+off,dir};
    }
    return L;
}

Framebuffer render(const PackedScene& scene, const Camera& camera, const RenderSettings& s, RenderReport* report, ProgressCallback progress) {
    if(!scene.built()) throw std::logic_error("Beamcast HX: scene geometry changed; call scene.build() before rendering");
    if(s.width<0||s.height<0) throw std::invalid_argument("Beamcast HX: render dimensions must be non-negative");
    if(s.min_samples<=0||s.max_samples<=0||s.min_samples>s.max_samples) throw std::invalid_argument("Beamcast HX: sample counts must satisfy 0 < min_samples <= max_samples");
    if(s.max_bounces<=0) throw std::invalid_argument("Beamcast HX: max_bounces must be positive");
    if(!std::isfinite(s.adaptive_threshold)||s.adaptive_threshold<0) throw std::invalid_argument("Beamcast HX: adaptive_threshold must be non-negative and finite");
    Framebuffer fb(s.width,s.height); if(s.width==0||s.height==0) return fb;
    auto start=std::chrono::steady_clock::now();
    int tile=std::max(4,s.tile_size), nx=(s.width+tile-1)/tile, ny=(s.height+tile-1)/tile, total=nx*ny;
    std::atomic<int> next{0},done{0}; std::atomic<bool> stop{false};
    std::atomic<std::uint64_t> samples{0},rays{0},boxes{0},prims{0},hits{0};
    std::mutex progress_mutex,error_mutex; std::exception_ptr error;
    int tc=s.thread_count>0?s.thread_count:static_cast<int>(std::thread::hardware_concurrency()); tc=std::max(1,tc);
    auto worker=[&](int tid){
        TraceStats local{};
        try {
            for(;;){
                if(stop.load(std::memory_order_relaxed)) break;
                int id=next.fetch_add(1,std::memory_order_relaxed); if(id>=total) break;
                int bx=(id%nx)*tile,by=(id/nx)*tile,ex=std::min(bx+tile,s.width),ey=std::min(by+tile,s.height);
                for(int y=by;y<ey;++y) for(int x=bx;x<ex;++x){
                    Color mean{0,0,0}; Scalar mean_l=0,m2=0; int n=0;
                    const std::size_t pixel=static_cast<std::size_t>(y)*static_cast<std::size_t>(s.width)+static_cast<std::size_t>(x);
                    RNG rng(hash_seed(s.seed^static_cast<std::uint64_t>(tid+1),static_cast<std::uint64_t>(pixel)));
                    for(;n<s.max_samples;++n){
                        Scalar u=(x+rng.uniform())/std::max(1,s.width-1), v=(y+rng.uniform())/std::max(1,s.height-1);
                        Color c=trace(camera.ray(u,v,rng),scene,s,rng,&local); mean+=(c-mean)/static_cast<Scalar>(n+1);
                        Scalar l=luminance(c),delta=l-mean_l; mean_l+=delta/static_cast<Scalar>(n+1); m2+=delta*(l-mean_l);
                        if(s.adaptive_sampling&&n+1>=s.min_samples&&n+1>=4){ Scalar var=m2/static_cast<Scalar>(n); Scalar se=std::sqrt(std::max(0.0f,var)/static_cast<Scalar>(n+1)); if(se<=s.adaptive_threshold*std::max(0.05f,std::fabs(mean_l))) { ++n; break; } }
                    }
                    fb.at(x,y)=mean; samples.fetch_add(static_cast<std::uint64_t>(n),std::memory_order_relaxed);
                }
                const int completed=done.fetch_add(1,std::memory_order_relaxed)+1;
                if(progress) { std::lock_guard<std::mutex> lock(progress_mutex); progress(completed,total); }
            }
        } catch (...) {
            { std::lock_guard<std::mutex> lock(error_mutex); if(!error) error=std::current_exception(); }
            stop.store(true,std::memory_order_relaxed);
        }
        rays.fetch_add(local.rays,std::memory_order_relaxed); boxes.fetch_add(local.box_tests,std::memory_order_relaxed); prims.fetch_add(local.primitive_tests,std::memory_order_relaxed); hits.fetch_add(local.hits,std::memory_order_relaxed);
    };
    std::vector<std::thread> pool; pool.reserve(tc); for(int i=0;i<tc;++i) pool.emplace_back(worker,i); for(auto& t:pool)t.join();
    if(error) std::rethrow_exception(error);
    double sec=std::chrono::duration<double>(std::chrono::steady_clock::now()-start).count();
    if(report){ report->camera_samples=samples.load(); report->path_rays=rays.load(); report->box_tests=boxes.load(); report->primitive_tests=prims.load(); report->hits=hits.load(); report->seconds=sec; report->primary_samples_per_second=sec>0?report->camera_samples/sec:0; report->rays_per_second=sec>0?report->path_rays/sec:0; }
    return fb;
}

} // namespace beamcast::hx
