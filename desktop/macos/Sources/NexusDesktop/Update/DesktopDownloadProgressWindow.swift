// INPUT: 更新版本与下载字节进度。
// OUTPUT: 非阻塞的原生下载进度窗口。
// POS: macOS 更新下载阶段的唯一可视反馈。

import AppKit

@MainActor
final class DesktopDownloadProgressWindow {
  private let panel: NSPanel
  private let progressIndicator = NSProgressIndicator()
  private let progressLabel = NSTextField(labelWithString: "正在连接下载服务…")

  init(release: DesktopReleaseInfo) {
    panel = NSPanel(
      contentRect: NSRect(x: 0, y: 0, width: 520, height: 220),
      styleMask: [.titled, .closable],
      backing: .buffered,
      defer: false
    )
    panel.title = "Nexus 正在下载更新"
    panel.isReleasedWhenClosed = false

    let titleLabel = NSTextField(labelWithString: "正在下载 Nexus \(release.displayText)")
    titleLabel.font = .systemFont(ofSize: 20, weight: .semibold)

    progressIndicator.isIndeterminate = true
    progressIndicator.style = .bar
    progressIndicator.minValue = 0
    progressIndicator.maxValue = 1
    progressIndicator.startAnimation(nil)

    let stack = NSStackView(views: [titleLabel, progressLabel, progressIndicator])
    stack.orientation = .vertical
    stack.alignment = .leading
    stack.spacing = 20
    stack.translatesAutoresizingMaskIntoConstraints = false
    progressIndicator.widthAnchor.constraint(equalToConstant: 464).isActive = true

    let contentView = NSView()
    contentView.addSubview(stack)
    NSLayoutConstraint.activate([
      stack.leadingAnchor.constraint(equalTo: contentView.leadingAnchor, constant: 28),
      stack.trailingAnchor.constraint(equalTo: contentView.trailingAnchor, constant: -28),
      stack.centerYAnchor.constraint(equalTo: contentView.centerYAnchor),
    ])
    panel.contentView = contentView
  }

  func show() {
    panel.center()
    panel.makeKeyAndOrderFront(nil)
  }

  func report(receivedBytes: Int64, totalBytes: Int64?) {
    progressLabel.stringValue = if let totalBytes, totalBytes > 0 {
      "\(Self.formatBytes(receivedBytes)) / \(Self.formatBytes(totalBytes))"
    } else {
      Self.formatBytes(receivedBytes)
    }
    if let totalBytes, totalBytes > 0 {
      progressIndicator.stopAnimation(nil)
      progressIndicator.isIndeterminate = false
      progressIndicator.doubleValue = min(1, Double(receivedBytes) / Double(totalBytes))
    }
  }

  func close() {
    panel.orderOut(nil)
  }

  private static func formatBytes(_ bytes: Int64) -> String {
    String(format: "%.1f MB", Double(bytes) / 1_024 / 1_024)
  }
}
