#include "beamcast/gx.hpp"

#include <algorithm>
#include <array>
#include <atomic>
#include <chrono>
#include <cmath>
#include <condition_variable>
#include <cstddef>
#include <cstdint>
#include <functional>
#include <limits>
#include <mutex>
#include <numeric>
#include <stdexcept>
#include <thread>
#include <utility>
#include <vector>

namespace beamcast::gx {
namespace {

constexpr Scalar kPi = 3.14159265358979323846f;
constexpr Scalar kEps = 1.0e-4f;

inline Scalar luminance(const Color& c) {
    return 0.2126f*c.x + 0.7152f*c.y + 0.0722f*c.z;
}
inline Scalar max_component(const Color& c) { return std::max({c.x,c.y,c.z}); }
inline Scalar power_heuristic(Scalar a, Scalar b) {
    const Scalar aa=a*a, bb=b*b;
    return aa/(aa+bb+1.0e-20f);
}

inline std::uint64_t next_u64(std::uint64_t& state) {
    std::uint64_t x=state ? state : 1ULL;
    x^=x>>12; x^=x<<25; x^=x>>27;
    state=x;
    return x*2685821657736338717ULL;
}
inline Scalar uniform(std::uint64_t& state) {
    const std::uint32_t v=static_cast<std::uint32_t>(next_u64(state)>>40);
    return static_cast<Scalar>(v)/static_cast<Scalar>(1u<<24);
}
inline Vec3 random_in_unit_sphere(std::uint64_t& s) {
    for(;;) {
        Vec3 p{2*uniform(s)-1,2*uniform(s)-1,2*uniform(s)-1};
        if(p.length_squared()<1) return p;
    }
}
inline Vec3 random_unit_vector(std::uint64_t& s) {
    const Scalar z=1-2*uniform(s);
    const Scalar a=2*kPi*uniform(s);
    const Scalar r=std::sqrt(std::max(Scalar(0),1-z*z));
    return {r*std::cos(a),r*std::sin(a),z};
}
inline Vec3 cosine_hemisphere(const Vec3& n, std::uint64_t& state) {
    const Scalar r1=2*kPi*uniform(state);
    const Scalar r2=uniform(state), r2s=std::sqrt(r2);
    const Vec3 w=n;
    const Vec3 a=std::fabs(w.x)>0.1f?Vec3{0,1,0}:Vec3{1,0,0};
    const Vec3 v=unit_vector(cross(w,a));
    const Vec3 u=cross(v,w);
    return unit_vector(u*(std::cos(r1)*r2s)+v*(std::sin(r1)*r2s)+w*std::sqrt(std::max(Scalar(0),1-r2)));
}
inline Scalar schlick(Scalar cosine, Scalar eta) {
    Scalar r0=(1-eta)/(1+eta); r0*=r0;
    return r0+(1-r0)*std::pow(1-cosine,5.0f);
}

class ParallelExecutor {
    using Job = std::function<void(int,std::size_t,std::size_t)>;
    std::vector<std::thread> workers_;
    std::mutex mutex_;
    std::condition_variable start_cv_;
    std::condition_variable done_cv_;
    std::atomic<std::size_t> next_{0};
    std::size_t count_{0};
    std::size_t chunk_{1};
    const Job* job_{nullptr};
    std::uint64_t generation_{0};
    int pending_{0};
    bool stop_{false};

