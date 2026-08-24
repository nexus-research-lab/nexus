import AppKit

enum DesktopWindowMetrics {
  static let windowControlsCenterY: CGFloat = 24

  private static let fallbackCloseButtonCenter = CGPoint(
    x: 16,
    y: windowControlsCenterY
  )
  private static let fallbackWindowControlsLeadingInset: CGFloat = 96
  private static let windowControlsTrailingPadding: CGFloat = 16
  private static let windowButtonTypes: [NSWindow.ButtonType] = [
    .closeButton,
    .miniaturizeButton,
    .zoomButton,
  ]

  static func alignWindowControls(in window: NSWindow) {
    let offset = windowControlsCenterY - windowCloseButtonCenter(in: window).y
    guard abs(offset) >= 0.5 else {
      return
    }
    windowButtonTypes.forEach { buttonType in
      guard let button = window.standardWindowButton(buttonType),
            !button.isHidden,
            let buttonSuperview = button.superview else {
        return
      }
      var origin = button.frame.origin
      origin.y += buttonSuperview.isFlipped ? offset : -offset
      button.setFrameOrigin(origin)
    }
  }

  static func windowControlsLeadingInset(in window: NSWindow) -> CGFloat {
    let trailingEdge = windowButtonTypes.compactMap { buttonType -> CGFloat? in
      guard let button = window.standardWindowButton(buttonType),
            !button.isHidden else {
        return nil
      }
      return button.convert(button.bounds, to: nil).maxX
    }.max()
    guard let trailingEdge else {
      return fallbackWindowControlsLeadingInset
    }
    return ceil(trailingEdge + windowControlsTrailingPadding)
  }

  static func windowCloseButtonCenter(in window: NSWindow) -> CGPoint {
    guard let button = window.standardWindowButton(.closeButton),
          !button.isHidden,
          let contentView = window.contentView else {
      return fallbackCloseButtonCenter
    }
    let buttonFrame = button.convert(button.bounds, to: contentView)
    let centerY = contentView.isFlipped
      ? buttonFrame.midY - contentView.bounds.minY
      : contentView.bounds.maxY - buttonFrame.midY
    return CGPoint(
      x: buttonFrame.midX - contentView.bounds.minX,
      y: centerY
    )
  }
}
