import WebKit
import XCTest

@testable import NexusDesktop

final class WebViewDataStoreTests: XCTestCase {
  @MainActor
  func testRegularLaunchKeepsPersistentStore() {
    for suite: String? in [nil, "", "  "] {
      XCTAssertTrue(WebViewConfigurationFactory.makeWebsiteDataStore(preferencesSuite: suite).isPersistent)
    }
  }

  @MainActor
  func testIsolatedLaunchDoesNotReuseLoginCookies() async throws {
    let first = WebViewConfigurationFactory.makeWebsiteDataStore(preferencesSuite: "onboarding-test")
    let next = WebViewConfigurationFactory.makeWebsiteDataStore(preferencesSuite: "onboarding-test")
    XCTAssertFalse(first.isPersistent)
    let cookie = try XCTUnwrap(HTTPCookie(properties: [
      .name: "nexus_session", .value: "test-session", .domain: "127.0.0.1", .path: "/",
    ]))
    await first.httpCookieStore.setCookie(cookie)
    let configuration = WKWebViewConfiguration()
    configuration.websiteDataStore = first
    let sharedCookies = await configuration.websiteDataStore.httpCookieStore.allCookies()
    let nextCookies = await next.httpCookieStore.allCookies()
    XCTAssertEqual(sharedCookies.first(where: { $0.name == cookie.name })?.value, cookie.value)
    XCTAssertTrue(nextCookies.isEmpty)
  }
}