    void worker(int tid) {
        std::uint64_t seen=0;
        for(;;) {
            const Job* job=nullptr;
            std::size_t count=0,chunk=1;
            {
                std::unique_lock<std::mutex> lock(mutex_);
                start_cv_.wait(lock,[&]{ return stop_||generation_!=seen; });
                if(stop_) return;
                seen=generation_; job=job_; count=count_; chunk=chunk_;
            }
            for(;;) {
                const std::size_t begin=next_.fetch_add(chunk,std::memory_order_relaxed);
                if(begin>=count) break;
                (*job)(tid,begin,std::min(count,begin+chunk));
            }
            {
                std::lock_guard<std::mutex> lock(mutex_);
                if(--pending_==0) done_cv_.notify_one();
            }
        }
    }
public:
    explicit ParallelExecutor(int threads) {
        threads=std::max(1,threads);
        workers_.reserve(static_cast<std::size_t>(threads));
        for(int i=0;i<threads;++i) workers_.emplace_back([this,i]{worker(i);});
    }
    ~ParallelExecutor() {
        {
            std::lock_guard<std::mutex> lock(mutex_); stop_=true; ++generation_;
        }
        start_cv_.notify_all();
        for(auto& t:workers_) if(t.joinable()) t.join();
    }
    int size() const { return static_cast<int>(workers_.size()); }
    void run(std::size_t count, std::size_t chunk, const Job& job) {
        if(count==0) return;
        {
            std::lock_guard<std::mutex> lock(mutex_);
            count_=count; chunk_=std::max<std::size_t>(1,chunk); job_=&job; next_.store(0,std::memory_order_relaxed);
            pending_=static_cast<int>(workers_.size()); ++generation_;
        }
        start_cv_.notify_all();
        std::unique_lock<std::mutex> lock(mutex_);
        done_cv_.wait(lock,[&]{return pending_==0;});
        job_=nullptr;
    }
};

struct PathQueue {
    std::vector<Point3> origin;
    std::vector<Vec3> direction;
    std::vector<Color> beta;
    std::vector<Color> radiance;
    std::vector<Point3> previous_point;
    std::vector<Scalar> previous_bsdf_pdf;
    std::vector<std::uint32_t> pixel;
    std::vector<std::uint64_t> rng;
    std::vector<std::uint8_t> previous_delta;

