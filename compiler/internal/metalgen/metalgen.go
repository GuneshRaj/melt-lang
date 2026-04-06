package metalgen

import "strings"

func GenerateMapScaleKernel() string {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\n")
	b.WriteString("using namespace metal;\n\n")
	b.WriteString("struct MapScaleF32Params {\n")
	b.WriteString("    ulong length;\n")
	b.WriteString("    float factor;\n")
	b.WriteString("};\n\n")
	b.WriteString("kernel void map_scale_f32(\n")
	b.WriteString("    device const float* in0 [[buffer(0)]],\n")
	b.WriteString("    device float* out0 [[buffer(1)]],\n")
	b.WriteString("    constant MapScaleF32Params* params [[buffer(2)]],\n")
	b.WriteString("    uint gid [[thread_position_in_grid]]\n")
	b.WriteString(") {\n")
	b.WriteString("    if (gid >= params->length) return;\n")
	b.WriteString("    out0[gid] = in0[gid] * params->factor;\n")
	b.WriteString("}\n")
	return b.String()
}
