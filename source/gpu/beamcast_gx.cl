// Beamcast GX OpenCL 1.2 reference wavefront kernels.
// The ABI mirrors beamcast::hx::GpuSceneData. Host-side queue compaction can use
// any prefix-sum implementation; each kernel is deliberately vendor-neutral.

#define GX_PI 3.14159265358979323846f
#define GX_EPS 1.0e-4f

typedef struct {
    float4 bmin;
    float4 bmax;
    uint left, right, first, count, leaf, axis;
    uint _pad0, _pad1; // C++ GpuBvhNode is 64 bytes
} GpuBvhNode;

typedef struct { uint index, type; } GpuPrimitiveRef;

typedef struct {
    float4 center;
    float radius;
    uint material;
    uint _pad0, _pad1;
} PackedSphere;

typedef struct {
    float4 v0, e1, e2, normal;
    uint material;
    uint _pad0, _pad1, _pad2;
} PackedTriangle;

typedef struct {
    uchar type;
    uchar _pad0[15];
    float4 albedo;
    float4 emission;
    float roughness;
    float ior;
    float _pad1[2];
} PackedMaterial;

typedef struct {
    float4 origin;
    float4 direction;
    float4 beta;
    float4 radiance;
    ulong rng;
    uint pixel;
    uint alive;
    uint bounce;
    uint _pad0;
} GxPath;

typedef struct {
    float t;
    float4 p;
    float4 normal;
    uint material;
    uint primitive_index;
    uint primitive_type;
    uint front_face;
} GxHit;

inline uint gx_rng_u32(__private ulong* state) {
    ulong x=*state ? *state : 1UL;
    x^=x>>12; x^=x<<25; x^=x>>27; *state=x;
    return (uint)((x*2685821657736338717UL)>>32);
}
inline float gx_rng(__private ulong* state) { return (float)gx_rng_u32(state)*(1.0f/4294967296.0f); }
inline float3 gx_unit(float3 v) { float l2=dot(v,v); return l2>0 ? v*rsqrt(l2) : (float3)(0); }
inline float3 gx_reflect(float3 v,float3 n){return v-2.0f*dot(v,n)*n;}
inline float3 gx_refract(float3 uv,float3 n,float eta){
    float ct=fmin(dot(-uv,n),1.0f); float3 p=eta*(uv+ct*n);
    float3 q=-sqrt(fabs(1.0f-dot(p,p)))*n; return p+q;
}
inline float gx_schlick(float c,float eta){float r=(1.0f-eta)/(1.0f+eta);r*=r;return r+(1.0f-r)*pown(1.0f-c,5);}
inline float3 gx_cosine(float3 n,__private ulong* rng){
    float r1=2.0f*GX_PI*gx_rng(rng),r2=gx_rng(rng),s=sqrt(r2);
    float3 a=fabs(n.x)>0.1f?(float3)(0,1,0):(float3)(1,0,0);
    float3 v=gx_unit(cross(n,a)),u=cross(v,n);
    return gx_unit(u*(cos(r1)*s)+v*(sin(r1)*s)+n*sqrt(fmax(0.0f,1.0f-r2)));
}
inline float3 gx_rand_sphere(__private ulong* rng){
    for(int i=0;i<16;++i){float3 p=(float3)(2*gx_rng(rng)-1,2*gx_rng(rng)-1,2*gx_rng(rng)-1);if(dot(p,p)<1)return p;}
    return (float3)(0);
}

inline int gx_box_hit(const GpuBvhNode n,float3 o,float3 inv,float tmin,float tmax,__private float* enter){
    float3 a=(n.bmin.xyz-o)*inv,b=(n.bmax.xyz-o)*inv;
    float3 lo=fmin(a,b),hi=fmax(a,b);
    float e=fmax(tmin,fmax(lo.x,fmax(lo.y,lo.z)));
    float x=fmin(tmax,fmin(hi.x,fmin(hi.y,hi.z)));
    *enter=e; return x>=e;
}

