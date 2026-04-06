import Foundation

struct PriceRow: Decodable {
    let ts: Int64
    let close: Float
}

@main
struct MeltProgram {
    static func main() throws {
        let metal = try MeltMetal()
        let exeDir = URL(fileURLWithPath: CommandLine.arguments[0]).deletingLastPathComponent().path
        let metallibPath = exeDir + "/default.metallib"
        let __loadStart = CFAbsoluteTimeGetCurrent()
        let rows = try MeltCSV.loadRows("data/prices_10k.csv", as: PriceRow.self)
        let close = rows.map { $0.close }
        let __tmp1 = Float(1.1)
        let __loadMs = (CFAbsoluteTimeGetCurrent() - __loadStart) * 1000.0
        let __computeStart = CFAbsoluteTimeGetCurrent()
        let (out, __gpuMetrics) = try metal.mapScaleF32Timed(close, factor: Float(__tmp1), metallibPath: metallibPath)
        let __computeMs = __gpuMetrics.dispatchMs
        let __saveStart = CFAbsoluteTimeGetCurrent()
        try MeltCSV.saveFloat32Array("build/out_gpu_scale_10k_f32.csv", out)
        let __saveMs = (CFAbsoluteTimeGetCurrent() - __saveStart) * 1000.0
        let __totalMs = __loadMs + __computeMs + __saveMs
        print("load_ms: \(__loadMs)")
        print("compute_ms: \(__computeMs)")
        print("gpu_setup_ms: \(__gpuMetrics.setupMs)")
        print("gpu_readback_ms: \(__gpuMetrics.readbackMs)")
        print("save_ms: \(__saveMs)")
        print("total_ms: \(__totalMs)")
    }
}
