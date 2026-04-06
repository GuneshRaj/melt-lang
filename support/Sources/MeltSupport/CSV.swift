import Foundation

public final class MeltCSV {
    public static func loadRows<T: Decodable>(_ path: String, as: T.Type) throws -> [T] {
        let text = try MeltFile.readText(path)
        let lines = text.split(whereSeparator: \.isNewline)
        guard let headerLine = lines.first else { return [] }
        let headers = headerLine.split(separator: ",").map(String.init)
        let decoder = JSONDecoder()
        return try lines.dropFirst().map { line in
            let cols = line.split(separator: ",", omittingEmptySubsequences: false).map(String.init)
            guard cols.count == headers.count else {
                throw MeltSupportError.csvDecodeError("column count mismatch in \(path)")
            }
            var obj: [String: Any] = [:]
            for (idx, header) in headers.enumerated() {
                let raw = cols[idx]
                if let intValue = Int64(raw), !raw.contains(".") {
                    obj[header] = intValue
                } else if let doubleValue = Double(raw) {
                    obj[header] = doubleValue
                } else {
                    obj[header] = raw
                }
            }
            let data = try JSONSerialization.data(withJSONObject: obj)
            return try decoder.decode(T.self, from: data)
        }
    }

    public static func loadFloat64Array(_ path: String) throws -> [Double] {
        let text = try MeltFile.readText(path)
        let rows = text.split(whereSeparator: \.isNewline)
        if rows.isEmpty { return [] }
        if rows[0].contains(",") {
            let body = rows.dropFirst()
            return try body.map { row in
                let cols = row.split(separator: ",", omittingEmptySubsequences: false)
                guard let first = cols.first, let value = Double(first) else {
                    throw MeltSupportError.csvDecodeError("invalid numeric row in \(path)")
                }
                return value
            }
        }
        return try rows.map {
            guard let value = Double($0) else {
                throw MeltSupportError.csvDecodeError("invalid numeric row in \(path)")
            }
            return value
        }
    }

    public static func loadFloat32Array(_ path: String) throws -> [Float] {
        return try loadFloat64Array(path).map(Float.init)
    }

    public static func saveFloat64Array(_ path: String, _ values: [Double]) throws {
        let text = values.map { String($0) }.joined(separator: "\n")
        try text.write(toFile: path, atomically: true, encoding: String.Encoding.utf8)
    }

    public static func saveFloat32Array(_ path: String, _ values: [Float]) throws {
        let text = values.map { String($0) }.joined(separator: "\n")
        try text.write(toFile: path, atomically: true, encoding: String.Encoding.utf8)
    }

    public static func saveInt64Array(_ path: String, _ values: [Int64]) throws {
        let text = values.map { String($0) }.joined(separator: "\n")
        try text.write(toFile: path, atomically: true, encoding: String.Encoding.utf8)
    }

    public static func saveInt32Array(_ path: String, _ values: [Int32]) throws {
        let text = values.map { String($0) }.joined(separator: "\n")
        try text.write(toFile: path, atomically: true, encoding: String.Encoding.utf8)
    }
}
