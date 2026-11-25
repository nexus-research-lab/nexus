// INPUT: 设置页提供的运行时权限事实、界面语言与用户选择的权限。
// OUTPUT: 独立原生授权窗口；将选中行交给系统设置旁的拖放条，返回后请求复查。
// POS: macOS 权限向导；只有运行时的回读能把一项标记为完成。
import AppKit

enum ComputerUsePermission: String, CaseIterable {
  case accessibility
  case screenCapture = "screen_capture"
  case inputControl = "input_control"

  func labels(chinese: Bool) -> (name: String, detail: String, symbol: String) {
    switch self {
    case .accessibility:
      return (chinese ? "辅助功能" : "Accessibility", chinese ? "读取和操作界面控件" : "Access buttons, menus and controls", "accessibility.fill")
    case .screenCapture:
      return (chinese ? "屏幕录制" : "Screen Recording", chinese ? "查看授权应用的窗口" : "See the windows of authorized apps", "camera.viewfinder")
    case .inputControl:
      return (chinese ? "输入控制" : "Input Control", chinese ? "操作键鼠，在你接管时暂停" : "Use the keyboard and mouse until you take over", "keyboard")
    }
  }

  var pane: ComputerUsePrivacyPane {
    switch self {
    case .accessibility: return .accessibility
    case .screenCapture: return .screenRecording
    case .inputControl: return CGPreflightPostEventAccess() ? .inputMonitoring : .postEvents
    }
  }
}

final class ComputerUsePermissionWizard: NSObject, NSWindowDelegate {
  private let window: NSWindow
  private let chinese: Bool
  private let requestCheck: () -> Void
  private let onClose: () -> Void
  private let cards = NSStackView()
  private let retry: NSButton
  private let progress = NSTextField(labelWithString: "")
  private var facts: [String: String]
  private var waiting: ComputerUsePermission?
  private var strip: ComputerUsePermissionGuide?
  private var rows: [ComputerUsePermission: NSView] = [:]

