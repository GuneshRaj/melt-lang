import Foundation
import Metal

public struct MeltKernelMetrics {
    public let setupMs: Double
    public let dispatchMs: Double
    public let readbackMs: Double
}

public final class MeltMetal {
    private let device: MTLDevice
    private let queue: MTLCommandQueue
    private var pipelineCache: [String: MTLComputePipelineState] = [:]

    public init() throws {
        guard let d = MTLCreateSystemDefaultDevice(), let q = d.makeCommandQueue() else {
            throw MeltSupportError.metalUnavailable
        }
        self.device = d
        self.queue = q
    }

    public func mapScaleF32(_ input: [Float], factor: Float, metallibPath: String) throws -> [Float] {
        let (result, _) = try mapScaleF32Timed(input, factor: factor, metallibPath: metallibPath)
        return result
    }

    public func mapScaleF32Timed(_ input: [Float], factor: Float, metallibPath: String) throws -> ([Float], MeltKernelMetrics) {
        let setupStart = CFAbsoluteTimeGetCurrent()
        let library = try device.makeLibrary(URL: URL(fileURLWithPath: metallibPath))
        let functionName = "map_scale_f32"
        let pipeline = try pipelineState(named: functionName, library: library)

        let count = input.count
        var output = Array(repeating: Float(0), count: count)
        var params = MapScaleF32Params(length: UInt64(count), factor: factor)

        guard
            let inBuffer = device.makeBuffer(bytes: input, length: MemoryLayout<Float>.stride * count),
            let outBuffer = device.makeBuffer(bytes: &output, length: MemoryLayout<Float>.stride * count),
            let paramsBuffer = device.makeBuffer(bytes: &params, length: MemoryLayout<MapScaleF32Params>.stride)
        else {
            throw MeltSupportError.kernelFailure("failed to allocate Metal buffers")
        }
        let setupMs = (CFAbsoluteTimeGetCurrent() - setupStart) * 1000.0

        guard let cmd = queue.makeCommandBuffer(), let encoder = cmd.makeComputeCommandEncoder() else {
            throw MeltSupportError.kernelFailure("failed to create command buffer")
        }
        encoder.setComputePipelineState(pipeline)
        encoder.setBuffer(inBuffer, offset: 0, index: 0)
        encoder.setBuffer(outBuffer, offset: 0, index: 1)
        encoder.setBuffer(paramsBuffer, offset: 0, index: 2)

        let width = min(pipeline.threadExecutionWidth, max(1, count))
        let threadsPerThreadgroup = MTLSize(width: width, height: 1, depth: 1)
        let threadsPerGrid = MTLSize(width: count, height: 1, depth: 1)
        let dispatchStart = CFAbsoluteTimeGetCurrent()
        encoder.dispatchThreads(threadsPerGrid, threadsPerThreadgroup: threadsPerThreadgroup)
        encoder.endEncoding()
        cmd.commit()
        cmd.waitUntilCompleted()
        let dispatchMs = (CFAbsoluteTimeGetCurrent() - dispatchStart) * 1000.0

        if let err = cmd.error {
            throw MeltSupportError.kernelFailure(err.localizedDescription)
        }

        let readbackStart = CFAbsoluteTimeGetCurrent()
        let ptr = outBuffer.contents().bindMemory(to: Float.self, capacity: count)
        let result = Array(UnsafeBufferPointer(start: ptr, count: count))
        let readbackMs = (CFAbsoluteTimeGetCurrent() - readbackStart) * 1000.0
        return (result, MeltKernelMetrics(setupMs: setupMs, dispatchMs: dispatchMs, readbackMs: readbackMs))
    }

    public func mapScaleF64(_ input: [Double], factor: Double, metallibPath: String) throws -> [Double] {
        let result = try mapScaleF32(input.map(Float.init), factor: Float(factor), metallibPath: metallibPath)
        return result.map(Double.init)
    }

    public func zipAddF64(_ a: [Double], _ b: [Double], metallibPath: String) throws -> [Double] {
        throw MeltSupportError.kernelFailure("Metal zipAddF64 not implemented yet")
    }

    public func reduceSumF64(_ input: [Double], metallibPath: String) throws -> Double {
        throw MeltSupportError.kernelFailure("Metal reduceSumF64 not implemented yet")
    }

    public func movingMeanF64(_ input: [Double], window: Int, metallibPath: String) throws -> [Double] {
        throw MeltSupportError.kernelFailure("Metal movingMeanF64 not implemented yet")
    }

    private func pipelineState(named name: String, library: MTLLibrary) throws -> MTLComputePipelineState {
        if let cached = pipelineCache[name] {
            return cached
        }
        guard let fn = library.makeFunction(name: name) else {
            throw MeltSupportError.kernelFailure("missing Metal function \(name)")
        }
        let pipeline = try device.makeComputePipelineState(function: fn)
        pipelineCache[name] = pipeline
        return pipeline
    }
}

private struct MapScaleF32Params {
    var length: UInt64
    var factor: Float
}
