// INPUT: 前台系统设置的公开窗口几何，以及已授权时可读的辅助功能元素。
// OUTPUT: 当前权限列表或本应用开关的可见位置；遇到系统确认窗口则停止引导。
// POS: macOS 拖放提示的只读定位边界，不请求权限、不读取截屏、不操作系统控件。
import AppKit

struct ComputerUseSettingsTarget {
  let window: NSRect
  var list: NSRect?
  var toggle: NSRect?
}

final class ComputerUseSettingsLocator {
  private let appName: String
  private var list: AXUIElement?
  private var toggle: AXUIElement?
  private var scrollAreas: [AXUIElement] = []

  init(appURL: URL) { appName = appURL.lastPathComponent }

  func locate(pid: pid_t) -> ComputerUseSettingsTarget? {
    guard let desktopTop = NSScreen.screens.first?.frame.maxY,
          let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else { return nil }
    let frames = windows.compactMap { info -> CGRect? in
      guard info[kCGWindowOwnerPID as String] as? pid_t == pid,
            info[kCGWindowLayer as String] as? Int == 0,
            let bounds = info[kCGWindowBounds as String] as? NSDictionary else { return nil }
      return CGRect(dictionaryRepresentation: bounds)
    }
    guard let main = Self.unobstructedWindow(frames) else { return nil }
    var target = ComputerUseSettingsTarget(window: Self.appKit(main, desktopTop))
    guard AXIsProcessTrusted() else { return target }
    let app = AXUIElementCreateApplication(pid)
    AXUIElementSetMessagingTimeout(app, 0.15)
    guard let window = app.element(kAXMainWindowAttribute) else { return target }
    if window.children.contains(where: { $0.role == kAXSheetRole }) { return nil }
    if let focus = app.element(kAXFocusedWindowAttribute),
       focus.attribute(kAXModalAttribute) as? Bool == true { return nil }
    if list?.bounds == nil {
      scrollAreas = []
      list = findList(in: window, clipping: [], depth: 0)
      toggle = list.flatMap(ownToggle)
    }
    guard var visible = list?.bounds?.intersection(main) else { return target }
    for scroll in scrollAreas {
      if let bounds = scroll.bounds { visible = visible.intersection(bounds) }
    }
    guard !visible.isEmpty else { return target }
    target.list = Self.appKit(visible, desktopTop)
    if let toggle, let bounds = toggle.bounds, visible.contains(CGPoint(x: bounds.midX, y: bounds.midY)) {
      if toggle.attribute(kAXValueAttribute) as? Bool == true { return nil }
      target.toggle = Self.appKit(bounds, desktopTop)
    }
    return target
  }

  // 系统的「退出并重新打开」可能也是 layer 0 窗口，不能把最前面的矩形直接当设置页。
  static func unobstructedWindow(_ frames: [CGRect]) -> CGRect? {
    guard let front = frames.first, front.width >= 450, front.height >= 300 else { return nil }
    if frames.dropFirst().contains(where: { $0.contains(front) && $0.width > front.width && $0.height > front.height }) { return nil }
    return front
  }

  private static func appKit(_ rect: CGRect, _ top: CGFloat) -> NSRect {
    NSRect(x: rect.minX, y: top - rect.maxY, width: rect.width, height: rect.height)
  }

  private func findList(in element: AXUIElement, clipping: [AXUIElement], depth: Int) -> AXUIElement? {
    guard depth < 10 else { return nil }
    let role = element.role
    if (element.attribute(kAXIdentifierAttribute) as? String)?.localizedCaseInsensitiveContains("sidebar") == true { return nil }
    if role == kAXListRole || role == kAXTableRole {
      scrollAreas = clipping
      return element
    }
    guard [kAXWindowRole, kAXSplitGroupRole, kAXGroupRole, kAXScrollAreaRole].contains(role) else { return nil }
    let ancestors = role == kAXScrollAreaRole ? clipping + [element] : clipping
    // 从详情栏开始查找，录屏页的第一张表才是屏幕录制权限，下面一张表是仅系统音频。
    let children = element.children
    let ordered = role == kAXSplitGroupRole ? Array(children.reversed()) : children
    for child in ordered {
      if let found = findList(in: child, clipping: ancestors, depth: depth + 1) { return found }
    }
    return nil
  }

  private func ownToggle(in list: AXUIElement) -> AXUIElement? {
    let rows = list.attribute(kAXVisibleRowsAttribute) as? [AXUIElement] ?? list.children
    for row in rows {
      if let match = row.children.first(where: { $0.attribute(kAXIdentifierAttribute) as? String == appName + "_Toggle" }) { return match }
    }
    return nil
  }
}

private extension AXUIElement {
  func attribute(_ key: String) -> CFTypeRef? {
    var value: CFTypeRef?
    return AXUIElementCopyAttributeValue(self, key as CFString, &value) == .success ? value : nil
  }

  func element(_ key: String) -> AXUIElement? {
    guard let value = attribute(key), CFGetTypeID(value) == AXUIElementGetTypeID() else { return nil }
    return (value as! AXUIElement)
  }

  var role: String { attribute(kAXRoleAttribute) as? String ?? "" }
  var children: [AXUIElement] { attribute(kAXChildrenAttribute) as? [AXUIElement] ?? [] }

  var bounds: CGRect? {
    guard let pos = attribute(kAXPositionAttribute), CFGetTypeID(pos) == AXValueGetTypeID(),
          let extent = attribute(kAXSizeAttribute), CFGetTypeID(extent) == AXValueGetTypeID() else { return nil }
    var point = CGPoint.zero
    var size = CGSize.zero
    guard AXValueGetValue(pos as! AXValue, .cgPoint, &point),
          AXValueGetValue(extent as! AXValue, .cgSize, &size), size.width > 0, size.height > 0 else { return nil }
    return CGRect(origin: point, size: size)
  }
}
