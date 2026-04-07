import Foundation

public final class MeltParquet {
    public static func loadFloat32Column(_ path: String, column: String) throws -> [Float] {
        throw MeltSupportError.parquetDecodeError("Parquet support is declared in the language but not implemented in the Swift runtime yet")
    }

    public static func loadInt32Column(_ path: String, column: String) throws -> [Int32] {
        throw MeltSupportError.parquetDecodeError("Parquet support is declared in the language but not implemented in the Swift runtime yet")
    }
}
