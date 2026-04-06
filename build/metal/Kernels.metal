#include <metal_stdlib>
using namespace metal;

struct MapScaleF32Params {
    ulong length;
    float factor;
};

kernel void map_scale_f32(
    device const float* in0 [[buffer(0)]],
    device float* out0 [[buffer(1)]],
    constant MapScaleF32Params* params [[buffer(2)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= params->length) return;
    out0[gid] = in0[gid] * params->factor;
}