inline int gx_sphere_hit(PackedSphere s,float3 o,float3 d,float tmin,float tmax,__private GxHit* hit,uint index){
    float3 oc=o-s.center.xyz; float a=dot(d,d),hb=dot(oc,d),c=dot(oc,oc)-s.radius*s.radius;
    float disc=hb*hb-a*c;if(disc<0)return 0;float root=(-hb-sqrt(disc))/a;
    if(root<=tmin||root>=tmax){root=(-hb+sqrt(disc))/a;if(root<=tmin||root>=tmax)return 0;}
    float3 p=o+root*d,n=(p-s.center.xyz)/s.radius;uint ff=dot(d,n)<0;
    hit->t=root;hit->p=(float4)(p,0);hit->normal=(float4)(ff?n:-n,0);hit->material=s.material;
    hit->primitive_index=index;hit->primitive_type=0;hit->front_face=ff;return 1;
}
inline int gx_triangle_hit(PackedTriangle t,float3 o,float3 d,float tmin,float tmax,__private GxHit* hit,uint index){
    float3 p=cross(d,t.e2.xyz);float det=dot(t.e1.xyz,p);if(fabs(det)<1e-8f)return 0;float inv=1.0f/det;
    float3 s=o-t.v0.xyz;float u=dot(s,p)*inv;if(u<0||u>1)return 0;float3 q=cross(s,t.e1.xyz);float v=dot(d,q)*inv;
    if(v<0||u+v>1)return 0;float tt=dot(t.e2.xyz,q)*inv;if(tt<=tmin||tt>=tmax)return 0;
    uint ff=dot(d,t.normal.xyz)<0;hit->t=tt;hit->p=(float4)(o+tt*d,0);hit->normal=(float4)(ff?t.normal.xyz:-t.normal.xyz,0);
    hit->material=t.material;hit->primitive_index=index;hit->primitive_type=1;hit->front_face=ff;return 1;
}

inline GxHit gx_trace_closest(__global const GpuBvhNode* nodes,__global const GpuPrimitiveRef* refs,
                             __global const PackedSphere* spheres,__global const PackedTriangle* triangles,
                             float3 o,float3 d){
    GxHit best;best.t=INFINITY;best.material=0;best.primitive_index=0;best.primitive_type=0;best.front_face=0;
    float3 inv=(float3)(fabs(d.x)>1e-20f?1.0f/d.x:copysign(1e30f,d.x),fabs(d.y)>1e-20f?1.0f/d.y:copysign(1e30f,d.y),fabs(d.z)>1e-20f?1.0f/d.z:copysign(1e30f,d.z));
    uint stack[96];float enters[96];int sp=0;float e;
    if(!gx_box_hit(nodes[0],o,inv,GX_EPS,best.t,&e))return best;stack[sp]=0;enters[sp++]=e;
    while(sp){--sp;uint ni=stack[sp];if(enters[sp]>best.t)continue;GpuBvhNode n=nodes[ni];
        if(n.leaf){for(uint j=0;j<n.count;++j){GpuPrimitiveRef pr=refs[n.first+j];GxHit h;int ok=pr.type==0?gx_sphere_hit(spheres[pr.index],o,d,GX_EPS,best.t,&h,pr.index):gx_triangle_hit(triangles[pr.index],o,d,GX_EPS,best.t,&h,pr.index);if(ok)best=h;}continue;}
        float le,re;int lh=gx_box_hit(nodes[n.left],o,inv,GX_EPS,best.t,&le),rh=gx_box_hit(nodes[n.right],o,inv,GX_EPS,best.t,&re);
        if(lh&&rh){uint nearN=le<re?n.left:n.right,farN=le<re?n.right:n.left;float nearE=fmin(le,re),farE=fmax(le,re);if(sp<95){stack[sp]=farN;enters[sp++]=farE;}if(sp<96){stack[sp]=nearN;enters[sp++]=nearE;}}
        else if(lh&&sp<96){stack[sp]=n.left;enters[sp++]=le;}else if(rh&&sp<96){stack[sp]=n.right;enters[sp++]=re;}
    }
    return best;
}

__kernel void gx_intersect(__global const GpuBvhNode* nodes,
                           __global const GpuPrimitiveRef* refs,
                           __global const PackedSphere* spheres,
                           __global const PackedTriangle* triangles,
                           __global const GxPath* paths,
                           __global GxHit* hits,
                           uint path_count){
    uint id=get_global_id(0);if(id>=path_count||!paths[id].alive)return;
    hits[id]=gx_trace_closest(nodes,refs,spheres,triangles,paths[id].origin.xyz,paths[id].direction.xyz);
}

