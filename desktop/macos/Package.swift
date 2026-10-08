// swift-tools-version: 5.9

import PackageDescription

let package = Package(
  name: "NexusDesktop",
  platforms: [
    .macOS("14.2"),
  ],
  products: [
    .executable(name: "NexusDesktop", targets: ["NexusDesktop"]),
  ],
  targets: [
    .executableTarget(
      name: "NexusDesktop",
      path: "Sources/NexusDesktop"
    ),
    .testTarget(
      name: "NexusDesktopTests",
      dependencies: ["NexusDesktop"],
      path: "Tests/NexusDesktopTests"
    ),
  ]
)
