import XCTest

@testable import NexusDesktop

final class WebViewHostRecoveryTests: XCTestCase {
  func testResumeCheckSkipsChangedOrLoadingNavigation() {
    XCTAssertTrue(WebViewHost.shouldSkipResumeCheck(
      observedNavigationRequestID: 1,
      currentNavigationRequestID: 2,
      navigationInFlight: false,
      webViewIsLoading: false
    ))
    XCTAssertTrue(WebViewHost.shouldSkipResumeCheck(
      observedNavigationRequestID: 2,
      currentNavigationRequestID: 2,
      navigationInFlight: true,
      webViewIsLoading: false
    ))
    XCTAssertTrue(WebViewHost.shouldSkipResumeCheck(
      observedNavigationRequestID: 2,
      currentNavigationRequestID: 2,
      navigationInFlight: false,
      webViewIsLoading: true
    ))
    XCTAssertFalse(WebViewHost.shouldSkipResumeCheck(
      observedNavigationRequestID: 2,
      currentNavigationRequestID: 2,
      navigationInFlight: false,
      webViewIsLoading: false
    ))
  }
}