__kernel void gx_shade(__global const PackedMaterial* materials,
                       __global GxPath* paths,
                       __global const GxHit* hits,
                       uint path_count,
                       uint max_bounces,
                       float4 background_bottom,
                       float4 background_top){
    uint id=get_global_id(0);if(id>=path_count||!paths[id].alive)return;
    GxPath p=paths[id];GxHit h=hits[id];
    if(!isfinite(h.t)){
        float3 d=gx_unit(p.direction.xyz);float t=0.5f*(d.y+1.0f);float3 bg=mix(background_bottom.xyz,background_top.xyz,t);
        p.radiance.xyz+=p.beta.xyz*bg;p.alive=0;paths[id]=p;return;
    }
    PackedMaterial m=materials[h.material];
    if(m.type==3){p.radiance.xyz+=p.beta.xyz*m.emission.xyz;p.alive=0;paths[id]=p;return;}
    float3 dir;float3 att=m.albedo.xyz;float3 incoming=gx_unit(p.direction.xyz);
    if(m.type==0)dir=gx_cosine(h.normal.xyz,&p.rng);
    else if(m.type==1){dir=gx_reflect(incoming,h.normal.xyz)+m.roughness*gx_rand_sphere(&p.rng);if(dot(dir,h.normal.xyz)<=0){p.alive=0;paths[id]=p;return;}}
    else {att=(float3)(1);float eta=h.front_face?1.0f/m.ior:m.ior;float ct=fmin(dot(-incoming,h.normal.xyz),1.0f);float st=sqrt(fmax(0.0f,1.0f-ct*ct));dir=(eta*st>1.0f||gx_schlick(ct,eta)>gx_rng(&p.rng))?gx_reflect(incoming,h.normal.xyz):gx_refract(incoming,h.normal.xyz,eta);}
    p.beta.xyz*=att;p.origin=(float4)(h.p.xyz+h.normal.xyz*(dot(dir,h.normal.xyz)>=0?GX_EPS:-GX_EPS),0);p.direction=(float4)(dir,0);++p.bounce;
    if(p.bounce>=max_bounces)p.alive=0;paths[id]=p;
}

__kernel void gx_mark_alive(__global const GxPath* paths,__global uint* flags,uint path_count){
    uint id=get_global_id(0);if(id<path_count)flags[id]=paths[id].alive?1u:0u;
}

typedef struct {
    ushort bmin[3];
    ushort bmax[3];
    uint left, right, first, count, leaf, axis;
} QuantizedGpuBvhNode;

inline int gx_qbox_hit(QuantizedGpuBvhNode n,float3 world_min,float3 world_extent,
                       float3 o,float3 inv,float tmin,float tmax,__private float* enter){
    const float s=1.0f/65535.0f;
    float3 mn=world_min+world_extent*(float3)((float)n.bmin[0]*s,(float)n.bmin[1]*s,(float)n.bmin[2]*s);
    float3 mx=world_min+world_extent*(float3)((float)n.bmax[0]*s,(float)n.bmax[1]*s,(float)n.bmax[2]*s);
    float3 a=(mn-o)*inv,b=(mx-o)*inv,lo=fmin(a,b),hi=fmax(a,b);
    float e=fmax(tmin,fmax(lo.x,fmax(lo.y,lo.z))),x=fmin(tmax,fmin(hi.x,fmin(hi.y,hi.z)));
    *enter=e;return x>=e;
}

