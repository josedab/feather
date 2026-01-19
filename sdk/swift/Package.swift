// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "FeatherSDK",
    platforms: [
        .iOS(.v15),
        .macOS(.v12),
        .watchOS(.v8),
        .tvOS(.v15)
    ],
    products: [
        .library(
            name: "FeatherSDK",
            targets: ["FeatherSDK"]
        ),
    ],
    targets: [
        .target(
            name: "FeatherSDK",
            path: "Sources/FeatherSDK"
        ),
        .testTarget(
            name: "FeatherSDKTests",
            dependencies: ["FeatherSDK"],
            path: "Tests/FeatherSDKTests"
        ),
    ]
)
