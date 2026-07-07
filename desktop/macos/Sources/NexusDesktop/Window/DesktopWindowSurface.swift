import AppKit

final class DesktopWindowSurface: NSView {
  private let effectView = NSVisualEffectView()
  private let dragRegionView = DesktopWindowDragRegionView()
  private let webContentView: NSView

  init(
    webContentView: NSView,
    material: NSVisualEffectView.Material,
    blendingMode: NSVisualEffectView.BlendingMode = .behindWindow,
    cornerRadius: CGFloat = 0
  ) {
    self.webContentView = webContentView
    super.init(frame: .zero)
    configureSurface(
      material: material,
      blendingMode: blendingMode,
      cornerRadius: cornerRadius
    )
  }

  @available(*, unavailable)
  required init?(coder: NSCoder) {
    fatalError("init(coder:) has not been implemented")
  }

  private func configureSurface(
    material: NSVisualEffectView.Material,
    blendingMode: NSVisualEffectView.BlendingMode,
    cornerRadius: CGFloat
  ) {
    wantsLayer = true
    layer?.cornerRadius = cornerRadius
    layer?.masksToBounds = cornerRadius > 0

    effectView.material = material
    effectView.blendingMode = blendingMode
    effectView.state = .active
    effectView.translatesAutoresizingMaskIntoConstraints = false
    addSubview(effectView)

    webContentView.translatesAutoresizingMaskIntoConstraints = false
    addSubview(webContentView)

    dragRegionView.translatesAutoresizingMaskIntoConstraints = false
    addSubview(dragRegionView)

    NSLayoutConstraint.activate([
      effectView.leadingAnchor.constraint(equalTo: leadingAnchor),
      effectView.trailingAnchor.constraint(equalTo: trailingAnchor),
      effectView.topAnchor.constraint(equalTo: topAnchor),
      effectView.bottomAnchor.constraint(equalTo: bottomAnchor),
      webContentView.leadingAnchor.constraint(equalTo: leadingAnchor),
      webContentView.trailingAnchor.constraint(equalTo: trailingAnchor),
      webContentView.topAnchor.constraint(equalTo: topAnchor),
      webContentView.bottomAnchor.constraint(equalTo: bottomAnchor),
      dragRegionView.leadingAnchor.constraint(equalTo: leadingAnchor, constant: DesktopWindowMetrics.trafficLightReservedWidth),
      dragRegionView.trailingAnchor.constraint(equalTo: trailingAnchor),
      dragRegionView.topAnchor.constraint(equalTo: topAnchor),
      dragRegionView.heightAnchor.constraint(equalToConstant: DesktopWindowMetrics.dragRegionHeight),
    ])
  }
}

private final class DesktopWindowDragRegionView: NSView {
  override var mouseDownCanMoveWindow: Bool {
    true
  }

  override func mouseDown(with event: NSEvent) {
    window?.performDrag(with: event)
  }
}
