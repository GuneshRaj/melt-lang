// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "MeltSupport",
    platforms: [.macOS(.v13)],
    products: [
        .library(name: "MeltSupport", targets: ["MeltSupport"]),
    ],
    targets: [
        .target(name: "MeltSupport", path: "Sources/MeltSupport"),
    ],
)
