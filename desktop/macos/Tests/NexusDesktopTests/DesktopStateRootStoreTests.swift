import Foundation
import XCTest

@testable import NexusDesktop

final class DesktopStateRootStoreTests: XCTestCase {
  func testManagedRootComparisonResolvesSymlinkedExistingAncestor() throws {
    let fileManager = FileManager.default
    let fixture = fileManager.temporaryDirectory.appendingPathComponent(
      "nexus-state-root-store-\(UUID().uuidString)",
      isDirectory: true
    )
    defer { try? fileManager.removeItem(at: fixture) }

    let source = fixture.appendingPathComponent("source", isDirectory: true)
    let alias = fixture.appendingPathComponent("source-alias", isDirectory: true)
    try fileManager.createDirectory(at: source, withIntermediateDirectories: true)
    try fileManager.createSymbolicLink(at: alias, withDestinationURL: source)

    XCTAssertTrue(DesktopStateRootStore.sameManagedRoot(source, alias))
    XCTAssertTrue(DesktopStateRootStore.managedRootContains(
      source,
      alias.appendingPathComponent("missing-target", isDirectory: true)
    ))
    XCTAssertFalse(DesktopStateRootStore.managedRootContains(alias, fixture))
  }
}
