import Foundation
import XCTest

@testable import NexusDesktop

final class DesktopWorkspaceFileActionsTests: XCTestCase {
  func testWorkspaceFileBoundaryResolvesSymlinks() throws {
    let fileManager = FileManager.default
    let fixture = fileManager.temporaryDirectory.appendingPathComponent(
      "nexus-workspace-file-actions-\(UUID().uuidString)",
      isDirectory: true
    )
    defer { try? fileManager.removeItem(at: fixture) }

    let root = fixture.appendingPathComponent("users", isDirectory: true)
    let file = root.appendingPathComponent("owner/workspace/report.html")
    let outside = fixture.appendingPathComponent("outside.html")
    let alias = root.appendingPathComponent("owner/workspace/alias.html")
    try fileManager.createDirectory(
      at: file.deletingLastPathComponent(),
      withIntermediateDirectories: true
    )
    try Data().write(to: file)
    try Data().write(to: outside)
    try fileManager.createSymbolicLink(at: alias, withDestinationURL: outside)

    XCTAssertTrue(DesktopWorkspaceFileActions.isFileURL(file, inside: root))
    XCTAssertFalse(DesktopWorkspaceFileActions.isFileURL(outside, inside: root))
    XCTAssertFalse(DesktopWorkspaceFileActions.isFileURL(alias, inside: root))
  }
}
