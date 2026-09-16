#include <cuda_runtime.h>
#include <cstdint>
namespace beamcast_gx_cuda {
__global__ void mark_alive(const std::uint32_t* alive,std::uint32_t* flags,std::uint32_t n){auto i=blockIdx.x*blockDim.x+threadIdx.x;if(i<n)flags[i]=alive[i]?1u:0u;}
__global__ void scatter_alive(const std::uint32_t* flags,const std::uint32_t* prefix,std::uint32_t* out,std::uint32_t n){auto i=blockIdx.x*blockDim.x+threadIdx.x;if(i<n&&flags[i])out[prefix[i]]=i;}
// Runtime uses CUB/DeviceScan when available; these kernels keep the scatter ABI independent of CUB.
}
