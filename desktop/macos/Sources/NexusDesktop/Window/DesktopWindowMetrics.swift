import AppKit

enum DesktopWindowMetrics {
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
}