inline GxHit gx_trace_closest_q(__global const QuantizedGpuBvhNode* nodes,__global const GpuPrimitiveRef* refs,
                               __global const PackedSphere* spheres,__global const PackedTriangle* triangles,
                               float3 world_min,float3 world_extent,float3 o,float3 d){
    GxHit best;best.t=INFINITY;best.material=0;best.primitive_index=0;best.primitive_type=0;best.front_face=0;
    float3 inv=(float3)(fabs(d.x)>1e-20f?1.0f/d.x:copysign(1e30f,d.x),fabs(d.y)>1e-20f?1.0f/d.y:copysign(1e30f,d.y),fabs(d.z)>1e-20f?1.0f/d.z:copysign(1e30f,d.z));
    uint stack[96];float enters[96];int sp=0;float e;
    if(!gx_qbox_hit(nodes[0],world_min,world_extent,o,inv,GX_EPS,best.t,&e))return best;stack[sp]=0;enters[sp++]=e;
    while(sp){--sp;uint ni=stack[sp];if(enters[sp]>best.t)continue;QuantizedGpuBvhNode n=nodes[ni];
        if(n.leaf){for(uint j=0;j<n.count;++j){GpuPrimitiveRef pr=refs[n.first+j];GxHit h;int ok=pr.type==0?gx_sphere_hit(spheres[pr.index],o,d,GX_EPS,best.t,&h,pr.index):gx_triangle_hit(triangles[pr.index],o,d,GX_EPS,best.t,&h,pr.index);if(ok)best=h;}continue;}
        float le,re;int lh=gx_qbox_hit(nodes[n.left],world_min,world_extent,o,inv,GX_EPS,best.t,&le),rh=gx_qbox_hit(nodes[n.right],world_min,world_extent,o,inv,GX_EPS,best.t,&re);
        if(lh&&rh){uint nearN=le<re?n.left:n.right,farN=le<re?n.right:n.left;float nearE=fmin(le,re),farE=fmax(le,re);if(sp<95){stack[sp]=farN;enters[sp++]=farE;}if(sp<96){stack[sp]=nearN;enters[sp++]=nearE;}}
        else if(lh&&sp<96){stack[sp]=n.left;enters[sp++]=le;}else if(rh&&sp<96){stack[sp]=n.right;enters[sp++]=re;}
    }
    return best;
}

__kernel void gx_intersect_quantized(__global const QuantizedGpuBvhNode* nodes,
                                     __global const GpuPrimitiveRef* refs,
                                     __global const PackedSphere* spheres,
                                     __global const PackedTriangle* triangles,
                                     float4 world_min,float4 world_extent,
                                     __global const GxPath* paths,
                                     __global GxHit* hits,
                                     uint path_count){
    uint id=get_global_id(0);if(id>=path_count||!paths[id].alive)return;
    hits[id]=gx_trace_closest_q(nodes,refs,spheres,triangles,world_min.xyz,world_extent.xyz,paths[id].origin.xyz,paths[id].direction.xyz);
}

// ---------------- Device queue compaction ----------------
// Three-stage exclusive scan/scatter. gx_scan_block writes one sum per work-group;
// recursively scan block_sums, then gx_add_block_offsets and gx_scatter_alive.
// Local size must be GX_SCAN_WG (256) for this reference implementation.
#define GX_SCAN_WG 256

__kernel __attribute__((reqd_work_group_size(GX_SCAN_WG,1,1)))
void gx_scan_block(__global const uint* flags,
                   __global uint* prefix,
                   __global uint* block_sums,
                   uint count) {
    __local uint temp[GX_SCAN_WG];
    const uint lid=get_local_id(0);
    const uint gid=get_global_id(0);
    const uint group=get_group_id(0);
    temp[lid]=gid<count ? (flags[gid] ? 1u : 0u) : 0u;
    barrier(CLK_LOCAL_MEM_FENCE);

    // Hillis-Steele inclusive scan in local memory, then convert to exclusive.
    for(uint offset=1; offset<GX_SCAN_WG; offset<<=1) {
        const uint v=lid>=offset ? temp[lid-offset] : 0u;
        barrier(CLK_LOCAL_MEM_FENCE);
        const uint cur=temp[lid];
        barrier(CLK_LOCAL_MEM_FENCE);
        temp[lid]=cur+v;
        barrier(CLK_LOCAL_MEM_FENCE);
    }
    const uint inclusive=temp[lid];
    if(gid<count) prefix[gid]=inclusive-(flags[gid]?1u:0u);
    if(lid==GX_SCAN_WG-1) block_sums[group]=inclusive;
}

__kernel void gx_add_block_offsets(__global uint* prefix,
                                   __global const uint* scanned_block_sums,
                                   uint count) {
    const uint gid=get_global_id(0);
    if(gid>=count) return;
    const uint group=gid/GX_SCAN_WG;
    if(group>0) prefix[gid]+=scanned_block_sums[group];
}

