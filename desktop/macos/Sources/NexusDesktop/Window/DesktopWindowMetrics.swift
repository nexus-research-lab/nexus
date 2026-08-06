import AppKit

enum DesktopWindowMetrics {
  private static let fallbackCloseButtonCenter = CGPoint(x: 28, y: 26)
  private static let fallbackWindowControlsLeadingInset: CGFloat = 96
  private static let windowControlsTrailingPadding: CGFloat = 16

  static func windowControlsLeadingInset(in window: NSWindow) -> CGFloat {
    let buttonTypes: [NSWindow.ButtonType] = [
      .closeButton,
      .miniaturizeButton,
      .zoomButton,
    ]
    let trailingEdge = buttonTypes.compactMap { buttonType -> CGFloat? in
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
