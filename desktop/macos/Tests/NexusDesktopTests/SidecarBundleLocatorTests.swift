import Foundation
import XCTest

@testable import NexusDesktop

final class SidecarBundleLocatorTests: XCTestCase {
  func testDevelopmentLocatorRejectsDistOlderThanNestedSource() throws {
    let fixture = try makeFixture()
    defer { try? FileManager.default.removeItem(at: fixture.root) }

    let sourceURL = fixture.webRoot.appendingPathComponent("src/features/conversation/Composer.tsx")
    try write("export {}", to: sourceURL)
    try setModificationDate(Date(timeIntervalSince1970: 2_000), for: fixture.distIndex)
    try setModificationDate(Date(timeIntervalSince1970: 3_000), for: sourceURL)

    XCTAssertThrowsError(try SidecarBundleLocator.resolveDevelopment(projectRoot: fixture.root)) { error in
      guard case let DesktopShellError.webDistStale(webDistPath, newerInputPath) = error else {
        return XCTFail("Expected webDistStale, got \(error)")
      }
      XCTAssertEqual(webDistPath, fixture.webRoot.appendingPathComponent("dist").path)
      XCTAssertEqual(newerInputPath, sourceURL.path)
      XCTAssertTrue(error.localizedDescription.contains("make app-run-dev"))
    }
  }

  func testDevelopmentLocatorRejectsDistOlderThanBuildConfiguration() throws {
    let fixture = try makeFixture()
    defer { try? FileManager.default.removeItem(at: fixture.root) }

    let configURL = fixture.webRoot.appendingPathComponent("vite.config.ts")
    try write("export default {}", to: configURL)
    try setModificationDate(Date(timeIntervalSince1970: 2_000), for: fixture.distIndex)
    try setModificationDate(Date(timeIntervalSince1970: 3_000), for: configURL)

    XCTAssertThrowsError(try SidecarBundleLocator.resolveDevelopment(projectRoot: fixture.root)) { error in
      guard case let DesktopShellError.webDistStale(_, newerInputPath) = error else {
        return XCTFail("Expected webDistStale, got \(error)")
      }
      XCTAssertEqual(newerInputPath, configURL.path)
    }
  }

  func testDevelopmentLocatorAcceptsDistNewerThanInputs() throws {
    let fixture = try makeFixture()
    defer { try? FileManager.default.removeItem(at: fixture.root) }

    let sourceURL = fixture.webRoot.appendingPathComponent("src/main.tsx")
    let configURL = fixture.webRoot.appendingPathComponent("package.json")
    try write("export {}", to: sourceURL)
    try write("{}", to: configURL)
    try setModificationDate(Date(timeIntervalSince1970: 2_000), for: sourceURL)
    try setModificationDate(Date(timeIntervalSince1970: 2_500), for: configURL)
    try setModificationDate(Date(timeIntervalSince1970: 3_000), for: fixture.distIndex)

    let locator = try SidecarBundleLocator.resolveDevelopment(projectRoot: fixture.root)

    XCTAssertEqual(locator.projectRoot, fixture.root)
    XCTAssertEqual(locator.webDistURL, fixture.webRoot.appendingPathComponent("dist", isDirectory: true))
    XCTAssertEqual(locator.command, "/usr/bin/env")
    XCTAssertEqual(locator.arguments, ["go", "run", "./cmd/nexus-server"])
  }

  private func makeFixture() throws -> (root: URL, webRoot: URL, distIndex: URL) {
    let fileManager = FileManager.default
    let root = fileManager.temporaryDirectory.appendingPathComponent(
      "nexus-sidecar-bundle-locator-\(UUID().uuidString)",
      isDirectory: true
    )
    let webRoot = root.appendingPathComponent("web", isDirectory: true)
    let distIndex = webRoot.appendingPathComponent("dist/index.html")
    try write("<!doctype html>", to: distIndex)
    return (root, webRoot, distIndex)
  }

  private func write(_ contents: String, to url: URL) throws {
    try FileManager.default.createDirectory(
      at: url.deletingLastPathComponent(),
      withIntermediateDirectories: true
    )
    try contents.write(to: url, atomically: true, encoding: .utf8)
  }

  private func setModificationDate(_ date: Date, for url: URL) throws {
    try FileManager.default.setAttributes([.modificationDate: date], ofItemAtPath: url.path)
  }
}
