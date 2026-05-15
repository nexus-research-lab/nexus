import AppKit
import Carbon.HIToolbox

@MainActor
final class GlobalShortcutMonitor {
  typealias Handler = @MainActor () -> Void

  private let handler: Handler
  private var hotKeyRef: EventHotKeyRef?
  private var eventHandlerRef: EventHandlerRef?

  init(handler: @escaping Handler) {
    self.handler = handler
  }

  func start(definition: GlobalShortcutDefinition) throws {
    stop()

    var eventSpec = EventTypeSpec(
      eventClass: OSType(kEventClassKeyboard),
      eventKind: UInt32(kEventHotKeyPressed)
    )
    let installStatus = InstallEventHandler(
      GetApplicationEventTarget(),
      { _, event, userData in
        guard let event, let userData else {
          return noErr
        }

        var hotKeyID = EventHotKeyID()
        let parameterStatus = GetEventParameter(
          event,
          EventParamName(kEventParamDirectObject),
          EventParamType(typeEventHotKeyID),
          nil,
          MemoryLayout<EventHotKeyID>.size,
          nil,
          &hotKeyID
        )
        guard parameterStatus == noErr,
              hotKeyID.signature == GlobalShortcutMonitor.hotKeySignature,
              hotKeyID.id == GlobalShortcutMonitor.hotKeyID else {
          return parameterStatus
        }

        let monitor = Unmanaged<GlobalShortcutMonitor>.fromOpaque(userData).takeUnretainedValue()
        Task { @MainActor in
          monitor.handler()
        }
        return noErr
      },
      1,
      &eventSpec,
      UnsafeMutableRawPointer(Unmanaged.passUnretained(self).toOpaque()),
      &eventHandlerRef
    )
    guard installStatus == noErr else {
      throw DesktopShellError.globalShortcutInstallFailed(installStatus)
    }

    let hotKeyID = EventHotKeyID(
      signature: Self.hotKeySignature,
      id: Self.hotKeyID
    )
    let registerStatus = RegisterEventHotKey(
      definition.keyCode,
      definition.modifierFlags,
      hotKeyID,
      GetApplicationEventTarget(),
      0,
      &hotKeyRef
    )
    guard registerStatus == noErr else {
      stop()
      throw DesktopShellError.globalShortcutRegisterFailed(registerStatus)
    }
  }

  func stop() {
    if let hotKeyRef {
      UnregisterEventHotKey(hotKeyRef)
      self.hotKeyRef = nil
    }
    if let eventHandlerRef {
      RemoveEventHandler(eventHandlerRef)
      self.eventHandlerRef = nil
    }
  }

  private static let hotKeySignature: OSType = 0x4E455853
  private static let hotKeyID: UInt32 = 1
}