__kernel void gx_scatter_alive(__global const uint* flags,
                               __global const uint* prefix,
                               __global uint* compacted_indices,
                               uint count) {
    const uint gid=get_global_id(0);
    if(gid<count && flags[gid]) compacted_indices[prefix[gid]]=gid;
}

// ---------------- Quantized BVH8 reference traversal ----------------
// Host ABI: beamcast::gx::WideBvh8Node, 160 bytes.
typedef struct {
    ushort bmin[3][8];
    ushort bmax[3][8];
    uint index[8];
    ushort count[8];
    uchar kind[8];
    uchar child_count;
    uchar pad[7];
} GxWideBvh8Node;

inline int gx_wide_box_hit(__global const GxWideBvh8Node* nodes,uint node_id,uint child,
                           float3 world_min,float3 world_extent,float3 o,float3 inv,
                           float tmin,float tmax,__private float* enter){
    const float s=1.0f/65535.0f;
    float3 mn=world_min+world_extent*(float3)(
        (float)nodes[node_id].bmin[0][child]*s,
        (float)nodes[node_id].bmin[1][child]*s,
        (float)nodes[node_id].bmin[2][child]*s);
    float3 mx=world_min+world_extent*(float3)(
        (float)nodes[node_id].bmax[0][child]*s,
        (float)nodes[node_id].bmax[1][child]*s,
        (float)nodes[node_id].bmax[2][child]*s);
    float3 a=(mn-o)*inv,b=(mx-o)*inv,lo=fmin(a,b),hi=fmax(a,b);
    float e=fmax(tmin,fmax(lo.x,fmax(lo.y,lo.z))),x=fmin(tmax,fmin(hi.x,fmin(hi.y,hi.z)));
    *enter=e;return x>=e;
}

inline GxHit gx_trace_closest_wide(__global const GxWideBvh8Node* nodes,
                                   __global const GpuPrimitiveRef* refs,
                                   __global const PackedSphere* spheres,
                                   __global const PackedTriangle* triangles,
                                   float3 world_min,float3 world_extent,float3 o,float3 d){
    GxHit best;best.t=INFINITY;best.material=0;best.primitive_index=0;best.primitive_type=0;best.front_face=0;
    float3 inv=(float3)(fabs(d.x)>1e-20f?1.0f/d.x:copysign(1e30f,d.x),fabs(d.y)>1e-20f?1.0f/d.y:copysign(1e30f,d.y),fabs(d.z)>1e-20f?1.0f/d.z:copysign(1e30f,d.z));
    uint stack[128];int sp=0;stack[sp++]=0;
    while(sp){
        uint ni=stack[--sp];GxWideBvh8Node n=nodes[ni];
        for(uint c=0;c<(uint)n.child_count&&c<8u;++c){
            if(n.kind[c]==0)continue;float enter;
            if(!gx_wide_box_hit(nodes,ni,c,world_min,world_extent,o,inv,GX_EPS,best.t,&enter))continue;
            if(n.kind[c]==2){
                for(uint j=0;j<(uint)n.count[c];++j){GpuPrimitiveRef pr=refs[n.index[c]+j];GxHit h;int ok=pr.type==0?gx_sphere_hit(spheres[pr.index],o,d,GX_EPS,best.t,&h,pr.index):gx_triangle_hit(triangles[pr.index],o,d,GX_EPS,best.t,&h,pr.index);if(ok)best=h;}
            }else if(sp<128)stack[sp++]=n.index[c];
        }
    }
    return best;
}

__kernel void gx_intersect_wide(__global const GxWideBvh8Node* nodes,
                                __global const GpuPrimitiveRef* refs,
                                __global const PackedSphere* spheres,
                                __global const PackedTriangle* triangles,
                                float4 world_min,float4 world_extent,
                                __global const GxPath* paths,
                                __global GxHit* hits,
                                uint path_count){
    uint id=get_global_id(0);if(id>=path_count||!paths[id].alive)return;
    hits[id]=gx_trace_closest_wide(nodes,refs,spheres,triangles,world_min.xyz,world_extent.xyz,paths[id].origin.xyz,paths[id].direction.xyz);
}
