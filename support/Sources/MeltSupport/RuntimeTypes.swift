import Foundation

public enum MeltSupportError: Error {
    case fileNotFound(String)
    case csvDecodeError(String)
    case parquetDecodeError(String)
    case jsonDecodeError(String)
    case metalUnavailable
    case kernelFailure(String)
}

public enum MeltValue {
    case int32(Int32)
    case int64(Int64)
    case float32(Float)
    case float64(Double)
    case bool(Bool)
    case string(String)
    case int32Array([Int32])
    case int64Array([Int64])
    case float32Array([Float])
    case float64Array([Double])
}
