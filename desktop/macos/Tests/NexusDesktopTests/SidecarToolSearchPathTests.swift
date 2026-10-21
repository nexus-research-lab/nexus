import XCTest
@testable import NexusDesktop

final class SidecarToolSearchPathTests: XCTestCase {
  func testAppendsBundledBinAfterSystemPath() {
    XCTAssertEqual(
      SidecarSupervisor.appendingSearchPath("/App/bin", to: "/usr/bin:/bin"),
      "/usr/bin:/bin:/App/bin"
    )
  }

  func testDoesNotDuplicateOrShadowExistingEntries() {
    XCTAssertEqual(
      SidecarSupervisor.appendingSearchPath("/App/bin", to: "/opt/homebrew/bin:/App/bin:/usr/bin"),
      "/opt/homebrew/bin:/App/bin:/usr/bin"
    )
  }

  func testUsesBundledBinWhenPathIsMissing() {
    XCTAssertEqual(SidecarSupervisor.appendingSearchPath("/App/bin", to: nil), "/App/bin")
    XCTAssertEqual(SidecarSupervisor.appendingSearchPath("/App/bin", to: "::"), "/App/bin")
  }
}
