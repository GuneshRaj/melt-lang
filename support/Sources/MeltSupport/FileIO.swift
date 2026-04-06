import Foundation

public final class MeltFile {
    public static func readText(_ path: String) throws -> String {
        guard FileManager.default.fileExists(atPath: path) else {
            throw MeltSupportError.fileNotFound(path)
        }
        return try String(contentsOfFile: path, encoding: .utf8)
    }

    public static func writeText(_ path: String, _ value: String) throws {
        try value.write(toFile: path, atomically: true, encoding: .utf8)
    }
}
