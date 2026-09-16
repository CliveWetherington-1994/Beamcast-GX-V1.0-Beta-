#include "beamcast/gx_research.hpp"

#include <algorithm>
#include <cmath>
#include <cstring>
#include <filesystem>
#include <fstream>
#include <iomanip>
#include <limits>
#include <numeric>
#include <sstream>
#include <stdexcept>

#if defined(_WIN32)
#include <windows.h>
#else
#include <dlfcn.h>
#include <unistd.h>
#endif

namespace beamcast::gx {
namespace {

constexpr Scalar kPi = 3.14159265358979323846f;
constexpr Scalar kEps = 1.0e-4f;

inline Scalar lum(const Color& c) { return 0.2126f*c.x + 0.7152f*c.y + 0.0722f*c.z; }
inline std::uint64_t mix64(std::uint64_t x) {
    x ^= x >> 30; x *= 0xbf58476d1ce4e5b9ULL;
    x ^= x >> 27; x *= 0x94d049bb133111ebULL;
    x ^= x >> 31; return x;
}
inline std::uint64_t hash3(std::int64_t x,std::int64_t y,std::int64_t z) {
    return mix64(static_cast<std::uint64_t>(x)*0x9E3779B185EBCA87ULL ^
                 static_cast<std::uint64_t>(y)*0xC2B2AE3D27D4EB4FULL ^
                 static_cast<std::uint64_t>(z)*0x165667B19E3779F9ULL);
}
inline std::uint64_t rng_next(std::uint64_t& state) {
    std::uint64_t x=state?state:1ULL;
    x^=x>>12; x^=x<<25; x^=x>>27; state=x;
    return x*2685821657736338717ULL;
}
inline Scalar rng_uniform(std::uint64_t& state) {
    return static_cast<Scalar>(static_cast<std::uint32_t>(rng_next(state)>>40)) / static_cast<Scalar>(1u<<24);
}
inline void face_normal(const Ray& ray,const Vec3& outward,hx::PackedHit& h) {
    h.front_face=dot(ray.direction,outward)<0;
    h.normal=h.front_face?outward:-outward;
}

bool hit_sphere(const hx::PackedSphere& s,std::uint32_t index,const Ray& ray,Scalar tmin,Scalar tmax,hx::PackedHit& h) {
    const Vec3 oc=ray.origin-s.center;
    const Scalar a=ray.direction.length_squared();
    const Scalar half_b=dot(oc,ray.direction);
    const Scalar c=oc.length_squared()-s.radius*s.radius;
    const Scalar disc=half_b*half_b-a*c;
    if(disc<0||a<=0) return false;
    const Scalar sq=std::sqrt(disc);
    Scalar root=(-half_b-sq)/a;
    if(root<tmin||root>tmax){root=(-half_b+sq)/a;if(root<tmin||root>tmax)return false;}
    h.t=root;h.p=ray.origin+root*ray.direction;h.material=s.material;h.primitive_index=index;h.primitive_type=0;
    face_normal(ray,(h.p-s.center)/s.radius,h);return true;
}

bool hit_triangle(const hx::PackedTriangle& t,std::uint32_t index,const Ray& ray,Scalar tmin,Scalar tmax,hx::PackedHit& h) {
    const Vec3 pvec=cross(ray.direction,t.e2);
    const Scalar det=dot(t.e1,pvec);
    if(std::fabs(det)<1e-8f) return false;
    const Scalar inv=1/det; const Vec3 tv=ray.origin-t.v0;
    const Scalar u=dot(tv,pvec)*inv;if(u<0||u>1)return false;
    const Vec3 q=cross(tv,t.e1);const Scalar v=dot(ray.direction,q)*inv;if(v<0||u+v>1)return false;
    const Scalar tt=dot(t.e2,q)*inv;if(tt<tmin||tt>tmax)return false;
    h.t=tt;h.p=ray.origin+tt*ray.direction;h.material=t.material;h.primitive_index=index;h.primitive_type=1;
    face_normal(ray,t.normal,h);return true;
}

#if defined(_WIN32)
using DynLib=HMODULE;
DynLib open_library(const char* name){return LoadLibraryA(name);}
void close_library(DynLib h){if(h)FreeLibrary(h);}
void* load_symbol(DynLib h,const char* name){return reinterpret_cast<void*>(GetProcAddress(h,name));}
#else
using DynLib=void*;
DynLib open_library(const char* name){return dlopen(name,RTLD_LAZY|RTLD_LOCAL);}
void close_library(DynLib h){if(h)dlclose(h);}
void* load_symbol(DynLib h,const char* name){return dlsym(h,name);}
#endif

DynLib open_any(const char* const* names,std::size_t count){for(std::size_t i=0;i<count;++i)if(auto h=open_library(names[i]))return h;return {};}
bool runtime_library(const char* const* names,std::size_t count){auto h=open_any(names,count);const bool ok=h!=nullptr;close_library(h);return ok;}

struct RuntimeProbe { std::uint32_t devices{0}; std::uint32_t compute_queues{0}; std::string detail; };

RuntimeProbe probe_opencl(const char* const* names,std::size_t count){
    RuntimeProbe out;auto lib=open_any(names,count);if(!lib)return out;
    using cl_int=std::int32_t;using cl_uint=std::uint32_t;using cl_platform_id=void*;using cl_device_id=void*;using cl_device_type=std::uint64_t;
    using GetPlatforms=cl_int(*)(cl_uint,cl_platform_id*,cl_uint*);using GetDevices=cl_int(*)(cl_platform_id,cl_device_type,cl_uint,cl_device_id*,cl_uint*);
    auto gp=reinterpret_cast<GetPlatforms>(load_symbol(lib,"clGetPlatformIDs"));auto gd=reinterpret_cast<GetDevices>(load_symbol(lib,"clGetDeviceIDs"));
    if(gp&&gd){cl_uint np=0;const cl_int rc=gp(0,nullptr,&np);if(rc==0&&np){std::vector<cl_platform_id> p(np);if(gp(np,p.data(),nullptr)==0){for(auto platform:p){cl_uint nd=0;if(gd(platform,cl_device_type(0xFFFFFFFFULL),0,nullptr,&nd)==0)out.devices+=nd;}}}else if(rc==-1001)out.detail="OpenCL loader present, no installed platform ICD";}
    close_library(lib);return out;
}

RuntimeProbe probe_cuda(const char* const* names,std::size_t count){RuntimeProbe out;auto lib=open_any(names,count);if(!lib)return out;using Init=int(*)(unsigned int);using Count=int(*)(int*);auto init=reinterpret_cast<Init>(load_symbol(lib,"cuInit"));auto get=reinterpret_cast<Count>(load_symbol(lib,"cuDeviceGetCount"));if(init&&get&&init(0)==0){int n=0;if(get(&n)==0&&n>0)out.devices=static_cast<std::uint32_t>(n);}close_library(lib);return out;}
RuntimeProbe probe_hip(const char* const* names,std::size_t count){RuntimeProbe out;auto lib=open_any(names,count);if(!lib)return out;using Count=int(*)(int*);auto get=reinterpret_cast<Count>(load_symbol(lib,"hipGetDeviceCount"));if(get){int n=0;if(get(&n)==0&&n>0)out.devices=static_cast<std::uint32_t>(n);}close_library(lib);return out;}

RuntimeProbe probe_vulkan(const char* const* names,std::size_t count){
    RuntimeProbe out;auto lib=open_any(names,count);if(!lib)return out;
    using VkInstance=void*;using VkPhysicalDevice=void*;using VkResult=std::int32_t;using VkFlags=std::uint32_t;
    struct VkApplicationInfo{std::int32_t sType;const void* pNext;const char* pApplicationName;std::uint32_t applicationVersion;const char* pEngineName;std::uint32_t engineVersion;std::uint32_t apiVersion;};
    struct VkInstanceCreateInfo{std::int32_t sType;const void* pNext;VkFlags flags;const VkApplicationInfo* pApplicationInfo;std::uint32_t enabledLayerCount;const char* const* ppEnabledLayerNames;std::uint32_t enabledExtensionCount;const char* const* ppEnabledExtensionNames;};
    struct VkExtent3D{std::uint32_t width,height,depth;};
    struct VkQueueFamilyProperties{VkFlags queueFlags;std::uint32_t queueCount;std::uint32_t timestampValidBits;VkExtent3D minImageTransferGranularity;};
    using Create=VkResult(*)(const VkInstanceCreateInfo*,const void*,VkInstance*);using Destroy=void(*)(VkInstance,const void*);using Enum=VkResult(*)(VkInstance,std::uint32_t*,VkPhysicalDevice*);using Queues=void(*)(VkPhysicalDevice,std::uint32_t*,VkQueueFamilyProperties*);
    auto create=reinterpret_cast<Create>(load_symbol(lib,"vkCreateInstance"));auto destroy=reinterpret_cast<Destroy>(load_symbol(lib,"vkDestroyInstance"));auto enumerate=reinterpret_cast<Enum>(load_symbol(lib,"vkEnumeratePhysicalDevices"));auto queues=reinterpret_cast<Queues>(load_symbol(lib,"vkGetPhysicalDeviceQueueFamilyProperties"));
    if(create&&destroy&&enumerate&&queues){VkApplicationInfo app{0,nullptr,"Beamcast GX",1,"Beamcast",1,(1u<<22)};VkInstanceCreateInfo ci{1,nullptr,0,&app,0,nullptr,0,nullptr};VkInstance instance=nullptr;if(create(&ci,nullptr,&instance)==0&&instance){std::uint32_t n=0;if(enumerate(instance,&n,nullptr)==0&&n){std::vector<VkPhysicalDevice> dev(n);if(enumerate(instance,&n,dev.data())==0){out.devices=n;for(auto d:dev){std::uint32_t qn=0;queues(d,&qn,nullptr);std::vector<VkQueueFamilyProperties> q(qn);if(qn)queues(d,&qn,q.data());for(const auto& f:q)if((f.queueFlags&0x2u)&&f.queueCount)++out.compute_queues;}}}destroy(instance,nullptr);}}
    if(out.devices==0) out.detail="Vulkan loader present, no usable physical device in this environment";
    close_library(lib);return out;
}


Scalar node_area(const hx::GpuBvhNode& n) {
    const Vec3 e=n.bmax-n.bmin;
    return std::max(Scalar(0),Scalar(2)*(e.x*e.y+e.y*e.z+e.z*e.x));
}

struct RestirLightDistribution {
    struct Ref {std::uint32_t type,index,material;Scalar area,pdf,cdf;};
    const hx::PackedScene* scene{nullptr};
    std::vector<Ref> lights;
    explicit RestirLightDistribution(const hx::PackedScene& s):scene(&s){
        Scalar total=0;
        for(std::uint32_t i=0;i<s.spheres().size();++i){const auto& p=s.spheres()[i];if(p.material>=s.materials().size())continue;const auto&m=s.materials()[p.material];if(m.type!=hx::MaterialType::Emissive)continue;const Scalar a=4*kPi*p.radius*p.radius,w=a*std::max(Scalar(0),lum(m.emission));if(w>0){lights.push_back({0,i,p.material,a,w,0});total+=w;}}
        for(std::uint32_t i=0;i<s.triangles().size();++i){const auto& t=s.triangles()[i];if(t.material>=s.materials().size())continue;const auto&m=s.materials()[t.material];if(m.type!=hx::MaterialType::Emissive)continue;const Scalar a=0.5f*cross(t.e1,t.e2).length(),w=a*std::max(Scalar(0),lum(m.emission));if(w>0){lights.push_back({1,i,t.material,a,w,0});total+=w;}}
        Scalar c=0;if(total>0)for(auto& l:lights){l.pdf/=total;c+=l.pdf;l.cdf=c;}if(!lights.empty())lights.back().cdf=1;
    }
    bool empty()const{return lights.empty();}
    const Ref& select(std::uint64_t& rng)const{const Scalar u=rng_uniform(rng);auto it=std::lower_bound(lights.begin(),lights.end(),u,[](const Ref& a,Scalar v){return a.cdf<v;});return it==lights.end()?lights.back():*it;}
    RestirLightSample sample(std::uint64_t& rng)const{
        const auto& l=select(rng);RestirLightSample s{};s.primitive_type=l.type;s.primitive_index=l.index;s.area=l.area;s.selection_pdf=l.pdf;s.emission=scene->materials()[l.material].emission;
        if(l.type==0){const auto& p=scene->spheres()[l.index];const Scalar z=1-2*rng_uniform(rng),a=2*kPi*rng_uniform(rng),r=std::sqrt(std::max(Scalar(0),1-z*z));s.normal={r*std::cos(a),r*std::sin(a),z};s.position=p.center+p.radius*s.normal;}
        else{const auto&t=scene->triangles()[l.index];const Scalar u=rng_uniform(rng),v=rng_uniform(rng),su=std::sqrt(u),b1=su*(1-v),b2=su*v;s.position=t.v0+b1*t.e1+b2*t.e2;s.normal=t.normal;}
        return s;
    }
};

struct Reeval { Color f_area{}; Scalar target{0}; Scalar proposal_area{0}; Scalar distance{0}; Vec3 wi{}; };
Reeval reevaluate(const SurfacePoint& surf,const RestirLightSample& ls) {
    Reeval r{};if(!surf.valid||ls.area<=0||ls.selection_pdf<=0)return r;Vec3 d=ls.position-surf.position;const Scalar d2=d.length_squared();if(d2<=1e-12f)return r;r.distance=std::sqrt(d2);r.wi=d/r.distance;const Scalar cs=std::max(Scalar(0),dot(surf.normal,r.wi));const Scalar cl=std::fabs(dot(ls.normal,-r.wi));if(cs<=0||cl<=1e-8f)return r;r.f_area=(surf.albedo*(1/kPi))*ls.emission*(cs*cl/d2);r.target=std::max(Scalar(0),lum(r.f_area));r.proposal_area=ls.selection_pdf/ls.area;return r;
}
bool sample_visible(const hx::PackedScene& scene,const SurfacePoint& s,const Reeval& e,std::uint64_t& rays){if(e.distance<=2*kEps)return false;++rays;hx::PackedHit h;return !scene.hit({s.position+s.normal*kEps,e.wi},kEps,e.distance-2*kEps,h,nullptr);}
void reservoir_add(RestirReservoir& r,const RestirLightSample& s,Scalar target,Scalar weight,std::uint32_t M,std::uint64_t& rng){
    if(weight<=0||target<=0||M==0)return;
    const Scalar old=r.weight_sum;
    const Scalar combined=old+weight;
    if(!std::isfinite(combined)||combined<old){r={};return;}
    if(!r.valid||rng_uniform(rng)<weight/std::max(combined,1e-20f)){r.sample=s;r.selected_target=target;r.valid=true;}
    const std::uint64_t total_m=static_cast<std::uint64_t>(r.M)+M;
    if(total_m>std::numeric_limits<std::uint32_t>::max()){
        const Scalar scale=static_cast<Scalar>(std::numeric_limits<std::uint32_t>::max())/static_cast<Scalar>(total_m);
        r.weight_sum=combined*scale;r.M=std::numeric_limits<std::uint32_t>::max();
    }else{r.weight_sum=combined;r.M=static_cast<std::uint32_t>(total_m);}
}

Color bilinear_color(const Framebuffer& f,Scalar x,Scalar y){if(f.width<=0||f.height<=0)return {};x=std::clamp(x,Scalar(0),Scalar(f.width-1));y=std::clamp(y,Scalar(0),Scalar(f.height-1));const int x0=static_cast<int>(std::floor(x)),y0=static_cast<int>(std::floor(y)),x1=std::min(f.width-1,x0+1),y1=std::min(f.height-1,y0+1);const Scalar tx=x-x0,ty=y-y0;return (1-ty)*((1-tx)*f.at(x0,y0)+tx*f.at(x1,y0))+ty*((1-tx)*f.at(x0,y1)+tx*f.at(x1,y1));}

} // namespace

const char* backend_name(NativeBackend b){switch(b){case NativeBackend::CPU:return "CPU";case NativeBackend::OpenCL:return "OpenCL";case NativeBackend::Vulkan:return "Vulkan";case NativeBackend::CUDA:return "CUDA";case NativeBackend::HIP:return "HIP";}return "Unknown";}

std::vector<BackendInfo> discover_native_backends(){
    std::vector<BackendInfo> out;
    out.push_back({NativeBackend::CPU,"CPU reference",true,true,true,1,1,"host reference backend"});
#if defined(_WIN32)
    const char* ocl[]={"OpenCL.dll"}; const char* vk[]={"vulkan-1.dll"}; const char* cu[]={"nvcuda.dll"}; const char* hip[]={"amdhip64.dll"};
#else
    const char* ocl[]={"libOpenCL.so.1","libOpenCL.so"}; const char* vk[]={"libvulkan.so.1","libvulkan.so"}; const char* cu[]={"libcuda.so.1","libcuda.so"}; const char* hip[]={"libamdhip64.so"};
#endif
#ifdef BEAMCAST_HAS_OPENCL_BACKEND
    constexpr bool opencl_compiled=true;
#else
    constexpr bool opencl_compiled=false;
#endif
#ifdef BEAMCAST_HAS_VULKAN_BACKEND
    constexpr bool vulkan_compiled=true;
#else
    constexpr bool vulkan_compiled=false;
#endif
#ifdef BEAMCAST_HAS_CUDA_BACKEND
    constexpr bool cuda_compiled=true;
#else
    constexpr bool cuda_compiled=false;
#endif
#ifdef BEAMCAST_HAS_HIP_BACKEND
    constexpr bool hip_compiled=true;
#else
    constexpr bool hip_compiled=false;
#endif
    const auto op=probe_opencl(ocl,sizeof(ocl)/sizeof(ocl[0]));const auto vp=probe_vulkan(vk,sizeof(vk)/sizeof(vk[0]));const auto cp=probe_cuda(cu,sizeof(cu)/sizeof(cu[0]));const auto hp=probe_hip(hip,sizeof(hip)/sizeof(hip[0]));
    out.push_back({NativeBackend::OpenCL,"OpenCL",runtime_library(ocl,sizeof(ocl)/sizeof(ocl[0])),opencl_compiled,true,op.devices,op.devices,op.detail});
    out.push_back({NativeBackend::Vulkan,"Vulkan compute",runtime_library(vk,sizeof(vk)/sizeof(vk[0])),vulkan_compiled,true,vp.devices,vp.compute_queues,vp.detail});
    out.push_back({NativeBackend::CUDA,"CUDA",runtime_library(cu,sizeof(cu)/sizeof(cu[0])),cuda_compiled,true,cp.devices,cp.devices,cp.detail});
    out.push_back({NativeBackend::HIP,"HIP",runtime_library(hip,sizeof(hip)/sizeof(hip[0])),hip_compiled,true,hp.devices,hp.devices,hp.detail});
    return out;
}


DeviceQueueCompactionResult compact_alive_reference(const std::vector<std::uint32_t>& flags){DeviceQueueCompactionResult r;r.prefix.resize(flags.size());std::uint32_t sum=0;for(std::size_t i=0;i<flags.size();++i){r.prefix[i]=sum;if(flags[i]){r.live_indices.push_back(static_cast<std::uint32_t>(i));++sum;}}return r;}

std::vector<std::uint32_t> radix_sort_indices_by_key(const std::vector<std::uint32_t>& keys){
    std::vector<std::uint32_t> a(keys.size()),b(keys.size());
    std::iota(a.begin(),a.end(),0u);
    std::array<std::size_t,256> count{};
    for(int pass=0;pass<4;++pass){
        count.fill(0);const int shift=pass*8;
        for(auto i:a)++count[(keys[i]>>shift)&255u];
        std::array<std::size_t,256> offset{};std::size_t sum=0;
        for(std::size_t i=0;i<256;++i){offset[i]=sum;sum+=count[i];}
        for(auto i:a)b[offset[(keys[i]>>shift)&255u]++]=i;
        a.swap(b);
    }
    return a;
}

Scalar queue_coherence_score(const std::vector<std::uint32_t>& keys,const std::vector<std::uint32_t>& order){
    if(order.size()!=keys.size()||keys.size()<2)return keys.empty()?0.0f:1.0f;
    std::size_t same=0;
    for(std::size_t i=1;i<order.size();++i)if(keys[order[i]]==keys[order[i-1]])++same;
    return static_cast<Scalar>(same)/static_cast<Scalar>(order.size()-1);
}

QueueOrderingDecision evaluate_queue_ordering(const std::vector<std::uint32_t>& keys,double divergence_cost_ns_per_ray){
    QueueOrderingDecision d;if(keys.empty())return d;
    std::vector<std::uint32_t> identity(keys.size());std::iota(identity.begin(),identity.end(),0u);
    d.before_coherence=queue_coherence_score(keys,identity);
    const auto t0=std::chrono::steady_clock::now();const auto sorted=radix_sort_indices_by_key(keys);const auto t1=std::chrono::steady_clock::now();
    d.after_coherence=queue_coherence_score(keys,sorted);d.measured_sort_ns=std::chrono::duration<double,std::nano>(t1-t0).count();
    d.estimated_saved_ns=std::max(0.0,static_cast<double>(d.after_coherence-d.before_coherence))*static_cast<double>(keys.size())*std::max(0.0,divergence_cost_ns_per_ray);
    d.sort=d.estimated_saved_ns>d.measured_sort_ns;return d;
}

void WideBvh8::build(const hx::PackedScene& scene){
    const auto g=scene.export_gpu();nodes_.clear();refs_=g.refs;spheres_=g.spheres;triangles_=g.triangles;
    if(g.nodes.empty()){world_min_={};world_extent_={1,1,1};return;}
    world_min_=g.nodes[0].bmin;world_extent_=g.nodes[0].bmax-g.nodes[0].bmin;for(int a=0;a<3;++a)if(world_extent_[a]<1e-9f)world_extent_[a]=1;
    auto qmin=[&](Scalar v,int a){const Scalar u=(v-world_min_[a])/world_extent_[a]*65535.0f;return static_cast<std::uint16_t>(std::clamp(std::floor(u),0.0f,65535.0f));};
    auto qmax=[&](Scalar v,int a){const Scalar u=(v-world_min_[a])/world_extent_[a]*65535.0f;return static_cast<std::uint16_t>(std::clamp(std::ceil(u),0.0f,65535.0f));};
    std::function<std::uint32_t(std::uint32_t)> build_node=[&](std::uint32_t src)->std::uint32_t{
        const std::uint32_t dst=static_cast<std::uint32_t>(nodes_.size());nodes_.push_back({});
        std::vector<std::uint32_t> frontier{src};
        while(frontier.size()<8){std::size_t best=frontier.size();Scalar area=-1;for(std::size_t i=0;i<frontier.size();++i){const auto& n=g.nodes[frontier[i]];if(!n.leaf){const Scalar a=node_area(n);if(a>area){area=a;best=i;}}}if(best==frontier.size())break;const auto n=g.nodes[frontier[best]];frontier.erase(frontier.begin()+static_cast<std::ptrdiff_t>(best));frontier.insert(frontier.begin()+static_cast<std::ptrdiff_t>(best),n.right);frontier.insert(frontier.begin()+static_cast<std::ptrdiff_t>(best),n.left);}
        WideBvh8Node node{};
        node.child_count=static_cast<std::uint8_t>(frontier.size());
        for(std::size_t i=0;i<frontier.size();++i){
            const auto& sn=g.nodes[frontier[i]];
            for(int axis=0;axis<3;++axis){node.bmin[axis][i]=qmin(sn.bmin[axis],axis);node.bmax[axis][i]=qmax(sn.bmax[axis],axis);}
            if(sn.leaf){
                if(sn.count>std::numeric_limits<std::uint16_t>::max()) throw std::runtime_error("BVH8 leaf exceeds 16-bit primitive count");
                node.kind[i]=2;node.index[i]=sn.first;node.count[i]=static_cast<std::uint16_t>(sn.count);
            }else{node.kind[i]=1;node.index[i]=build_node(frontier[i]);}
        }
        nodes_[dst]=node;return dst;
    };
    build_node(0);
}

bool WideBvh8::hit(const Ray& ray,Scalar tmin,Scalar tmax,hx::PackedHit& best,hx::TraceStats* stats) const {
    if(stats) ++stats->rays;
    if(nodes_.empty()) return false;
    best.t=tmax;
    bool any=false;
    struct Action { std::uint8_t kind; std::uint32_t index,count; Scalar enter; };
    std::vector<Action> stack;
    stack.push_back({1,0,0,tmin});
    auto decode=[&](const WideBvh8Node& node,std::size_t child){
        Point3 mn{},mx{};
        for(int axis=0;axis<3;++axis){
            mn[axis]=world_min_[axis]+world_extent_[axis]*(static_cast<Scalar>(node.bmin[axis][child])/65535.0f);
            mx[axis]=world_min_[axis]+world_extent_[axis]*(static_cast<Scalar>(node.bmax[axis][child])/65535.0f);
        }
        return AABB{mn,mx};
    };
    while(!stack.empty()){
        const Action action=stack.back();
        stack.pop_back();
        if(action.enter>best.t) continue;
        if(action.kind==2){
            for(std::uint32_t j=0;j<action.count;++j){
                if(action.index+j>=refs_.size()) continue;
                const auto& ref=refs_[action.index+j];
                if(stats) ++stats->primitive_tests;
                hx::PackedHit h;
                const bool ok=ref.type==0
                    ? (ref.index<spheres_.size() && hit_sphere(spheres_[ref.index],ref.index,ray,tmin,best.t,h))
                    : (ref.index<triangles_.size() && hit_triangle(triangles_[ref.index],ref.index,ray,tmin,best.t,h));
                if(ok){best=h;any=true;}
            }
            continue;
        }
        if(action.index>=nodes_.size()) continue;
        const auto& node=nodes_[action.index];
        std::array<Action,8> children{};
        std::size_t count=0;
        for(std::uint32_t i=0;i<node.child_count && i<8;++i){
            if(!node.kind[i]) continue;
            Scalar enter=0;
            if(stats) ++stats->box_tests;
            if(decode(node,i).intersect(ray,tmin,best.t,&enter,nullptr)) children[count++]={node.kind[i],node.index[i],node.count[i],enter};
        }
        for(std::size_t i=1;i<count;++i){
            Action key=children[i];
            std::size_t j=i;
            while(j>0 && children[j-1].enter<key.enter){children[j]=children[j-1];--j;}
            children[j]=key;
        }
        for(std::size_t i=0;i<count;++i) stack.push_back(children[i]);
    }
    if(any && stats) ++stats->hits;
    return any;
}
std::size_t WideBvh8::byte_size()const{return nodes_.size()*sizeof(WideBvh8Node)+refs_.size()*sizeof(hx::GpuPrimitiveRef)+spheres_.size()*sizeof(hx::PackedSphere)+triangles_.size()*sizeof(hx::PackedTriangle);}
Scalar WideBvh8::average_child_occupancy()const{if(nodes_.empty())return 0;std::size_t used=0;for(const auto&n:nodes_)used+=n.child_count;return static_cast<Scalar>(used)/(static_cast<Scalar>(nodes_.size())*8.0f);}

void RestirDIHistory::reset(){width=height=0;reservoirs.clear();surfaces.clear();}
std::vector<SurfacePoint> surfaces_from_guides(const Guides& g){const std::size_t n=static_cast<std::size_t>(std::max(0,g.width))*static_cast<std::size_t>(std::max(0,g.height));std::vector<SurfacePoint> s(n);for(std::size_t i=0;i<n;++i){if(i<g.position.size())s[i].position=g.position[i];if(i<g.normal.size())s[i].normal=g.normal[i];if(i<g.albedo.size())s[i].albedo=g.albedo[i];if(i<g.depth.size())s[i].depth=g.depth[i];s[i].valid=i<g.position.size()&&i<g.normal.size()&&i<g.depth.size()&&s[i].depth>0&&s[i].normal.length_squared()>0;}return s;}

std::vector<MotionVector> motion_vectors_from_positions(const std::vector<Point3>& positions,int width,int height,const Camera& current_camera,const Camera& previous_camera){
    if(width<=0||height<=0||positions.size()!=static_cast<std::size_t>(width)*static_cast<std::size_t>(height))throw std::invalid_argument("motion vector position count mismatch");
    std::vector<MotionVector> out(positions.size());
    const Scalar sx=static_cast<Scalar>(std::max(1,width-1)),sy=static_cast<Scalar>(std::max(1,height-1));
    for(std::size_t i=0;i<positions.size();++i){Scalar cu=0,cv=0,pu=0,pv=0;if(current_camera.project(positions[i],cu,cv)&&previous_camera.project(positions[i],pu,pv)){out[i].x=(pu-cu)*sx;out[i].y=(pv-cv)*sy;}}
    return out;
}

RestirDIResult restir_di(const hx::PackedScene& scene,
                         int width,
                         int height,
                         const std::vector<SurfacePoint>& surfaces,
                         const std::vector<MotionVector>& motion,
                         const RestirDIHistory* previous,
                         const RestirDIConfig& cfg) {
    if(width<=0 || height<=0) throw std::invalid_argument("restir_di dimensions must be positive");
    const std::size_t n=static_cast<std::size_t>(width)*static_cast<std::size_t>(height);
    if(surfaces.size()!=n) throw std::invalid_argument("restir_di surface count mismatch");
    if(!motion.empty() && motion.size()!=n) throw std::invalid_argument("restir_di motion count mismatch");

    RestirDIResult out;
    out.direct=Framebuffer(width,height);
    out.reservoirs.resize(n);
    RestirLightDistribution lights(scene);
    if(lights.empty()) return out;

    auto compatible=[&](const SurfacePoint& a,const SurfacePoint& b){
        if(!a.valid || !b.valid) return false;
        if(dot(a.normal,b.normal)<cfg.normal_threshold) return false;
        if(a.depth>0 && b.depth>0){
            const Scalar rel=std::fabs(a.depth-b.depth)/std::max(Scalar(1e-4f),std::max(a.depth,b.depth));
            if(rel>cfg.relative_depth_threshold) return false;
        }
        return true;
    };

    for(std::size_t i=0;i<n;++i){
        if(!surfaces[i].valid) continue;
        std::uint64_t rng=mix64(cfg.seed^static_cast<std::uint64_t>(i));
        RestirReservoir reservoir{};
        for(int c=0;c<std::max(1,cfg.initial_candidates);++c){
            const auto sample=lights.sample(rng);
            const auto eval=reevaluate(surfaces[i],sample);
            ++out.fresh_candidates;
            if(eval.target>0 && eval.proposal_area>0)
                reservoir_add(reservoir,sample,eval.target,eval.target/eval.proposal_area,1,rng);
        }

        const bool temporal_ok=cfg.temporal_reuse && previous && previous->width==width && previous->height==height &&
                               previous->reservoirs.size()==n && previous->surfaces.size()==n;
        if(temporal_ok){
            const int x=static_cast<int>(i%static_cast<std::size_t>(width));
            const int y=static_cast<int>(i/static_cast<std::size_t>(width));
            const MotionVector mv=motion.empty()?MotionVector{}:motion[i];
            const int px=static_cast<int>(std::lround(static_cast<Scalar>(x)+mv.x));
            const int py=static_cast<int>(std::lround(static_cast<Scalar>(y)+mv.y));
            if(px>=0 && py>=0 && px<width && py<height){
                const std::size_t pi=static_cast<std::size_t>(py*width+px);
                const auto& previous_reservoir=previous->reservoirs[pi];
                if(previous_reservoir.valid && compatible(surfaces[i],previous->surfaces[pi])){
                    const auto eval=reevaluate(surfaces[i],previous_reservoir.sample);
                    bool visible=eval.target>0;
                    if(visible && cfg.visibility_validation) visible=sample_visible(scene,surfaces[i],eval,out.visibility_rays);
                    if(visible && previous_reservoir.selected_target>0 && previous_reservoir.M>0){
                        const Scalar W=previous_reservoir.weight_sum/(static_cast<Scalar>(previous_reservoir.M)*previous_reservoir.selected_target);
                        reservoir_add(reservoir,previous_reservoir.sample,eval.target,
                                      eval.target*W*static_cast<Scalar>(previous_reservoir.M),
                                      previous_reservoir.M,rng);
                        ++out.temporal_reuses;
                    }
                }
            }
        }
        out.reservoirs[i]=reservoir;
    }

    if(cfg.spatial_reuse && cfg.spatial_neighbors>0){
        const auto base=out.reservoirs;
        for(std::size_t i=0;i<n;++i){
            if(!surfaces[i].valid) continue;
            std::uint64_t rng=mix64(cfg.seed^0x5350415449414cULL^static_cast<std::uint64_t>(i));
            auto reservoir=out.reservoirs[i];
            const int x=static_cast<int>(i%static_cast<std::size_t>(width));
            const int y=static_cast<int>(i/static_cast<std::size_t>(width));
            for(int k=0;k<cfg.spatial_neighbors;++k){
                const int dx=static_cast<int>(std::lround((2*rng_uniform(rng)-1)*cfg.spatial_radius));
                const int dy=static_cast<int>(std::lround((2*rng_uniform(rng)-1)*cfg.spatial_radius));
                const int nx=x+dx,ny=y+dy;
                if(nx<0 || ny<0 || nx>=width || ny>=height) continue;
                const std::size_t ni=static_cast<std::size_t>(ny*width+nx);
                if(ni==i) continue;
                const auto& neighbor=base[ni];
                if(!neighbor.valid || !compatible(surfaces[i],surfaces[ni])) continue;
                const auto eval=reevaluate(surfaces[i],neighbor.sample);
                bool visible=eval.target>0;
                if(visible && cfg.visibility_validation) visible=sample_visible(scene,surfaces[i],eval,out.visibility_rays);
                if(!visible || neighbor.selected_target<=0 || neighbor.M==0) continue;
                const Scalar W=neighbor.weight_sum/(static_cast<Scalar>(neighbor.M)*neighbor.selected_target);
                reservoir_add(reservoir,neighbor.sample,eval.target,
                              eval.target*W*static_cast<Scalar>(neighbor.M),neighbor.M,rng);
                ++out.spatial_reuses;
            }
            out.reservoirs[i]=reservoir;
        }
    }

    for(std::size_t i=0;i<n;++i){
        const auto& reservoir=out.reservoirs[i];
        if(!reservoir.valid || reservoir.M==0 || reservoir.selected_target<=0 || !surfaces[i].valid) continue;
        const auto eval=reevaluate(surfaces[i],reservoir.sample);
        if(eval.target<=0) continue;
        if(!sample_visible(scene,surfaces[i],eval,out.visibility_rays)) continue;
        const Scalar W=reservoir.weight_sum/(static_cast<Scalar>(reservoir.M)*reservoir.selected_target);
        out.direct.pixels[i]=eval.f_area*W;
    }
    return out;
}

void update_restir_history(RestirDIHistory& h,int w,int ht,const std::vector<SurfacePoint>& s,const std::vector<RestirReservoir>& r){if(w<=0||ht<=0||s.size()!=static_cast<std::size_t>(w*ht)||r.size()!=s.size())throw std::invalid_argument("invalid ReSTIR history dimensions");h.width=w;h.height=ht;h.surfaces=s;h.reservoirs=r;}

std::uint64_t RadianceCache::key(Point3 p)const{return hash3(static_cast<std::int64_t>(std::floor(p.x/cell_size_)),static_cast<std::int64_t>(std::floor(p.y/cell_size_)),static_cast<std::int64_t>(std::floor(p.z/cell_size_)));}
void RadianceCache::begin_frame(std::uint32_t f){frame_=f;}
void RadianceCache::insert(Point3 p,Color r){auto& c=cells_[key(p)];c.sum+=r;++c.count;c.last_frame=frame_;}
std::optional<Color> RadianceCache::lookup(Point3 p,std::uint32_t max_age)const{auto it=cells_.find(key(p));if(it==cells_.end()||it->second.count==0||frame_<it->second.last_frame||frame_-it->second.last_frame>max_age)return std::nullopt;return it->second.sum/static_cast<Scalar>(it->second.count);}
void RadianceCache::prune(std::uint32_t age){for(auto it=cells_.begin();it!=cells_.end();){if(frame_<it->second.last_frame||frame_-it->second.last_frame>age)it=cells_.erase(it);else ++it;}}

std::uint64_t PathGuideGrid::key(Point3 p)const{return hash3(static_cast<std::int64_t>(std::floor(p.x/cell_size_)),static_cast<std::int64_t>(std::floor(p.y/cell_size_)),static_cast<std::int64_t>(std::floor(p.z/cell_size_)));}
void PathGuideGrid::update(Point3 p,Vec3 d,Scalar c){const int oct=(d.x>=0?1:0)|(d.y>=0?2:0)|(d.z>=0?4:0);auto& cell=cells_[key(p)];cell.weight[static_cast<std::size_t>(oct)]+=std::max(Scalar(0),c);}
Vec3 PathGuideGrid::sample(Point3 p,std::uint64_t& rng)const{std::array<Scalar,8>w{{1,1,1,1,1,1,1,1}};auto it=cells_.find(key(p));if(it!=cells_.end())w=it->second.weight;const Scalar sum=std::accumulate(w.begin(),w.end(),Scalar(0));Scalar u=rng_uniform(rng)*sum;int oct=0;for(;oct<7;++oct){if(u<w[static_cast<std::size_t>(oct)])break;u-=w[static_cast<std::size_t>(oct)];}Vec3 v{rng_uniform(rng),rng_uniform(rng),rng_uniform(rng)};if(v.length_squared()<1e-12f)v={1,1,1};v=unit_vector(v);if(!(oct&1))v.x=-v.x;if(!(oct&2))v.y=-v.y;if(!(oct&4))v.z=-v.z;return v;}
Scalar PathGuideGrid::pdf(Point3 p,Vec3 d)const{std::array<Scalar,8>w{{1,1,1,1,1,1,1,1}};auto it=cells_.find(key(p));if(it!=cells_.end())w=it->second.weight;const Scalar sum=std::accumulate(w.begin(),w.end(),Scalar(0));const int oct=(d.x>=0?1:0)|(d.y>=0?2:0)|(d.z>=0?4:0);return sum>0?(w[static_cast<std::size_t>(oct)]/sum)*(2/kPi):1/(4*kPi);}

std::uint32_t ClusterBvh::build_node(std::uint32_t begin,std::uint32_t end){const std::uint32_t idx=static_cast<std::uint32_t>(nodes_.size());nodes_.push_back({});AABB b;for(std::uint32_t i=begin;i<end;++i)b.expand(pages_[order_[i]].bounds);if(end-begin<=4){nodes_[idx]={b,0,0,begin,end-begin,true};return idx;}AABB cb;for(std::uint32_t i=begin;i<end;++i)cb.expand(pages_[order_[i]].bounds.centroid());const Vec3 e=cb.extent();const int axis=e.y>e.x?(e.z>e.y?2:1):(e.z>e.x?2:0);const std::uint32_t mid=begin+(end-begin)/2;std::nth_element(order_.begin()+begin,order_.begin()+mid,order_.begin()+end,[&](std::uint32_t a,std::uint32_t c){return pages_[a].bounds.centroid()[axis]<pages_[c].bounds.centroid()[axis];});const auto l=build_node(begin,mid),r=build_node(mid,end);nodes_[idx]={b,l,r,0,0,false};return idx;}
void ClusterBvh::build(const PagedTriangleStore& store){pages_.clear();order_.clear();nodes_.clear();pages_.reserve(store.page_count());for(std::size_t p=0;p<store.page_count();++p){const auto tris=store.load_page(p);AABB b;for(const auto&t:tris){b.expand(t.v0);b.expand(t.v0+t.e1);b.expand(t.v0+t.e2);}pages_.push_back({b,p,static_cast<std::uint32_t>(tris.size())});}order_.resize(pages_.size());std::iota(order_.begin(),order_.end(),0u);if(!order_.empty())build_node(0,static_cast<std::uint32_t>(order_.size()));}
std::vector<std::size_t> ClusterBvh::query(const Ray& ray,Scalar tmin,Scalar tmax)const{std::vector<std::size_t> out;if(nodes_.empty())return out;std::vector<std::uint32_t> stack{0};while(!stack.empty()){const auto i=stack.back();stack.pop_back();if(i>=nodes_.size()||!nodes_[i].bounds.intersect(ray,tmin,tmax))continue;const auto&n=nodes_[i];if(n.leaf){for(std::uint32_t j=0;j<n.count;++j)out.push_back(pages_[order_[n.first+j]].page);}else{stack.push_back(n.left);stack.push_back(n.right);}}return out;}

PageResidencyManager::PageResidencyManager(const PagedTriangleStore& s,std::size_t hb,std::size_t db,UploadCallback u,EvictCallback e):store_(&s),host_budget_(hb),device_budget_(db),upload_(std::move(u)),evict_(std::move(e)){if(host_budget_==0)throw std::invalid_argument("host residency budget must be non-zero");worker_=std::thread([this]{worker_loop();});}
PageResidencyManager::~PageResidencyManager(){{std::lock_guard<std::mutex>l(mutex_);stop_=true;}work_cv_.notify_all();if(worker_.joinable())worker_.join();}
void PageResidencyManager::request(std::size_t page){if(page>=store_->page_count())throw std::out_of_range("page index");std::lock_guard<std::mutex>l(mutex_);auto& e=entries_[page];e.stamp=++stamp_;if(e.ready||e.loading)return;e.loading=true;pending_.push_back(page);work_cv_.notify_one();}
bool PageResidencyManager::wait(std::size_t page){request(page);std::unique_lock<std::mutex>l(mutex_);cv_.wait(l,[&]{auto it=entries_.find(page);return stop_||it==entries_.end()||it->second.ready||!it->second.loading;});auto it=entries_.find(page);return it!=entries_.end()&&it->second.ready;}
std::optional<std::vector<hx::PackedTriangle>> PageResidencyManager::get(std::size_t page){std::lock_guard<std::mutex>l(mutex_);auto it=entries_.find(page);if(it==entries_.end()||!it->second.ready)return std::nullopt;it->second.stamp=++stamp_;return it->second.triangles;}
bool PageResidencyManager::resident(std::size_t page)const{std::lock_guard<std::mutex>l(mutex_);auto it=entries_.find(page);return it!=entries_.end()&&it->second.ready;}
std::size_t PageResidencyManager::host_resident_bytes()const{std::lock_guard<std::mutex>l(mutex_);return host_bytes_;}
std::size_t PageResidencyManager::device_resident_bytes()const{std::lock_guard<std::mutex>l(mutex_);return device_bytes_;}
std::size_t PageResidencyManager::resident_pages()const{std::lock_guard<std::mutex>l(mutex_);std::size_t n=0;for(const auto&kv:entries_)if(kv.second.ready)++n;return n;}
std::vector<std::size_t> PageResidencyManager::evict_to_budget_locked(std::optional<std::size_t> protect){
    std::vector<std::size_t> device_evictions;
    while(host_bytes_>host_budget_||(device_budget_>0&&device_bytes_>device_budget_)){
        auto victim=entries_.end();
        for(auto it=entries_.begin();it!=entries_.end();++it){if(!it->second.ready||it->second.loading||(protect&&it->first==*protect))continue;if(victim==entries_.end()||it->second.stamp<victim->second.stamp)victim=it;}
        if(victim==entries_.end())break;
        const auto page=victim->first;host_bytes_-=victim->second.host_bytes;device_bytes_-=victim->second.device_bytes;if(victim->second.device_bytes>0)device_evictions.push_back(page);entries_.erase(victim);
    }
    return device_evictions;
}
void PageResidencyManager::worker_loop(){for(;;){std::size_t page=0;{std::unique_lock<std::mutex>l(mutex_);work_cv_.wait(l,[&]{return stop_||!pending_.empty();});if(stop_&&pending_.empty())return;page=pending_.front();pending_.pop_front();}std::vector<hx::PackedTriangle> tris;std::size_t dev=0;bool ok=true;try{tris=store_->load_page(page);if(upload_&&device_budget_>0)dev=upload_(page,tris);}catch(...){ok=false;}
        std::vector<std::size_t> evicted;
        {std::lock_guard<std::mutex>l(mutex_);auto it=entries_.find(page);if(it==entries_.end())continue;it->second.loading=false;if(ok){it->second.triangles=std::move(tris);it->second.host_bytes=it->second.triangles.size()*sizeof(hx::PackedTriangle);it->second.device_bytes=dev;it->second.ready=true;it->second.stamp=++stamp_;host_bytes_+=it->second.host_bytes;device_bytes_+=dev;evicted=evict_to_budget_locked(page);}else it->second.ready=false;}
        if(evict_)for(auto victim:evicted)evict_(victim);
        cv_.notify_all();
    }
}

void TemporalAA::reset(){history_=Framebuffer{};guides_=ReconstructionGuides{};history_length_.clear();}
Framebuffer TemporalAA::resolve(const Framebuffer& cur,const ReconstructionGuides& g,int maxh,Scalar nt,Scalar dt,Scalar clamp_strength){const std::size_t n=cur.pixels.size();if(cur.width<=0||cur.height<=0||g.width!=cur.width||g.height!=cur.height||g.motion.size()!=n){reset();history_=cur;guides_=g;history_length_.assign(n,1);return cur;}maxh=std::clamp(maxh,1,65535);nt=std::clamp(nt,-1.0f,1.0f);dt=std::max(Scalar(0),dt);clamp_strength=std::max(Scalar(0),clamp_strength);if(history_.width!=cur.width||history_.height!=cur.height||history_.pixels.size()!=n){history_=cur;guides_=g;history_length_.assign(n,1);return cur;}Framebuffer out(cur.width,cur.height);std::vector<std::uint16_t> nextlen(n,1);for(int y=0;y<cur.height;++y)for(int x=0;x<cur.width;++x){const std::size_t i=static_cast<std::size_t>(y*cur.width+x);const Scalar px=x+g.motion[i].x,py=y+g.motion[i].y;bool accept=px>=0&&py>=0&&px<=cur.width-1&&py<=cur.height-1;const int nx=std::clamp(static_cast<int>(std::lround(px)),0,cur.width-1),ny=std::clamp(static_cast<int>(std::lround(py)),0,cur.height-1);const std::size_t pi=static_cast<std::size_t>(ny*cur.width+nx);if(accept&&i<g.normal.size()&&pi<guides_.normal.size()&&g.normal[i].length_squared()>0&&guides_.normal[pi].length_squared()>0)accept=dot(g.normal[i],guides_.normal[pi])>=nt;if(accept&&i<g.depth.size()&&pi<guides_.depth.size()&&g.depth[i]>0&&guides_.depth[pi]>0)accept=std::fabs(g.depth[i]-guides_.depth[pi])/std::max(Scalar(1e-4f),std::max(g.depth[i],guides_.depth[pi]))<=dt;if(!accept){out.pixels[i]=cur.pixels[i];continue;}Color mn{std::numeric_limits<Scalar>::infinity(),std::numeric_limits<Scalar>::infinity(),std::numeric_limits<Scalar>::infinity()},mx{-mn.x,-mn.y,-mn.z},mean{};int cnt=0;for(int oy=-1;oy<=1;++oy)for(int ox=-1;ox<=1;++ox){const int sx=std::clamp(x+ox,0,cur.width-1),sy=std::clamp(y+oy,0,cur.height-1);const Color c=cur.at(sx,sy);mn=min_components(mn,c);mx=max_components(mx,c);mean+=c;++cnt;}mean/=static_cast<Scalar>(cnt);mn=mean+(mn-mean)*clamp_strength;mx=mean+(mx-mean)*clamp_strength;Color hist=bilinear_color(history_,px,py);hist.x=std::clamp(hist.x,mn.x,mx.x);hist.y=std::clamp(hist.y,mn.y,mx.y);hist.z=std::clamp(hist.z,mn.z,mx.z);const int h=std::min(maxh,static_cast<int>(history_length_[pi])+1);const Scalar a=1/static_cast<Scalar>(h);out.pixels[i]=hist*(1-a)+cur.pixels[i]*a;nextlen[i]=static_cast<std::uint16_t>(h);}history_=out;guides_=g;history_length_=std::move(nextlen);return history_;}

Framebuffer TinyNeuralDenoiser::denoise(const Framebuffer& noisy,const Guides& g)const{if(!configured_||noisy.width<=0||noisy.height<=0)return noisy;Framebuffer out(noisy.width,noisy.height);for(int y=0;y<noisy.height;++y)for(int x=0;x<noisy.width;++x){const std::size_t i=static_cast<std::size_t>(y*noisy.width+x);Color mean{};int n=0;for(int oy=-1;oy<=1;++oy)for(int ox=-1;ox<=1;++ox){mean+=noisy.at(std::clamp(x+ox,0,noisy.width-1),std::clamp(y+oy,0,noisy.height-1));++n;}mean/=static_cast<Scalar>(n);const Color a=i<g.albedo.size()?g.albedo[i]:Color{};const Vec3 no=i<g.normal.size()?g.normal[i]:Vec3{};const Scalar d=i<g.depth.size()?g.depth[i]:0;std::array<Scalar,TinyNeuralModel::kInputs> in{{noisy.pixels[i].x,noisy.pixels[i].y,noisy.pixels[i].z,a.x,a.y,a.z,no.x,no.y,no.z,std::log1p(std::max(Scalar(0),d)),lum(mean),1}};std::array<Scalar,TinyNeuralModel::kHidden> h{};for(int j=0;j<TinyNeuralModel::kHidden;++j){Scalar v=model_.b1[static_cast<std::size_t>(j)];for(int k=0;k<TinyNeuralModel::kInputs;++k)v+=model_.w1[static_cast<std::size_t>(j*TinyNeuralModel::kInputs+k)]*in[static_cast<std::size_t>(k)];h[static_cast<std::size_t>(j)]=std::max(Scalar(0),v);}Color residual{};for(int c=0;c<3;++c){Scalar v=model_.b2[static_cast<std::size_t>(c)];for(int j=0;j<TinyNeuralModel::kHidden;++j)v+=model_.w2[static_cast<std::size_t>(c*TinyNeuralModel::kHidden+j)]*h[static_cast<std::size_t>(j)];residual[c]=v;}out.pixels[i]=max_components(Color{},noisy.pixels[i]+residual);}return out;}

FrameBudgetController::FrameBudgetController(FrameBudgetConfig c):config_(c){if(config_.target_ms<=0)config_.target_ms=16.667;if(config_.min_samples<1)config_.min_samples=1;if(config_.max_samples<config_.min_samples)config_.max_samples=config_.min_samples;if(config_.min_bounces<1)config_.min_bounces=1;if(config_.max_bounces<config_.min_bounces)config_.max_bounces=config_.min_bounces;}
void FrameBudgetController::reset(double q){quality_=std::clamp(q,0.0,1.0);integral_=0;previous_error_=0;have_previous_=false;}
FrameBudgetDecision FrameBudgetController::update(double ms){ms=std::max(0.001,ms);const double e=(config_.target_ms-ms)/config_.target_ms;integral_=std::clamp(integral_+e,-2.0,2.0);const double d=have_previous_?e-previous_error_:0;previous_error_=e;have_previous_=true;quality_=std::clamp(quality_+config_.kp*e+config_.ki*integral_+config_.kd*d,0.0,1.0);FrameBudgetDecision o;o.quality_scale=quality_;const double q2=quality_*quality_;o.samples=config_.min_samples+static_cast<int>(std::lround((config_.max_samples-config_.min_samples)*q2));o.max_bounces=config_.min_bounces+static_cast<int>(std::lround((config_.max_bounces-config_.min_bounces)*quality_));o.foveation_strength=std::clamp(config_.max_foveation-static_cast<Scalar>(quality_)*(config_.max_foveation-config_.min_foveation),config_.min_foveation,config_.max_foveation);o.reconstruction_history=config_.min_history+static_cast<int>(std::lround((config_.max_history-config_.min_history)*(1-quality_)));return o;}
void FrameBudgetController::apply(Settings& s,const FrameBudgetDecision& d)const{s.samples_per_pixel=std::max(1,d.samples);s.max_bounces=std::max(1,d.max_bounces);s.variable_rate_sampling=d.foveation_strength>0;s.foveation_strength=std::clamp(d.foveation_strength,Scalar(0),Scalar(0.95f));}

ImageQualityMetrics compare_images(const Framebuffer& a,const Framebuffer& b){if(a.width!=b.width||a.height!=b.height||a.pixels.size()!=b.pixels.size()||a.pixels.empty())throw std::invalid_argument("image dimensions differ");double mse=0,ma=0,mb=0;const std::size_t n=a.pixels.size();std::vector<double> la(n),lb(n);for(std::size_t i=0;i<n;++i){const Color d=a.pixels[i]-b.pixels[i];mse+=(static_cast<double>(d.x)*d.x+static_cast<double>(d.y)*d.y+static_cast<double>(d.z)*d.z)/3.0;la[i]=lum(a.pixels[i]);lb[i]=lum(b.pixels[i]);ma+=la[i];mb+=lb[i];}mse/=n;ma/=n;mb/=n;double va=0,vb=0,cov=0;for(std::size_t i=0;i<n;++i){const double da=la[i]-ma,db=lb[i]-mb;va+=da*da;vb+=db*db;cov+=da*db;}const double den=std::max<std::size_t>(1,n-1);va/=den;vb/=den;cov/=den;constexpr double C1=0.01*0.01,C2=0.03*0.03;const double ssim=((2*ma*mb+C1)*(2*cov+C2))/((ma*ma+mb*mb+C1)*(va+vb+C2));ImageQualityMetrics m;m.mse=mse;m.psnr_db=mse<=1e-30?std::numeric_limits<double>::infinity():10*std::log10(1.0/mse);m.ssim=std::clamp(ssim,-1.0,1.0);return m;}
std::optional<double> read_linux_rapl_joules(){
#if defined(__linux__)
    namespace fs=std::filesystem;
    const fs::path root{"/sys/class/powercap"};
    std::error_code ec;
    if(!fs::exists(root,ec)) return std::nullopt;
    double sum=0; bool any=false;
    for(auto it=fs::recursive_directory_iterator(root,fs::directory_options::skip_permission_denied,ec); it!=fs::recursive_directory_iterator(); it.increment(ec)){
        if(ec){ec.clear();continue;}
        if(it->path().filename()=="energy_uj"){
            std::ifstream in(it->path()); double uj=0;
            if(in>>uj){sum+=uj*1e-6;any=true;}
        }
    }
    if(any) return sum;
#endif
    return std::nullopt;
}

std::size_t process_resident_bytes(){
#if defined(__linux__)
    std::ifstream in("/proc/self/statm");
    std::size_t total=0,res=0;
    if(in>>total>>res){const long p=sysconf(_SC_PAGESIZE);if(p>0)return res*static_cast<std::size_t>(p);}
#endif
    return 0;
}

PricePerformanceRecord make_price_performance_record(std::string device,NativeBackend b,double price,double ms,double joules,std::size_t bytes,const Framebuffer& img,const Framebuffer& ref){PricePerformanceRecord r;r.device=std::move(device);r.backend=b;r.purchase_price_usd=std::max(0.0,price);r.frame_ms=std::max(0.0,ms);r.energy_joules=std::max(0.0,joules);r.resident_bytes=bytes;r.quality=compare_images(img,ref);const double fps=r.frame_ms>0?1000/r.frame_ms:0;r.fps_per_dollar=r.purchase_price_usd>0?fps/r.purchase_price_usd:0;r.quality_per_joule=r.energy_joules>0?std::max(0.0,r.quality.ssim)/r.energy_joules:0;return r;}
std::string price_performance_csv_header(){return "device,backend,price_usd,frame_ms,energy_joules,resident_bytes,mse,psnr_db,ssim,fps_per_dollar,quality_per_joule";}
std::string price_performance_csv_row(const PricePerformanceRecord&r){std::ostringstream o;o<<std::quoted(r.device)<<','<<backend_name(r.backend)<<','<<r.purchase_price_usd<<','<<r.frame_ms<<','<<r.energy_joules<<','<<r.resident_bytes<<','<<r.quality.mse<<','<<r.quality.psnr_db<<','<<r.quality.ssim<<','<<r.fps_per_dollar<<','<<r.quality_per_joule;return o.str();}

} // namespace beamcast::gx