  init(locale: String, facts: [String: String], requestCheck: @escaping () -> Void, onClose: @escaping () -> Void) {
    chinese = locale == "zh"
    self.facts = facts
    self.requestCheck = requestCheck
    self.onClose = onClose
    window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 520, height: 420),
                      styleMask: [.titled, .closable, .fullSizeContentView], backing: .buffered, defer: false)
    retry = NSButton(title: chinese ? "权限读取失败 · 重试" : "Couldn’t check permissions · Retry", target: nil, action: nil)
    super.init()
    window.delegate = self
    window.title = chinese ? "启用电脑操控" : "Enable Computer Use"
    window.titleVisibility = .hidden
    window.titlebarAppearsTransparent = true
    window.isReleasedWhenClosed = false
    window.isMovableByWindowBackground = true

    let background = NSVisualEffectView()
    background.material = .windowBackground
    background.blendingMode = .behindWindow
    window.contentView = background
    let icon = NSImageView(image: NSApp.applicationIconImage)
    icon.imageScaling = .scaleProportionallyUpOrDown
    icon.widthAnchor.constraint(equalToConstant: 60).isActive = true
    icon.heightAnchor.constraint(equalTo: icon.widthAnchor).isActive = true
    let title = NSTextField(labelWithString: window.title)
    title.font = .systemFont(ofSize: 20, weight: .semibold)
    progress.font = .systemFont(ofSize: 12)
    progress.textColor = .secondaryLabelColor
    cards.orientation = .vertical
    cards.spacing = 6
    cards.alignment = .width
    retry.bezelStyle = .inline
    retry.target = self
    retry.action = #selector(recheck)
    retry.isHidden = !facts.isEmpty
    let stack = NSStackView(views: [icon, title, progress, cards, retry])
    stack.orientation = .vertical
    stack.alignment = .centerX
    stack.spacing = 10
    stack.setCustomSpacing(4, after: title)
    stack.setCustomSpacing(18, after: progress)
    stack.translatesAutoresizingMaskIntoConstraints = false
    background.addSubview(stack)
    NSLayoutConstraint.activate([
      stack.centerXAnchor.constraint(equalTo: background.centerXAnchor),
      stack.centerYAnchor.constraint(equalTo: background.centerYAnchor, constant: 7),
      cards.widthAnchor.constraint(equalTo: background.widthAnchor, constant: -48),
    ])
    drawSteps()
    NotificationCenter.default.addObserver(self, selector: #selector(appReturned), name: NSApplication.didBecomeActiveNotification, object: NSApp)
  }

  func present(near owner: NSWindow?) {
    if !window.isVisible {
      window.center()
      if let owner, let screen = owner.screen {
        let area = screen.visibleFrame
        window.setFrameOrigin(NSPoint(
          x: max(area.minX, min(owner.frame.midX - window.frame.width / 2, area.maxX - window.frame.width)),
          y: max(area.minY, min(owner.frame.midY - window.frame.height / 2, area.maxY - window.frame.height))
        ))
      }
    }
    window.makeKeyAndOrderFront(nil)
    NSApp.activate()
  }

  func update(facts: [String: String]?, failed: Bool) {
    retry.isHidden = !failed
    if let facts { self.facts = facts }
    if !failed && ComputerUsePermission.allCases.allSatisfy(done) { close(); return }
    if let waiting, done(waiting) { endStep() }
    drawSteps()
  }

  func close() { window.close() }

  func windowWillClose(_ notification: Notification) {
    NotificationCenter.default.removeObserver(self)
    endStep()
    onClose()
  }

  func windowDidBecomeKey(_ notification: Notification) { appReturned() }

  @objc private func appReturned() {
    if waiting != nil { returnFromSettings() }
  }

  private func done(_ permission: ComputerUsePermission) -> Bool {
    let state = facts[permission.rawValue]
    return state == "granted" || state == "not_applicable"
  }

  private func drawSteps() {
    cards.arrangedSubviews.forEach { $0.removeFromSuperview() }
    rows.removeAll()
    let all = ComputerUsePermission.allCases
    let granted = all.filter(done).count
    progress.stringValue = chinese ? "已开启 \(granted)/\(all.count) 项" : "\(granted) of \(all.count) enabled"
    // 只有下一项待开启的权限使用主按钮，其余保持次要，用户一眼知道从哪里继续。
    let nextPermission = waiting == nil ? all.first { !done($0) } : nil
    for permission in ComputerUsePermission.allCases {
      let isWaiting = permission == waiting
      let card = isWaiting ? PendingPermissionCard() : NSBox()
      card.boxType = .custom
      card.fillColor = isWaiting ? .clear : .controlBackgroundColor.withAlphaComponent(0.66)
      card.cornerRadius = 10
      card.borderWidth = 0
      card.contentViewMargins = .zero
      card.heightAnchor.constraint(equalToConstant: 65).isActive = true
      let body = permissionRow(permission, waiting: isWaiting, primary: permission == nextPermission)
      body.translatesAutoresizingMaskIntoConstraints = false
      if let content = card.contentView {
        content.addSubview(body)
        NSLayoutConstraint.activate([
          body.leadingAnchor.constraint(equalTo: content.leadingAnchor, constant: 15),
          body.trailingAnchor.constraint(equalTo: content.trailingAnchor, constant: -15),
          body.centerYAnchor.constraint(equalTo: content.centerYAnchor),
        ])
      }
      cards.addArrangedSubview(card)
      rows[permission] = card
    }
  }

  private func permissionRow(_ permission: ComputerUsePermission, waiting: Bool, primary: Bool) -> NSView {
    let copy = permission.labels(chinese: chinese)
    // 三项权限用同一尺寸与中性色的图标，状态只由右侧表达。
    let icon = NSImageView()
    icon.image = NSImage(systemSymbolName: copy.symbol, accessibilityDescription: nil)
    icon.symbolConfiguration = .init(pointSize: 22, weight: .regular)
    icon.contentTintColor = .secondaryLabelColor
    icon.widthAnchor.constraint(equalToConstant: 32).isActive = true
    let title = NSTextField(labelWithString: copy.name)
    title.font = .systemFont(ofSize: 14, weight: .semibold)
    let detail = NSTextField(labelWithString: copy.detail)
    detail.font = .systemFont(ofSize: 12)
    detail.textColor = .secondaryLabelColor
    let labels = NSStackView(views: [title, detail])
    labels.orientation = .vertical
    labels.alignment = .leading
    labels.spacing = 3
    let action: NSView
    if waiting {
      action = statusView(chinese ? "在系统设置中完成" : "Finish in System Settings", symbol: nil)
    } else if done(permission) {
      action = statusView(chinese ? "已开启" : "Enabled", symbol: "checkmark.circle.fill")
    } else {
      let button = NSButton(title: chinese ? "允许" : "Allow", target: self, action: #selector(allow(_:)))
      button.identifier = .init(permission.rawValue)
      button.setAccessibilityLabel((chinese ? "允许" : "Allow ") + copy.name)
      button.bezelStyle = .rounded
      button.controlSize = .regular
      if primary { button.keyEquivalent = "\r" }
      action = button
    }
    let row = NSStackView(views: [icon, labels, NSView(), action])
    row.spacing = 12
    return row
  }

  // 完成态用系统绿色勾选；等待态用小号转圈，二者共用同一右侧位置。
  private func statusView(_ text: String, symbol: String?) -> NSView {
    let label = NSTextField(labelWithString: text)
    label.font = .systemFont(ofSize: 12, weight: .medium)
    label.textColor = .secondaryLabelColor
    let leading: NSView
    if let symbol {
      let mark = NSImageView(image: NSImage(systemSymbolName: symbol, accessibilityDescription: nil)!)
      mark.symbolConfiguration = .init(pointSize: 14, weight: .medium)
      mark.contentTintColor = .systemGreen
      leading = mark
    } else {
      let spinner = NSProgressIndicator()
      spinner.style = .spinning
      spinner.controlSize = .small
      spinner.startAnimation(nil)
      leading = spinner
    }
    let stack = NSStackView(views: [leading, label])
    stack.spacing = 5
    stack.setAccessibilityElement(true)
    stack.setAccessibilityLabel(text)
    return stack
  }

  @objc private func allow(_ sender: NSButton) {
    guard let raw = sender.identifier?.rawValue, let permission = ComputerUsePermission(rawValue: raw),
          let card = rows[permission] else { return }
    let source = window.convertToScreen(card.convert(card.bounds, to: nil))
    let snapshot = card.bitmapImageRepForCachingDisplay(in: card.bounds).map { bitmap in
      card.cacheDisplay(in: card.bounds, to: bitmap)
      let image = NSImage(size: card.bounds.size)
      image.addRepresentation(bitmap)
      return image
    }
    let pane = permission.pane
    endStep()
    waiting = permission
    drawSteps()
    // 应用包通过真实文件拖放进入系统列表；开发态裸进程没有可拖对象，使用系统请求。
    let appURL = Bundle.main.bundleURL
    if appURL.pathExtension.lowercased() != "app" { pane.requestAccess() }
    guard NSWorkspace.shared.open(pane.url) else {
      waiting = nil
      drawSteps()
      let alert = NSAlert()
      alert.messageText = chinese ? "无法打开系统设置" : "Couldn’t open System Settings"
      alert.beginSheetModal(for: window)
      return
    }
    guard appURL.pathExtension.lowercased() == "app" else { return }
    let strip = ComputerUsePermissionGuide(appURL: appURL, pane: pane, chinese: chinese) { [weak self] in
      guard let self else { return }
      self.returnFromSettings()
      self.present(near: nil)
    }
    self.strip = strip
    strip.show(from: source, snapshot: snapshot)
  }

  private func endStep() {
    strip?.close()
    strip = nil
    waiting = nil
  }

  private func returnFromSettings() {
    endStep()
    drawSteps()
    requestCheck()
  }

  @objc private func recheck() { requestCheck() }
}

private final class PendingPermissionCard: NSBox {
  override func draw(_ dirtyRect: NSRect) {
    super.draw(dirtyRect)
    let border = NSBezierPath(roundedRect: bounds.insetBy(dx: 1, dy: 1), xRadius: cornerRadius, yRadius: cornerRadius)
    border.setLineDash([3, 4], count: 2, phase: 0)
    NSColor.separatorColor.setStroke()
    border.stroke()
  }
}
