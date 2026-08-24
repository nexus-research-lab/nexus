import AppKit
import XCTest

@testable import NexusDesktop

@MainActor
final class DesktopWindowMetricsTests: XCTestCase {
  func testAlignWindowControlsUsesSharedHeaderCenter() {
    _ = NSApplication.shared
    let window = NSWindow(
      contentRect: NSRect(x: 0, y: 0, width: 800, height: 600),
      styleMask: [
        .titled,
        .closable,
        .miniaturizable,
        .resizable,
        .fullSizeContentView,
      ],
      backing: .buffered,
      defer: false
    )
    window.titleVisibility = .hidden
    window.titlebarAppearsTransparent = true
    window.layoutIfNeeded()

    DesktopWindowMetrics.alignWindowControls(in: window)

    XCTAssertEqual(
      DesktopWindowMetrics.windowCloseButtonCenter(in: window).y,
      DesktopWindowMetrics.windowControlsCenterY,
      accuracy: 0.5
    )
  }
}
