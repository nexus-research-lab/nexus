// swift-tools-version: 5.9

import PackageDescription

let package = Package(
  name: "NexusDesktop",
  platforms: [
    .macOS(.v14),
  ],
  products: [
    .executable(name: "NexusDesktop", targets: ["NexusDesktop"]),
  ],
  targets: [
    .executableTarget(
      name: "NexusDesktop",
      path: "Sources/NexusDesktop"
    ),
  ]
)
