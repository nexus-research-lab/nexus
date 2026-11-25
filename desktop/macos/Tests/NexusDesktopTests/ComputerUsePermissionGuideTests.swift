// INPUT: 带空格的应用包路径和系统设置在副屏上的窗口几何。
// OUTPUT: 拖放提供准确的文件 URL，侧边提示避开目标并留在可见屏幕内。
// POS: 验证授权引导的 AppKit 边界，不触发 macOS 授权请求。
import AppKit
import XCTest

@testable import NexusDesktop

@MainActor
final class ComputerUsePermissionGuideTests: XCTestCase {
  func testDragProvidesAnAppFileURLOnAPrivatePasteboard() throws {
    _ = NSApplication.shared
    let url = URL(fileURLWithPath: "/Applications/Nexus Preview.app", isDirectory: true)
    let row = ApplicationDragSource(url: url, name: "Nexus Preview")
    row.frame.size = NSSize(width: 280, height: 30)
    row.layoutSubtreeIfNeeded()
    let item = row.draggingItem()
    XCTAssertGreaterThan(item.draggingFrame.width, 45)
    let pasteboard = NSPasteboard.withUniqueName()
    defer { pasteboard.releaseGlobally() }
    let writer = try XCTUnwrap(item.item as? NSPasteboardWriting)
    XCTAssertTrue(pasteboard.writeObjects([writer]))
    let written = try XCTUnwrap(pasteboard.string(forType: .fileURL))
    XCTAssertEqual(URL(string: written)?.standardizedFileURL, url.standardizedFileURL)
  }

  func testDragLeavesAnEmptyRowAndOnlyAnAcceptedDropFinishesTheStep() throws {
    _ = NSApplication.shared
    let row = ApplicationDragSource(url: URL(fileURLWithPath: "/Applications/Nexus.app"), name: "Nexus")
    let icon = try XCTUnwrap(row.arrangedSubviews.compactMap { $0 as? NSImageView }.first)
    let name = try XCTUnwrap(row.arrangedSubviews.compactMap { $0 as? NSTextField }.first)
    let session = NSDraggingSession()
    var finished = 0
    row.onDrop = { finished += 1 }
    XCTAssertEqual(row.draggingSession(session, sourceOperationMaskFor: .outsideApplication), .copy)
    row.draggingSession(session, willBeginAt: .zero)
    XCTAssertEqual([icon.alphaValue, name.alphaValue], [0, 0])
    row.draggingSession(session, endedAt: .zero, operation: [])
    XCTAssertEqual([icon.alphaValue, name.alphaValue], [1, 1])
    XCTAssertEqual(finished, 0)
    row.draggingSession(session, willBeginAt: .zero)
    row.draggingSession(session, endedAt: .zero, operation: .copy)
    XCTAssertEqual(finished, 1)
  }

  func testStripUsesFileRowUntilTheAppSwitchIsVisible() throws {
    _ = NSApplication.shared
    let strip = ComputerUsePermissionGuide(appURL: URL(fileURLWithPath: "/Applications/Nexus.app"),
                                          pane: .screenRecording, chinese: true, back: {})
    let screen = NSRect(x: 0, y: 0, width: 1440, height: 900)
    let list = NSRect(x: 460, y: 500, width: 445, height: 140)
    var target = ComputerUseSettingsTarget(window: screen.insetBy(dx: 160, dy: 110), list: list)
    let entry = strip.layout(near: target, visible: screen)
    strip.setFrame(entry, display: false)
    let content = try XCTUnwrap(strip.contentView)
    content.layoutSubtreeIfNeeded()
    let back = try XCTUnwrap(content.subviews.compactMap { $0 as? NSButton }.first)
    let body = try XCTUnwrap(content.subviews.compactMap { $0 as? NSStackView }.first)
    let chip = try XCTUnwrap(body.arrangedSubviews.compactMap { $0 as? ApplicationDragSource }.first)
    XCTAssertFalse(strip.styleMask.contains(.titled))
    XCTAssertFalse(chip.isHidden)
    XCTAssertGreaterThanOrEqual(entry.width, list.width)
    XCTAssertEqual(back.convert(back.bounds, to: content).midY, chip.convert(chip.bounds, to: content).midY, accuracy: 1)

    target.toggle = NSRect(x: 866, y: 550, width: 30, height: 20)
    let existing = strip.layout(near: target, visible: screen)
    XCTAssertTrue(chip.isHidden)
    XCTAssertLessThan(existing.height, entry.height)
    target.toggle = nil
    _ = strip.layout(near: target, visible: screen)
    XCTAssertFalse(chip.isHidden)
  }

  func testStripAlignsWithListAndAvoidsAnExistingSwitchOnASecondaryDisplay() {
    let screen = NSRect(x: -1280, y: 200, width: 1280, height: 760)
    let settings = NSRect(x: -1060, y: 250, width: 740, height: 590)
    let list = NSRect(x: -830, y: 600, width: 480, height: 120)
    let size = NSSize(width: 308, height: 82)
    let anchor = ComputerUseSettingsTarget(window: settings, list: list)
    let (origin, arrow) = ComputerUsePermissionGuide.placement(size: size, target: anchor, screen: screen)
    let strip = NSRect(origin: origin, size: size)
    XCTAssertTrue(screen.contains(strip))
    XCTAssertLessThan(strip.maxY, list.minY)
    XCTAssertTrue(list.minX > strip.minX && list.minX < strip.maxX)
    XCTAssertEqual(arrow, "arrowshape.up.fill")

    let toggle = NSRect(x: -420, y: 525, width: 46, height: 26)
    let entry = ComputerUseSettingsTarget(window: settings, list: settings, toggle: toggle)
    let adjacent = ComputerUsePermissionGuide.placement(size: size, target: entry, screen: screen)
    XCTAssertFalse(NSRect(origin: adjacent.origin, size: size).intersects(toggle))
  }

  func testRestartSheetIsNeverAPlacementTarget() {
    let main = CGRect(x: 222, y: 201, width: 740, height: 597)
    let restart = CGRect(x: 462, y: 389, width: 260, height: 220)
    XCTAssertEqual(ComputerUseSettingsLocator.unobstructedWindow([main]), main)
    XCTAssertNil(ComputerUseSettingsLocator.unobstructedWindow([restart, main]))
    XCTAssertNil(ComputerUseSettingsLocator.unobstructedWindow([main.insetBy(dx: 40, dy: 40), main]))
    XCTAssertNil(ComputerUseSettingsLocator.unobstructedWindow([]))
  }
}
