// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "FeatherQuickstart",
    platforms: [.macOS(.v12)],
    dependencies: [
        .package(path: "../../"),
    ],
    targets: [
        .executableTarget(
            name: "FeatherQuickstart",
            dependencies: [
                .product(name: "FeatherSDK", package: "feather"),
            ],
            path: "Sources"
        ),
    ]
)
