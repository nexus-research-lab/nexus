/**
 * INPUT: Web Header 的拖动与交互矩形，以及 AppKit 原始鼠标事件。
 * OUTPUT: 窗口拖动、双击缩放与普通 Web 交互之间的互斥分发。
 * POS: macOS 标题栏交互真相源，不绘制或覆盖任何可见界面。
 */
import AppKit
import WebKit

final class DesktopWindowInteractionHandler: NSObject, WKScriptMessageHandler {
  private let runtime: SidecarRuntimeConfig
  private let startupTimeline: DesktopStartupTimeline?
  private let interactionState: DesktopWindowInteractionState
  private var didReportReady = false

  init(
    runtime: SidecarRuntimeConfig,
    startupTimeline: DesktopStartupTimeline?,
    interactionState: DesktopWindowInteractionState
  ) {
    self.runtime = runtime
    self.startupTimeline = startupTimeline
    self.interactionState = interactionState
  }

  func userContentController(
    _ userContentController: WKUserContentController,
    didReceive message: WKScriptMessage
  ) {
    if let reason = DesktopWebOriginPolicy.rejectionReason(message: message, runtime: runtime) {
      var metadata = DesktopWebOriginPolicy.metadata(message: message, runtime: runtime)
      metadata["reason"] = reason
      startupTimeline?.mark("desktop_window_interaction.rejected", metadata: metadata)
      return
    }
    guard message.frameInfo.isMainFrame,
          let payload = message.body as? [String: Any],
          payload["schema_version"] as? Int == 1,
          let kind = payload["kind"] as? String else {
      return
    }

    guard kind == "regions_changed" else {
      startupTimeline?.mark(
        "desktop_window_interaction.ignored",
        metadata: ["kind": kind]
      )
      return
    }

    let dragRegions = Self.rectangles(from: payload["drag_regions"])
    let noDragRegions = Self.rectangles(from: payload["no_drag_regions"])
    interactionState.update(
      dragRegions: dragRegions,
      noDragRegions: noDragRegions
    )
    reportReadyIfNeeded(
      dragRegionCount: dragRegions.count,
      noDragRegionCount: noDragRegions.count
    )
  }

  private static func rectangles(from value: Any?) -> [CGRect] {
    guard let records = value as? [Any] else {
      return []
    }
    return records.prefix(512).compactMap { value in
      guard let record = value as? [String: Any],
            let x = number(record["x"]),
            let y = number(record["y"]),
            let width = number(record["width"]),
            let height = number(record["height"]),
            width > 0,
            height > 0 else {
        return nil
      }
      return CGRect(x: x, y: y, width: width, height: height)
    }
  }

  private static func number(_ value: Any?) -> CGFloat? {
    guard let number = value as? NSNumber else {
      return nil
    }
    let result = CGFloat(truncating: number)
    return result.isFinite ? result : nil
  }

  private func reportReadyIfNeeded(
    dragRegionCount: Int,
    noDragRegionCount: Int
  ) {
    guard !didReportReady else {
      return
    }
    didReportReady = true
    startupTimeline?.mark(
      "desktop_window_interaction.ready",
      metadata: [
        "drag_region_count": "\(dragRegionCount)",
        "no_drag_region_count": "\(noDragRegionCount)",
      ]
    )
  }
}

final class DesktopWindowInteractionState {
  private var dragRegions: [CGRect] = []
  private var noDragRegions: [CGRect] = []

  func update(dragRegions: [CGRect], noDragRegions: [CGRect]) {
    self.dragRegions = dragRegions
    self.noDragRegions = noDragRegions
  }

  func canTrackWindowDrag(at point: CGPoint) -> Bool {
    dragRegions.contains(where: { $0.contains(point) })
      && !noDragRegions.contains(where: { $0.contains(point) })
  }
}

final class DesktopWindow: NSWindow {
  private enum PointerInteractionOutcome {
    case click(mouseUpEvent: NSEvent)
    case drag
    case cancelled
  }

  private static let dragActivationDistance: CGFloat = 4

  var interactionState: DesktopWindowInteractionState?

  override func sendEvent(_ event: NSEvent) {
    guard event.type == .leftMouseDown,
          let interactionState,
          let contentPoint = contentPoint(for: event),
          !isWindowControl(at: event.locationInWindow),
          interactionState.canTrackWindowDrag(at: contentPoint) else {
      super.sendEvent(event)
      return
    }

    switch trackPointerInteraction(from: event) {
    case .click(let mouseUpEvent):
      if event.clickCount == 2 {
        zoom(nil)
        return
      }
      super.sendEvent(event)
      super.sendEvent(mouseUpEvent)
    case .drag, .cancelled:
      return
    }
  }

  private func trackPointerInteraction(
    from mouseDownEvent: NSEvent
  ) -> PointerInteractionOutcome {
    let origin = mouseDownEvent.locationInWindow
    var outcome = PointerInteractionOutcome.cancelled

    // 先保留完整点击序列；只有越过系统手势阈值后才把原始按下事件交给窗口服务。
    trackEvents(
      matching: [.leftMouseDragged, .leftMouseUp],
      timeout: NSEvent.foreverDuration,
      mode: .eventTracking
    ) { [weak self] event, stop in
      guard let self, let event else {
        stop.pointee = true
        return
      }

      switch event.type {
      case .leftMouseUp:
        outcome = .click(mouseUpEvent: event)
        stop.pointee = true
      case .leftMouseDragged:
        guard Self.dragDistance(
          from: origin,
          to: event.locationInWindow
        ) >= Self.dragActivationDistance else {
          return
        }
        outcome = .drag
        stop.pointee = true
        performDrag(with: mouseDownEvent)
      default:
        break
      }
    }
    return outcome
  }

  private static func dragDistance(
    from origin: CGPoint,
    to point: CGPoint
  ) -> CGFloat {
    hypot(point.x - origin.x, point.y - origin.y)
  }

  private func contentPoint(for event: NSEvent) -> CGPoint? {
    guard let contentView else {
      return nil
    }
    let point = contentView.convert(event.locationInWindow, from: nil)
    let y = contentView.isFlipped
      ? point.y - contentView.bounds.minY
      : contentView.bounds.maxY - point.y
    return CGPoint(
      x: point.x - contentView.bounds.minX,
      y: y
    )
  }

  private func isWindowControl(at point: CGPoint) -> Bool {
    let buttonTypes: [NSWindow.ButtonType] = [
      .closeButton,
      .miniaturizeButton,
      .zoomButton,
    ]
    return buttonTypes.contains { buttonType in
      guard let button = standardWindowButton(buttonType),
            !button.isHidden else {
        return false
      }
      return button.convert(button.bounds, to: nil).contains(point)
    }
  }
}
