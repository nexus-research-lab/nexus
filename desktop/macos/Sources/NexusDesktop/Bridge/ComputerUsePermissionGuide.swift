// INPUT: 向导中选中的行、系统隐私面板、应用包及本机窗口几何。
// OUTPUT: 从选中行移至权限列表的可拖放提示条；放下成功或系统确认时退场。
// POS: 系统设置旁的非激活面板；文件拖放与实际授权回读独立。
import AppKit
import QuartzCore

enum ComputerUsePrivacyPane: String {
  case screenRecording = "Privacy_ScreenCapture"
  case accessibility = "Privacy_Accessibility"
  case inputMonitoring = "Privacy_ListenEvent"
  case postEvents

  var url: URL {
    let anchor = self == .postEvents ? Self.accessibility.rawValue : rawValue
    return URL(string: "x-apple.systempreferences:com.apple.preference.security?\(anchor)")!
  }

  var granted: Bool {
    switch self {
    case .screenRecording: return CGPreflightScreenCaptureAccess()
    case .accessibility: return AXIsProcessTrusted()
    case .inputMonitoring: return CGPreflightListenEventAccess()
    case .postEvents: return CGPreflightPostEventAccess()
    }
  }

  func requestAccess() {
    switch self {
    case .screenRecording: _ = CGRequestScreenCaptureAccess()
    case .accessibility: _ = AXIsProcessTrustedWithOptions([kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary)
    case .inputMonitoring: _ = CGRequestListenEventAccess()
    case .postEvents: _ = CGRequestPostEventAccess()
    }
  }
}

final class ComputerUsePermissionGuide: NSPanel {
  private let pane: ComputerUsePrivacyPane
  private let locator: ComputerUseSettingsLocator
  private let back: () -> Void
  private let chinese: Bool
  private let appName: String
  private let arrow = NSImageView()
  private let instruction = NSTextField(wrappingLabelWithString: "")
  private let item: ApplicationDragSource
  private var backAtChip: NSLayoutConstraint!
  private var backAtHeadline: NSLayoutConstraint!
  private var timer: Timer?
  private var source: NSRect = .zero
  private var departingCard: NSImage?
  private var destination: NSRect?
  private let deadline = ProcessInfo.processInfo.systemUptime + 10

