import Foundation
import XCTest
@testable import NexusDesktop

final class DesktopURLRouterTests: XCTestCase {
  func testLauncherPreservesInitialPrompt() throws {
    let url = try XCTUnwrap(URL(string: "nexus://launcher?initial=%E6%9F%A5%E7%9C%8B%E7%BD%91%E9%A1%B5"))
    let route = try XCTUnwrap(DesktopURLRouter.webRoute(for: url))

    XCTAssertEqual(route.path, "/launcher")
    XCTAssertEqual(route.percentEncodedQuery, "initial=%E6%9F%A5%E7%9C%8B%E7%BD%91%E9%A1%B5")
  }
}
