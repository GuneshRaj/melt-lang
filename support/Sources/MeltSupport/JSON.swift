import Foundation

public final class MeltJSON {
    public static func load<T: Decodable>(_ path: String, as: T.Type) throws -> T {
        let data = try Data(contentsOf: URL(fileURLWithPath: path))
        return try JSONDecoder().decode(T.self, from: data)
    }

    public static func save<T: Encodable>(_ path: String, _ value: T) throws {
        let data = try JSONEncoder().encode(value)
        try data.write(to: URL(fileURLWithPath: path))
    }
}
