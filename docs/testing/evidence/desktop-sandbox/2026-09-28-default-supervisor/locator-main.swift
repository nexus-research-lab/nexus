import Foundation
func XCTFail(_ text: String) { fatalError(text) }
func XCTAssertTrue(_ value: Bool) { precondition(value) }
func XCTAssertEqual<T: Equatable>(_ lhs: T, _ rhs: T) { precondition(lhs == rhs, "values differ") }
func XCTAssertThrowsError<T>(_ expression: @autoclosure () throws -> T, _ handler: (Error) -> Void) {
 do { _ = try expression(); fatalError("expected error") } catch { handler(error) }
}
@main struct Check {
 static func main() throws {
  let tests = SidecarBundleLocatorTests()
  try tests.testDevelopmentLocatorRejectsDistOlderThanNestedSource()
  try tests.testDevelopmentLocatorRejectsDistOlderThanBuildConfiguration()
  try tests.testDevelopmentLocatorAcceptsDistNewerThanInputs()
  try tests.testDevelopmentLocatorRejectsMissingHelper()
  print("PASS four production locator test bodies (standalone assertions; XCTest unavailable)")
 }
}