    std::size_t size() const { return pixel.size(); }
    bool empty() const { return pixel.empty(); }
    void clear() {
        origin.clear();direction.clear();beta.clear();radiance.clear();previous_point.clear();
        previous_bsdf_pdf.clear();pixel.clear();rng.clear();previous_delta.clear();
    }
    void reserve(std::size_t n) {
        origin.reserve(n);direction.reserve(n);beta.reserve(n);radiance.reserve(n);previous_point.reserve(n);
        previous_bsdf_pdf.reserve(n);pixel.reserve(n);rng.reserve(n);previous_delta.reserve(n);
    }
    void push(const Ray& r, Color b, Color L, Point3 prev, Scalar prev_pdf,
              std::uint32_t px, std::uint64_t rs, bool prev_delta) {
        origin.push_back(r.origin); direction.push_back(r.direction); beta.push_back(b); radiance.push_back(L);
        previous_point.push_back(prev); previous_bsdf_pdf.push_back(prev_pdf); pixel.push_back(px); rng.push_back(rs);
        previous_delta.push_back(prev_delta?1u:0u);
    }
    void append(PathQueue&& q) {
        const std::size_t old=size(), add=q.size();
        if(add==0) return;
        reserve(old+add);
        origin.insert(origin.end(),q.origin.begin(),q.origin.end());
        direction.insert(direction.end(),q.direction.begin(),q.direction.end());
        beta.insert(beta.end(),q.beta.begin(),q.beta.end());
        radiance.insert(radiance.end(),q.radiance.begin(),q.radiance.end());
        previous_point.insert(previous_point.end(),q.previous_point.begin(),q.previous_point.end());
        previous_bsdf_pdf.insert(previous_bsdf_pdf.end(),q.previous_bsdf_pdf.begin(),q.previous_bsdf_pdf.end());
        pixel.insert(pixel.end(),q.pixel.begin(),q.pixel.end());
        rng.insert(rng.end(),q.rng.begin(),q.rng.end());
        previous_delta.insert(previous_delta.end(),q.previous_delta.begin(),q.previous_delta.end());
    }
};


void bucket_by_direction(PathQueue& q) {
    if(q.size()<2) return;
    std::array<PathQueue,8> buckets;
    for(auto& b:buckets) b.reserve(q.size()/8+8);
    for(std::size_t i=0;i<q.size();++i) {
        const Vec3& d=q.direction[i];
        const unsigned key=(d.x<0?1u:0u)|(d.y<0?2u:0u)|(d.z<0?4u:0u);
        buckets[key].push({q.origin[i],q.direction[i]},q.beta[i],q.radiance[i],q.previous_point[i],
                          q.previous_bsdf_pdf[i],q.pixel[i],q.rng[i],q.previous_delta[i]!=0);
    }
    q.clear();
    for(auto& b:buckets) q.append(std::move(b));
}

int pixel_sample_budget(const Settings& s,int x,int y) {
    if(!s.variable_rate_sampling) return s.samples_per_pixel;
    const int min_samples=std::clamp(s.minimum_pixel_samples,1,s.samples_per_pixel);
    const Scalar fx=s.width>1?static_cast<Scalar>(x)/static_cast<Scalar>(s.width-1):0.5f;
    const Scalar fy=s.height>1?static_cast<Scalar>(y)/static_cast<Scalar>(s.height-1):0.5f;
    const Scalar dx=fx-std::clamp(s.foveation_center_x,0.0f,1.0f);
    const Scalar dy=fy-std::clamp(s.foveation_center_y,0.0f,1.0f);
    const Scalar maxr=0.70710678118f;
    const Scalar r=std::clamp(std::sqrt(dx*dx+dy*dy)/maxr,0.0f,1.0f);
    const Scalar strength=std::clamp(s.foveation_strength,0.0f,0.95f);
    const Scalar quality=1.0f-strength*r*r;
    return std::clamp(static_cast<int>(std::lround(min_samples+(s.samples_per_pixel-min_samples)*quality)),min_samples,s.samples_per_pixel);
}

struct LightRef {
    std::uint32_t type{0};
    std::uint32_t index{0};
    std::uint32_t material{0};
    Scalar area{0};
    Scalar selection_pdf{0};
};

class LightAliasTable {
    const hx::PackedScene* scene_{nullptr};
    std::vector<LightRef> lights_;
    std::vector<Scalar> probability_;
    std::vector<std::uint32_t> alias_;
    std::vector<Scalar> sphere_pdf_;
    std::vector<Scalar> triangle_pdf_;
public:
    explicit LightAliasTable(const hx::PackedScene& scene):scene_(&scene) {
        const auto& mats=scene.materials();
        Scalar total=0;
        std::vector<Scalar> weights;
        sphere_pdf_.assign(scene.spheres().size(),0);
        triangle_pdf_.assign(scene.triangles().size(),0);
        for(std::uint32_t i=0;i<scene.spheres().size();++i) {
            const auto& s=scene.spheres()[i];
            if(s.material>=mats.size()||mats[s.material].type!=hx::MaterialType::Emissive) continue;
            const Scalar area=4*kPi*s.radius*s.radius;
            const Scalar w=area*std::max(Scalar(0),luminance(mats[s.material].emission));
            if(w<=0) continue;
            lights_.push_back({0,i,s.material,area,0}); weights.push_back(w); total+=w;
        }
        for(std::uint32_t i=0;i<scene.triangles().size();++i) {
            const auto& t=scene.triangles()[i];
            if(t.material>=mats.size()||mats[t.material].type!=hx::MaterialType::Emissive) continue;
            const Scalar area=0.5f*cross(t.e1,t.e2).length();
            const Scalar w=area*std::max(Scalar(0),luminance(mats[t.material].emission));
            if(w<=0) continue;
            lights_.push_back({1,i,t.material,area,0}); weights.push_back(w); total+=w;
        }
        if(lights_.empty()||total<=0) return;
        const std::size_t n=lights_.size();
        probability_.assign(n,1); alias_.resize(n);
        std::vector<Scalar> scaled(n);
        std::vector<std::uint32_t> small,large;
        small.reserve(n); large.reserve(n);
        for(std::size_t i=0;i<n;++i) {
            lights_[i].selection_pdf=weights[i]/total;
            scaled[i]=lights_[i].selection_pdf*static_cast<Scalar>(n);
            (scaled[i]<1?small:large).push_back(static_cast<std::uint32_t>(i));
            if(lights_[i].type==0) sphere_pdf_[lights_[i].index]=lights_[i].selection_pdf;
            else triangle_pdf_[lights_[i].index]=lights_[i].selection_pdf;
        }
        while(!small.empty()&&!large.empty()) {
            const auto s=small.back();small.pop_back();
            const auto l=large.back();large.pop_back();
            probability_[s]=scaled[s]; alias_[s]=l;
            scaled[l]=(scaled[l]+scaled[s])-1;
            (scaled[l]<1?small:large).push_back(l);
        }
        for(auto i:large){probability_[i]=1;alias_[i]=i;}
        for(auto i:small){probability_[i]=1;alias_[i]=i;}
    }
    bool empty() const { return lights_.empty(); }
    std::size_t size() const { return lights_.size(); }
    const LightRef& sample(std::uint64_t& rng) const {
        const Scalar u=uniform(rng)*static_cast<Scalar>(lights_.size());
        const std::uint32_t col=std::min<std::uint32_t>(static_cast<std::uint32_t>(u),static_cast<std::uint32_t>(lights_.size()-1));
        const Scalar frac=u-static_cast<Scalar>(col);
        return lights_[frac<probability_[col]?col:alias_[col]];
    }
    Scalar primitive_selection_pdf(std::uint32_t type,std::uint32_t index) const {
        if(type==0) return index<sphere_pdf_.size()?sphere_pdf_[index]:0;
        return index<triangle_pdf_.size()?triangle_pdf_[index]:0;
    }
    Scalar primitive_area(std::uint32_t type,std::uint32_t index) const {
        if(type==0 && index<scene_->spheres().size()) {
            const auto& s=scene_->spheres()[index]; return 4*kPi*s.radius*s.radius;
        }
        if(type==1 && index<scene_->triangles().size()) {
            const auto& t=scene_->triangles()[index]; return 0.5f*cross(t.e1,t.e2).length();
        }
        return 0;
    }
    Vec3 primitive_geometric_normal(std::uint32_t type,std::uint32_t index,const Point3& p) const {
        if(type==0 && index<scene_->spheres().size()) return unit_vector(p-scene_->spheres()[index].center);
        if(type==1 && index<scene_->triangles().size()) return scene_->triangles()[index].normal;
        return {};
    }
};

struct SampledLight {
    Point3 position{};
    Vec3 normal{};
    Color emission{};
    Scalar area{0};
    Scalar selection_pdf{0};
    std::uint32_t type{0};
    std::uint32_t index{0};
};

SampledLight sample_light(const hx::PackedScene& scene,const LightAliasTable& table,std::uint64_t& rng) {
    const LightRef& l=table.sample(rng);
    SampledLight s; s.area=l.area;s.selection_pdf=l.selection_pdf;s.type=l.type;s.index=l.index;
    s.emission=scene.materials()[l.material].emission;
    if(l.type==0) {
        const auto& sp=scene.spheres()[l.index];
        s.normal=random_unit_vector(rng); s.position=sp.center+sp.radius*s.normal;
    } else {
        const auto& t=scene.triangles()[l.index];
        const Scalar u=uniform(rng),v=uniform(rng); const Scalar su=std::sqrt(u);
        const Scalar b1=su*(1-v),b2=su*v;
        s.position=t.v0+b1*t.e1+b2*t.e2; s.normal=t.normal;
    }
    return s;
}

struct DirectCandidate {
    SampledLight light{};
    Vec3 wi{};
    Scalar distance{0};
    Scalar cos_surface{0};
    Scalar cos_light{0};
    Scalar target{0};
    Scalar proposal_area{0};
    Scalar bsdf_pdf{0};
    Color f_area{};
};

DirectCandidate evaluate_candidate(const hx::PackedScene& scene,const LightAliasTable& table,
                                   const Point3& p,const Vec3& n,const Color& albedo,std::uint64_t& rng) {
    DirectCandidate c; if(table.empty()) return c;
    c.light=sample_light(scene,table,rng);
    Vec3 d=c.light.position-p; const Scalar dist2=d.length_squared();
    if(dist2<=1e-12f) return c;
    c.distance=std::sqrt(dist2); c.wi=d/c.distance;
    c.cos_surface=std::max(Scalar(0),dot(n,c.wi));
    c.cos_light=std::fabs(dot(c.light.normal,-c.wi));
    if(c.cos_surface<=0||c.cos_light<=1e-7f||c.light.area<=0||c.light.selection_pdf<=0) return c;
    const Color brdf=albedo*(1/kPi);
    c.f_area=brdf*c.light.emission*(c.cos_surface*c.cos_light/dist2);
    c.target=std::max(Scalar(0),luminance(c.f_area));
    c.proposal_area=c.light.selection_pdf/c.light.area;
    c.bsdf_pdf=c.cos_surface/kPi;
    return c;
}

bool visible_to_light(const hx::PackedScene& scene,const Point3& p,const Vec3& n,const DirectCandidate& c,
                      hx::TraceStats& stats,std::uint64_t& shadow_rays) {
    if(c.distance<=2*kEps) return false;
    const Point3 o=p+n*kEps;
    hx::PackedHit h; ++shadow_rays;
    return !scene.hit({o,c.wi},kEps,c.distance-2*kEps,h,&stats);
}

Color direct_alias_mis(const hx::PackedScene& scene,const LightAliasTable& table,const Point3& p,const Vec3& n,
                       const Color& albedo,Color beta,std::uint64_t& rng,hx::TraceStats& stats,
                       std::uint64_t& shadow_rays,std::uint64_t& candidates) {
    if(table.empty()) return {};
    ++candidates;
    const DirectCandidate c=evaluate_candidate(scene,table,p,n,albedo,rng);
    if(c.target<=0||c.proposal_area<=0||c.cos_light<=0) return {};
    const Scalar pdf_omega=c.proposal_area*(c.distance*c.distance)/c.cos_light;
    if(pdf_omega<=1e-20f||!visible_to_light(scene,p,n,c,stats,shadow_rays)) return {};
    const Scalar w=power_heuristic(pdf_omega,c.bsdf_pdf);
    const Color f=albedo*(1/kPi);
    return beta*f*c.light.emission*(c.cos_surface/pdf_omega)*w;
}

Color direct_reservoir_ris(const hx::PackedScene& scene,const LightAliasTable& table,const Point3& p,const Vec3& n,
                           const Color& albedo,Color beta,int candidate_count,std::uint64_t& rng,hx::TraceStats& stats,
                           std::uint64_t& shadow_rays,std::uint64_t& candidates) {
    if(table.empty()) return {};
    DirectCandidate selected{}; Scalar weight_sum=0; int M=0;
    candidate_count=std::max(1,candidate_count);
    for(int i=0;i<candidate_count;++i) {
        ++candidates; ++M;
        DirectCandidate c=evaluate_candidate(scene,table,p,n,albedo,rng);
        if(c.target<=0||c.proposal_area<=0) continue;
        const Scalar w=c.target/c.proposal_area;
        weight_sum+=w;
        if(weight_sum>0 && uniform(rng)<w/weight_sum) selected=c;
    }
    if(selected.target<=0||weight_sum<=0||M<=0) return {};
    if(!visible_to_light(scene,p,n,selected,stats,shadow_rays)) return {};
    const Scalar normalization=weight_sum/(static_cast<Scalar>(M)*selected.target);
    return beta*selected.f_area*normalization;
}

Scalar bsdf_hit_light_pdf(const LightAliasTable& table,const hx::PackedHit& hit,const Point3& previous_point,const Vec3& ray_dir) {
    const Scalar select=table.primitive_selection_pdf(hit.primitive_type,hit.primitive_index);
    const Scalar area=table.primitive_area(hit.primitive_type,hit.primitive_index);
    if(select<=0||area<=0) return 0;
    const Vec3 d=hit.p-previous_point; const Scalar dist2=d.length_squared();
    if(dist2<=1e-12f) return 0;
    const Vec3 wi=unit_vector(ray_dir);
    const Vec3 ln=table.primitive_geometric_normal(hit.primitive_type,hit.primitive_index,hit.p);
    const Scalar cos_light=std::fabs(dot(ln,-wi));
    if(cos_light<=1e-7f) return 0;
    return (select/area)*dist2/cos_light;
}

struct LocalStats {
    hx::TraceStats trace{};
    std::uint64_t path_rays{0};
    std::uint64_t shadow_rays{0};
    std::uint64_t light_candidates{0};
};

inline Color background(const Settings& s,const Vec3& dir) {
    const Vec3 d=unit_vector(dir); const Scalar t=0.5f*(d.y+1);
    return (1-t)*s.background_bottom+t*s.background_top;
}

} // namespace

std::size_t exported_scene_bytes(const hx::PackedScene& scene) {
    return scene.gpu_export_bytes();
}

std::size_t exported_quantized_scene_bytes(const hx::PackedScene& scene) {
    return scene.gpu_quantized_export_bytes();
}

Framebuffer atrous_denoise(const Framebuffer& input,const Guides& guides,int iterations,
                            Scalar color_sigma,Scalar normal_sigma,Scalar depth_sigma) {
    if(input.width<=0||input.height<=0||input.pixels.empty()||iterations<=0) return input;
    Framebuffer a=input,b(input.width,input.height);
    static constexpr int k[5]={1,4,6,4,1};
    const Scalar cs2=std::max(1e-8f,color_sigma*color_sigma);
    for(int it=0;it<iterations;++it) {
        const int step=1<<it;
        for(int y=0;y<input.height;++y) for(int x=0;x<input.width;++x) {
            const std::size_t i=static_cast<std::size_t>(y*input.width+x);
            const Color c0=a.pixels[i];
            const Vec3 n0=i<guides.normal.size()?guides.normal[i]:Vec3{};
            const Scalar d0=i<guides.depth.size()?guides.depth[i]:0;
            Color sum{}; Scalar wsum=0;
            for(int oy=-2;oy<=2;++oy) for(int ox=-2;ox<=2;++ox) {
                const int sx=std::clamp(x+ox*step,0,input.width-1), sy=std::clamp(y+oy*step,0,input.height-1);
                const std::size_t j=static_cast<std::size_t>(sy*input.width+sx);
                const Color cj=a.pixels[j]; const Color dc=cj-c0;
                Scalar w=static_cast<Scalar>(k[ox+2]*k[oy+2]);
                w*=std::exp(-dc.length_squared()/cs2);
                if(i<guides.normal.size()&&j<guides.normal.size()&&n0.length_squared()>0&&guides.normal[j].length_squared()>0) {
                    w*=std::pow(std::max(Scalar(0),dot(n0,guides.normal[j])),std::max(Scalar(1),normal_sigma));
                }
                if(i<guides.depth.size()&&j<guides.depth.size()&&d0>0&&guides.depth[j]>0) {
                    const Scalar scale=std::max(1e-5f,depth_sigma*(1+0.1f*d0));
                    w*=std::exp(-std::fabs(guides.depth[j]-d0)/scale);
                }
                sum+=cj*w; wsum+=w;
            }
            b.pixels[i]=wsum>0?sum/wsum:c0;
        }
        std::swap(a.pixels,b.pixels);
    }
    return a;
}

Result render_wavefront(const hx::PackedScene& scene,const Camera& camera,const Settings& in,ProgressCallback progress) {
    if(!scene.built()) throw std::logic_error("Beamcast GX: scene geometry changed; call scene.build() before rendering");
    Settings s=in;
    s.width=std::max(1,s.width); s.height=std::max(1,s.height); s.samples_per_pixel=std::max(1,s.samples_per_pixel);
    s.max_bounces=std::max(1,s.max_bounces); s.work_chunk=std::max(64,s.work_chunk); s.sample_batch=std::max(1,s.sample_batch);
    const int tc=s.thread_count>0?s.thread_count:static_cast<int>(std::thread::hardware_concurrency());
    ParallelExecutor exec(std::max(1,tc));
    const int threads=exec.size();
    const std::size_t pixels=static_cast<std::size_t>(s.width)*static_cast<std::size_t>(s.height);
    Result result; result.noisy=Framebuffer(s.width,s.height); result.image=Framebuffer(s.width,s.height);
    result.guides.width=s.width; result.guides.height=s.height;
    result.guides.normal.assign(pixels,{}); result.guides.depth.assign(pixels,0); result.guides.albedo.assign(pixels,{}); result.guides.position.assign(pixels,{});
    result.report.gpu_export_bytes=exported_scene_bytes(scene);
    result.report.gpu_quantized_export_bytes=exported_quantized_scene_bytes(scene);
    LightAliasTable lights(scene);
    std::vector<LocalStats> locals(static_cast<std::size_t>(threads));
    std::vector<Color> sample_color(pixels);
    std::vector<std::uint32_t> sample_counts(pixels,0);
    std::vector<PathQueue> local_next(static_cast<std::size_t>(threads));
    for(auto& q:local_next) q.reserve(pixels/static_cast<std::size_t>(threads)+64);
    PathQueue active,next; active.reserve(pixels); next.reserve(pixels);
    auto t0=std::chrono::steady_clock::now();

    const int total_batches=(s.samples_per_pixel+s.sample_batch-1)/s.sample_batch;
    int completed_batches=0;
    for(int sample_base=0;sample_base<s.samples_per_pixel;sample_base+=s.sample_batch) {
        const int batch_end=std::min(s.samples_per_pixel,sample_base+s.sample_batch);
        for(int sample=sample_base;sample<batch_end;++sample) {
            std::fill(sample_color.begin(),sample_color.end(),Color{});
            active.clear(); active.reserve(pixels);
            for(std::size_t p=0;p<pixels;++p) {
                const int x=static_cast<int>(p%static_cast<std::size_t>(s.width));
                const int y=static_cast<int>(p/static_cast<std::size_t>(s.width));
                const int budget=pixel_sample_budget(s,x,y);
                if(sample>=budget){++result.report.pixels_skipped_by_vrs;continue;}
                const std::uint64_t seed=hash_seed(s.seed,p,static_cast<std::uint64_t>(sample));
                RNG camera_rng(seed);
                const Scalar u=(static_cast<Scalar>(x)+camera_rng.uniform())/static_cast<Scalar>(std::max(1,s.width-1));
                const Scalar v=(static_cast<Scalar>(y)+camera_rng.uniform())/static_cast<Scalar>(std::max(1,s.height-1));
                active.push(camera.ray(u,v,camera_rng),{1,1,1},{0,0,0},{0,0,0},0,static_cast<std::uint32_t>(p),
                            hash_seed(seed,0xA5A5A5A5u),true);
                ++sample_counts[p]; ++result.report.pixels_sampled;
            }
            result.report.primary_paths+=active.size();
            result.report.peak_active_paths=std::max(result.report.peak_active_paths,active.size());

            for(int bounce=0;bounce<s.max_bounces && !active.empty();++bounce) {
                for(auto& q:local_next) q.clear();
                const bool capture_guides=(sample==0&&bounce==0);
                const auto job=[&](int tid,std::size_t begin,std::size_t end) {
                    auto& ls=locals[static_cast<std::size_t>(tid)]; auto& out=local_next[static_cast<std::size_t>(tid)];
                    for(std::size_t i=begin;i<end;++i) {
                        const Ray ray{active.origin[i],active.direction[i]}; Color beta=active.beta[i],L=active.radiance[i];
                        std::uint64_t rs=active.rng[i]; const std::uint32_t px=active.pixel[i]; ++ls.path_rays;
                        hx::PackedHit h;
                        if(!scene.hit(ray,kEps,std::numeric_limits<Scalar>::infinity(),h,&ls.trace)) {
                            L+=beta*background(s,ray.direction); sample_color[px]=L; continue;
                        }
                        const auto& m=scene.material(h.material);
                        if(capture_guides) {
                            result.guides.normal[px]=h.normal; result.guides.depth[px]=h.t; result.guides.albedo[px]=m.albedo; result.guides.position[px]=h.p;
                        }
                        if(m.type==hx::MaterialType::Emissive) {
                            Scalar mis=1;
                            const bool prev_delta=active.previous_delta[i]!=0;
                            if(s.direct_lighting==DirectLightingMode::ReservoirRIS && !prev_delta && bounce>0) mis=0;
                            else if(s.direct_lighting==DirectLightingMode::AliasMIS && !prev_delta && bounce>0) {
                                const Scalar lp=bsdf_hit_light_pdf(lights,h,active.previous_point[i],ray.direction);
                                mis=power_heuristic(active.previous_bsdf_pdf[i],lp);
                            }
                            L+=beta*m.emission*mis; sample_color[px]=L; continue;
                        }

                        if(m.type==hx::MaterialType::Lambertian && s.direct_lighting!=DirectLightingMode::Off && !lights.empty()) {
                            if(s.direct_lighting==DirectLightingMode::AliasMIS)
                                L+=direct_alias_mis(scene,lights,h.p,h.normal,m.albedo,beta,rs,ls.trace,ls.shadow_rays,ls.light_candidates);
                            else
                                L+=direct_reservoir_ris(scene,lights,h.p,h.normal,m.albedo,beta,s.light_candidates,rs,ls.trace,ls.shadow_rays,ls.light_candidates);
                        }

                        Vec3 dir{}; Scalar bsdf_pdf=0; bool delta=true; Color att=m.albedo;
                        if(m.type==hx::MaterialType::Lambertian) {
                            dir=cosine_hemisphere(h.normal,rs); bsdf_pdf=std::max(Scalar(0),dot(h.normal,dir))/kPi; delta=false;
                        } else if(m.type==hx::MaterialType::Metal) {
                            dir=reflect(unit_vector(ray.direction),h.normal)+m.roughness*random_in_unit_sphere(rs);
                            if(dot(dir,h.normal)<=0){ sample_color[px]=L; continue; }
                        } else {
                            att={1,1,1}; const Scalar eta=h.front_face?(1/m.ior):m.ior; const Vec3 u=unit_vector(ray.direction);
                            const Scalar ct=std::min(dot(-u,h.normal),Scalar(1)); const Scalar st=std::sqrt(std::max(Scalar(0),1-ct*ct));
                            dir=(eta*st>1||schlick(ct,eta)>uniform(rs))?reflect(u,h.normal):refract(u,h.normal,eta);
                        }
                        beta=beta*att;
                        if(s.russian_roulette&&bounce>=s.russian_roulette_start) {
                            const Scalar p=std::clamp(max_component(beta),0.05f,0.98f);
                            if(uniform(rs)>p){sample_color[px]=L;continue;} beta/=p;
                        }
                        if(bounce+1>=s.max_bounces){sample_color[px]=L;continue;}
                        const Vec3 off=dot(dir,h.normal)>=0?h.normal*kEps:-h.normal*kEps;
                        out.push({h.p+off,dir},beta,L,h.p,bsdf_pdf,px,rs,delta);
                    }
                };
                exec.run(active.size(),static_cast<std::size_t>(s.work_chunk),job);
                next.clear();
                std::size_t total_next=0; for(const auto& q:local_next) total_next+=q.size(); next.reserve(total_next);
                for(auto& q:local_next) next.append(std::move(q));
                result.report.queue_pushes+=total_next; ++result.report.queue_compactions;
                if(s.direction_bucketing && total_next>2048){bucket_by_direction(next);++result.report.direction_bucket_passes;}
                result.report.peak_active_paths=std::max(result.report.peak_active_paths,total_next);
                std::swap(active,next);
            }
            for(std::size_t p=0;p<pixels;++p) result.noisy.pixels[p]+=sample_color[p];
        }
        ++completed_batches; if(progress) progress(completed_batches,total_batches);
    }
    for(std::size_t p=0;p<pixels;++p) {
        if(sample_counts[p]>0) result.noisy.pixels[p]/=static_cast<Scalar>(sample_counts[p]);
    }
    auto render_end=std::chrono::steady_clock::now();
    result.report.render_seconds=std::chrono::duration<double>(render_end-t0).count();
    for(const auto& ls:locals) {
        result.report.path_rays+=ls.path_rays; result.report.shadow_rays+=ls.shadow_rays;
        result.report.box_tests+=ls.trace.box_tests; result.report.primitive_tests+=ls.trace.primitive_tests;
        result.report.hits+=ls.trace.hits; result.report.light_candidates+=ls.light_candidates;
    }
    if(s.denoise) {
        auto d0=std::chrono::steady_clock::now();
        result.image=atrous_denoise(result.noisy,result.guides,s.denoise_iterations,s.denoise_color_sigma,s.denoise_normal_sigma,s.denoise_depth_sigma);
        result.report.denoise_seconds=std::chrono::duration<double>(std::chrono::steady_clock::now()-d0).count();
    } else result.image=result.noisy;
    result.report.total_seconds=std::chrono::duration<double>(std::chrono::steady_clock::now()-t0).count();
    const std::uint64_t total_rays=result.report.path_rays+result.report.shadow_rays;
    result.report.rays_per_second=result.report.render_seconds>0?static_cast<double>(total_rays)/result.report.render_seconds:0;
    return result;
}

} // namespace beamcast::gx
