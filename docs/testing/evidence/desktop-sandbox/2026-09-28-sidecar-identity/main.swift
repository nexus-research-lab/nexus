import Foundation
func XCTAssertTrue(_ value: Bool) { precondition(value) }
func XCTAssertFalse(_ value: Bool) { precondition(!value) }
func XCTAssertEqual<T: Equatable>(_ lhs: T, _ rhs: T) { precondition(lhs == rhs, "values differ") }
func XCTAssertThrowsError<T>(_ expression: @autoclosure () throws -> T) {
 do { _ = try expression(); fatalError("expected error") } catch {}
}
func XCTUnwrap<T>(_ expression: @autoclosure () throws -> T?) throws -> T {
 guard let result = try expression() else { fatalError("unexpected nil") }; return result
}
let tests = SidecarOrphanReaperTests()
try tests.testLegacyLiveAndObservationFailurePreserveRecord()
try tests.testReusedPIDNeverReceivesSignal()
try tests.testExactRecordSurvivesDifferentInstalledPath()
try tests.testCorruptAndReplacedRecordsAreRetained()
try tests.testNativeExactTerminationAndStaleTokenDenial()
try tests.testSignalFailureAndSymlinkRetainRecord()
try tests.testNativeOrphanCanBeObservedAndRetired()
try tests.testCaptureFailureLeavesPendingRecordAndNoSignalAuthority()
try tests.testIgnoredGracefulSignalEscalatesOnlySameIdentity()
print("PASS nine reaper test bodies, including native exact/stale-token termination")
