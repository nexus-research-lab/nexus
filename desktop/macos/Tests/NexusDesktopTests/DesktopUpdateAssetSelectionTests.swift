import Foundation
import XCTest

@testable import NexusDesktop

@MainActor
final class DesktopUpdateAssetSelectionTests: XCTestCase {
  func testSelectsMatchingArchitectureAssets() {
    let assets = [
      releaseAsset("Nexus-macos-arm64-0.1.33-100.dmg.metadata.json"),
      releaseAsset("Nexus-macos-intel-0.1.33-100.dmg.metadata.json"),
      releaseAsset("Nexus-macos-arm64-0.1.33-100.dmg"),
      releaseAsset("Nexus-macos-intel-0.1.33-100.dmg"),
      releaseAsset("Nexus-macos-arm64-0.1.33-100.dmg.sha256"),
      releaseAsset("Nexus-macos-intel-0.1.33-100.dmg.sha256"),
    ]

    let armPackage = DesktopReleaseAssetSelector.macOSPackageAsset(
      in: assets,
      architecture: "arm64"
    )
    XCTAssertEqual(armPackage?.name, "Nexus-macos-arm64-0.1.33-100.dmg")
    XCTAssertEqual(
      DesktopReleaseAssetSelector.macOSMetadataAsset(in: assets, architecture: "arm64")?.name,
      "Nexus-macos-arm64-0.1.33-100.dmg.metadata.json"
    )
    XCTAssertEqual(
      DesktopReleaseAssetSelector.macOSPackageSHA256Asset(
        in: assets,
        packageAsset: armPackage,
        architecture: "arm64"
      )?.name,
      "Nexus-macos-arm64-0.1.33-100.dmg.sha256"
    )

    let intelPackage = DesktopReleaseAssetSelector.macOSPackageAsset(
      in: assets,
      architecture: "x86_64"
    )
    XCTAssertEqual(intelPackage?.name, "Nexus-macos-intel-0.1.33-100.dmg")
    XCTAssertEqual(
      DesktopReleaseAssetSelector.macOSMetadataAsset(in: assets, architecture: "amd64")?.name,
      "Nexus-macos-intel-0.1.33-100.dmg.metadata.json"
    )
    XCTAssertEqual(
      DesktopReleaseAssetSelector.macOSPackageSHA256Asset(
        in: assets,
        packageAsset: intelPackage,
        architecture: "intel"
      )?.name,
      "Nexus-macos-intel-0.1.33-100.dmg.sha256"
    )
  }

  func testFallsBackToLegacyUnqualifiedAsset() {
    let assets = [
      releaseAsset("Nexus-macos-0.1.32-99.dmg.metadata.json"),
      releaseAsset("Nexus-macos-0.1.32-99.dmg"),
      releaseAsset("Nexus-macos-0.1.32-99.dmg.sha256"),
    ]

    let package = DesktopReleaseAssetSelector.macOSPackageAsset(
      in: assets,
      architecture: "x86_64"
    )
    XCTAssertEqual(package?.name, "Nexus-macos-0.1.32-99.dmg")
    XCTAssertEqual(
      DesktopReleaseAssetSelector.macOSMetadataAsset(in: assets, architecture: "x86_64")?.name,
      "Nexus-macos-0.1.32-99.dmg.metadata.json"
    )
    XCTAssertEqual(
      DesktopReleaseAssetSelector.macOSPackageSHA256Asset(
        in: assets,
        packageAsset: package,
        architecture: "x86_64"
      )?.name,
      "Nexus-macos-0.1.32-99.dmg.sha256"
    )
  }

  func testDoesNotFallBackToAnotherArchitecture() {
    let assets = [
      releaseAsset("Nexus-macos-arm64-0.1.33-100.dmg.metadata.json"),
      releaseAsset("Nexus-macos-arm64-0.1.33-100.dmg"),
      releaseAsset("Nexus-macos-arm64-0.1.33-100.dmg.sha256"),
    ]

    XCTAssertNil(DesktopReleaseAssetSelector.macOSMetadataAsset(
      in: assets,
      architecture: "x86_64"
    ))
    XCTAssertNil(DesktopReleaseAssetSelector.macOSPackageAsset(
      in: assets,
      architecture: "x86_64"
    ))
    XCTAssertNil(DesktopReleaseAssetSelector.macOSPackageSHA256Asset(
      in: assets,
      packageAsset: nil,
      architecture: "x86_64"
    ))
  }

  func testDoesNotPairArchitecturePackageWithLegacyChecksum() {
    let assets = [
      releaseAsset("Nexus-macos-intel-0.1.33-100.dmg"),
      releaseAsset("Nexus-macos-0.1.32-99.dmg.sha256"),
    ]
    let package = DesktopReleaseAssetSelector.macOSPackageAsset(
      in: assets,
      architecture: "x86_64"
    )

    XCTAssertNotNil(package)
    XCTAssertNil(DesktopReleaseAssetSelector.macOSPackageSHA256Asset(
      in: assets,
      packageAsset: package,
      architecture: "x86_64"
    ))
  }

  private func releaseAsset(_ name: String) -> GitHubReleaseAsset {
    GitHubReleaseAsset(
      name: name,
      browserDownloadURL: URL(string: "https://example.com/\(name)")
    )
  }
}