  init(appURL: URL, pane: ComputerUsePrivacyPane, chinese: Bool, back: @escaping () -> Void) {
    self.pane = pane
    self.chinese = chinese
    self.back = back
    locator = ComputerUseSettingsLocator(appURL: appURL)
    appName = appURL.deletingPathExtension().lastPathComponent
    item = ApplicationDragSource(url: appURL, name: appName)
    super.init(contentRect: .zero, styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
    isReleasedWhenClosed = false
    isOpaque = false
    backgroundColor = .clear
    hasShadow = true
    hidesOnDeactivate = false
    isFloatingPanel = true
    level = .floating
    collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
    setAccessibilityTitle(chinese ? "电脑操控权限" : "Computer Use Permissions")

    let material = NSVisualEffectView()
    material.material = .popover
    material.blendingMode = .behindWindow
    material.state = .active
    material.wantsLayer = true
    material.layer?.cornerRadius = 9
    material.layer?.masksToBounds = true
    contentView = material
    let previous = NSButton(image: NSImage(systemSymbolName: "chevron.left", accessibilityDescription: nil)!,
                            target: self, action: #selector(goBack))
    previous.isBordered = false
    previous.contentTintColor = .secondaryLabelColor
    previous.setAccessibilityLabel(chinese ? "返回权限向导" : "Back to permissions")
    // 箭头只作方向提示，与说明文字同级，不压过可拖动的应用条。
    arrow.symbolConfiguration = .init(pointSize: 13, weight: .semibold)
    arrow.contentTintColor = .controlAccentColor
    arrow.widthAnchor.constraint(equalToConstant: 16).isActive = true
    instruction.font = .systemFont(ofSize: 13, weight: .medium)
    let headline = NSStackView(views: [arrow, instruction])
    headline.spacing = 6
    let body = NSStackView(views: [headline, item])
    body.orientation = .vertical
    body.alignment = .leading
    body.spacing = 8
    item.widthAnchor.constraint(equalTo: body.widthAnchor).isActive = true
    previous.translatesAutoresizingMaskIntoConstraints = false
    body.translatesAutoresizingMaskIntoConstraints = false
    material.addSubview(previous)
    material.addSubview(body)
    backAtChip = previous.centerYAnchor.constraint(equalTo: item.centerYAnchor)
    backAtHeadline = previous.centerYAnchor.constraint(equalTo: headline.centerYAnchor)
    NSLayoutConstraint.activate([
      previous.leadingAnchor.constraint(equalTo: material.leadingAnchor, constant: 6),
      previous.widthAnchor.constraint(equalToConstant: 24),
      body.leadingAnchor.constraint(equalTo: previous.trailingAnchor, constant: 4),
      body.trailingAnchor.constraint(equalTo: material.trailingAnchor, constant: -14),
      body.centerYAnchor.constraint(equalTo: material.centerYAnchor),
      backAtChip,
    ])
    item.onDrop = { [weak self] in self?.finishDrop() }
  }

  @objc private func goBack() { close(); back() }

  func show(from row: NSRect, snapshot: NSImage?) {
    source = row
    departingCard = snapshot
    let update = Timer(timeInterval: 0.18, repeats: true) { [weak self] timer in
      guard let self else { timer.invalidate(); return }
      self.followSettings()
    }
    timer = update
    RunLoop.main.add(update, forMode: .common)
    followSettings()
  }

  override func close() {
    timer?.invalidate()
    timer = nil
    super.close()
  }

  private func finishDrop() {
    ignoresMouseEvents = true
    if NSWorkspace.shared.accessibilityDisplayShouldReduceMotion { close(); return }
    NSAnimationContext.runAnimationGroup { context in
      context.duration = 0.12
      animator().alphaValue = 0
    } completionHandler: { [weak self] in self?.close() }
  }

  private func followSettings() {
    guard let app = NSWorkspace.shared.frontmostApplication, app.bundleIdentifier == "com.apple.systempreferences",
          let target = locator.locate(pid: app.processIdentifier) else {
      if destination != nil || ProcessInfo.processInfo.systemUptime > deadline { close() }
      return
    }
    if pane.granted { close(); return }
    let screen = NSScreen.screens.max { $0.frame.intersection(target.window).area < $1.frame.intersection(target.window).area }
    guard let visible = screen?.visibleFrame else { return }
    let next = layout(near: target, visible: visible)
    guard next != destination else { return }
    let first = destination == nil
    destination = next
    if first && !NSWorkspace.shared.accessibilityDisplayShouldReduceMotion {
      setFrame(source, display: false)
      // 从原权限行淡变成拖放条，两个状态共用同一条运动轨迹。
      let cover = departingCard.flatMap { image -> NSImageView? in
        guard let contentView else { return nil }
        let view = NSImageView(image: image)
        view.frame = contentView.bounds
        view.imageScaling = .scaleAxesIndependently
        view.autoresizingMask = [.width, .height]
        contentView.addSubview(view)
        return view
      }
      departingCard = nil
      orderFrontRegardless()
      NSAnimationContext.runAnimationGroup { context in
        context.duration = 0.55
        context.timingFunction = CAMediaTimingFunction(name: .easeOut)
        cover?.animator().alphaValue = 0
        animator().setFrame(next, display: true)
      } completionHandler: { cover?.removeFromSuperview() }
    } else {
      departingCard = nil
      setFrame(next, display: true)
      if first { orderFrontRegardless() }
    }
  }

  func layout(near target: ComputerUseSettingsTarget, visible: NSRect) -> NSRect {
    let toggling = target.toggle != nil
    if item.isHidden != toggling {
      (toggling ? backAtChip : backAtHeadline)?.isActive = false
      item.isHidden = toggling
      (toggling ? backAtHeadline : backAtChip)?.isActive = true
    }
    let message = toggling
      ? (chinese ? "打开「\(appName)」的开关" : "Turn on \(appName) in the list")
      : (chinese ? "将 \(appName) 拖入权限列表" : "Drag \(appName) into the permission list")
    instruction.stringValue = message
    let textWidth = max(270, instruction.intrinsicContentSize.width + 110)
    let listWidth = toggling ? 0 : min(560, (target.list?.width ?? 360) + 64)
    let size = NSSize(width: min(visible.width - 16, max(listWidth, textWidth)), height: toggling ? 48 : 96)
    let placement = Self.placement(size: size, target: target, screen: visible)
    arrow.image = NSImage(systemSymbolName: placement.arrow, accessibilityDescription: nil)
    return NSRect(origin: placement.origin, size: size)
  }

  static func placement(size: NSSize, target: ComputerUseSettingsTarget, screen: NSRect) -> (origin: NSPoint, arrow: String) {
    let area = screen.insetBy(dx: 8, dy: 8)
    let anchor = target.toggle ?? target.list ?? target.window
    let aligned = target.toggle?.midX ?? target.list?.minX ?? target.window.midX
    let x = min(max(aligned - 55, area.minX), area.maxX - size.width)
    let y = min(max(anchor.midY - size.height / 2, area.minY), area.maxY - size.height)
    // 列表下方留出系统添加/移除控件。长列表铺满屏幕时优先放侧边。
    let gap: CGFloat = target.list != nil && target.toggle == nil ? 28 : 7
    let options = [
      (NSPoint(x: x, y: anchor.minY - size.height - gap), "arrowshape.up.fill"),
      (NSPoint(x: x, y: anchor.maxY + gap), "arrowshape.down.fill"),
      (NSPoint(x: anchor.minX - size.width - 7, y: y), "arrowshape.right.fill"),
      (NSPoint(x: anchor.maxX + 7, y: y), "arrowshape.left.fill"),
    ]
    return options.first { area.contains(NSRect(origin: $0.0, size: size)) }
      ?? (NSPoint(x: x, y: area.minY), "arrowshape.up.fill")
  }
}

private extension CGRect {
  var area: CGFloat { isNull ? 0 : width * height }
}

final class ApplicationDragSource: NSStackView, NSDraggingSource {
  private let url: URL
  private let icon: NSImageView
  private let label: NSTextField
  private let grip: NSImageView
  var onDrop: (() -> Void)?

  init(url: URL, name: String) {
    self.url = url
    icon = NSImageView(image: NSWorkspace.shared.icon(forFile: url.path))
    label = NSTextField(labelWithString: name)
    // 右侧抓手提示整条都可拖动；拖动开始后与图标、名称一起隐去。
    grip = NSImageView(image: NSImage(systemSymbolName: "line.3.horizontal", accessibilityDescription: nil)!)
    super.init(frame: .zero)
    alignment = .centerY
    spacing = 9
    edgeInsets = NSEdgeInsets(top: 6, left: 8, bottom: 6, right: 10)
    heightAnchor.constraint(equalToConstant: 42).isActive = true
    icon.imageScaling = .scaleProportionallyUpOrDown
    icon.widthAnchor.constraint(equalToConstant: 30).isActive = true
    icon.heightAnchor.constraint(equalTo: icon.widthAnchor).isActive = true
    label.font = .systemFont(ofSize: 13, weight: .semibold)
    grip.symbolConfiguration = .init(pointSize: 12, weight: .medium)
    grip.contentTintColor = .tertiaryLabelColor
    addArrangedSubview(icon)
    addArrangedSubview(label)
    addArrangedSubview(NSView())
    addArrangedSubview(grip)
    wantsLayer = true
    layer?.cornerRadius = 8
    layer?.borderWidth = 1
    setAccessibilityElement(true)
    setAccessibilityLabel(name)
  }

  required init?(coder: NSCoder) { nil }

  override func updateLayer() {
    super.updateLayer()
    layer?.borderColor = NSColor.separatorColor.cgColor
    layer?.backgroundColor = NSColor.controlBackgroundColor.cgColor
  }

  override var wantsUpdateLayer: Bool { true }
  override func hitTest(_ point: NSPoint) -> NSView? { super.hitTest(point) == nil ? nil : self }
  override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }
  override func shouldDelayWindowOrdering(for event: NSEvent) -> Bool { true }
  override func mouseDown(with event: NSEvent) { NSApp.preventWindowOrdering() }
  override func resetCursorRects() { addCursorRect(bounds, cursor: .openHand) }

  func draggingItem() -> NSDraggingItem {
    let item = NSDraggingItem(pasteboardWriter: url as NSURL)
    let frame = convert(icon.bounds, from: icon).union(convert(label.bounds, from: label))
    let image = NSImage(size: frame.size, flipped: isFlipped) { _ in
      self.icon.image?.draw(in: self.convert(self.icon.bounds, from: self.icon).offsetBy(dx: -frame.minX, dy: -frame.minY))
      (self.label.stringValue as NSString).draw(in: self.convert(self.label.bounds, from: self.label).offsetBy(dx: -frame.minX, dy: -frame.minY),
        withAttributes: [.font: self.label.font!, .foregroundColor: NSColor.labelColor])
      return true
    }
    item.setDraggingFrame(frame, contents: image)
    return item
  }

  override func mouseDragged(with event: NSEvent) {
    let session = beginDraggingSession(with: [draggingItem()], event: event, source: self)
    session.animatesToStartingPositionsOnCancelOrFail = true
  }

  func draggingSession(_ session: NSDraggingSession, sourceOperationMaskFor context: NSDraggingContext) -> NSDragOperation {
    context == .outsideApplication ? .copy : []
  }

  func draggingSession(_ session: NSDraggingSession, willBeginAt screenPoint: NSPoint) {
    icon.alphaValue = 0
    label.alphaValue = 0
    grip.alphaValue = 0
  }

  func draggingSession(_ session: NSDraggingSession, endedAt screenPoint: NSPoint, operation: NSDragOperation) {
    icon.alphaValue = 1
    label.alphaValue = 1
    grip.alphaValue = 1
    if !operation.isEmpty { onDrop?() }
  }
}
