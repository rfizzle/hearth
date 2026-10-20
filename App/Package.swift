// swift-tools-version: 6.2
// Hearth: the app target (scenes, windows, helper lifecycle, wiring) and the
// gallery target over the five local packages. Imports point down: Hearth ->
// HearthFeatures -> HearthUI -> HearthDesign, HearthKit below all but
// HearthDesign, HearthNative only from Hearth.
import PackageDescription

let package = Package(
    name: "Hearth",
    platforms: [.macOS(.v26)],
    products: [
        .executable(name: "Hearth", targets: ["Hearth"]),
        .executable(name: "HearthGallery", targets: ["HearthGallery"]),
    ],
    dependencies: [
        .package(path: "Packages/HearthDesign"),
        .package(path: "Packages/HearthUI"),
        .package(path: "Packages/HearthKit"),
        .package(path: "Packages/HearthFeatures"),
        .package(path: "Packages/HearthNative"),
    ],
    targets: [
        .executableTarget(
            name: "Hearth",
            dependencies: [
                .product(name: "HearthFeatures", package: "HearthFeatures"),
                .product(name: "HearthKit", package: "HearthKit"),
                .product(name: "HearthNative", package: "HearthNative"),
            ],
            path: "Hearth/Sources"
        ),
        .executableTarget(
            name: "HearthGallery",
            dependencies: [
                .product(name: "HearthFeatures", package: "HearthFeatures"),
                .product(name: "HearthKit", package: "HearthKit"),
                .product(name: "HearthUI", package: "HearthUI"),
                .product(name: "HearthDesign", package: "HearthDesign"),
            ],
            path: "HearthGallery/Sources"
        ),
    ],
    swiftLanguageModes: [.v6]
)
