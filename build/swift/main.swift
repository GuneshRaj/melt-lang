import Foundation

@main
struct MeltProgram {
    static func main() throws {
        let close = try MeltParquet.loadFloat32Column("data/prices_10k.parquet", column: "close")
        let movers = close.filter { $0 > Float(105) }
        let count = Int64(movers.count)
        let total = movers.reduce(Float(0), +)
        let avg = movers.isEmpty ? Float(0) : movers.reduce(Float(0), +) / Float(movers.count)
        let low = movers.min() ?? Float(0)
        let high = movers.max() ?? Float(0)
        print(count)
        print(total)
        print(avg)
        print(low)
        print(high)
    }
}
