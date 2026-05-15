import Carbon.HIToolbox
import Foundation

struct GlobalShortcutDefinition {
  let accelerator: String
  let keyCode: UInt32
  let modifierFlags: UInt32

  static func parse(_ rawAccelerator: String) throws -> GlobalShortcutDefinition {
    let tokens = rawAccelerator
      .split(separator: "+")
      .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
      .filter { !$0.isEmpty }
    guard tokens.count >= 2 else {
      throw DesktopShellError.globalShortcutInvalid(rawAccelerator)
    }

    var modifiers = ModifierSet()
    var selectedKey: KeySpec?
    for token in tokens {
      if let modifier = modifier(for: token) {
        modifiers.insert(modifier)
        continue
      }
      guard selectedKey == nil, let key = keySpec(for: token) else {
        throw DesktopShellError.globalShortcutInvalid(rawAccelerator)
      }
      selectedKey = key
    }

    guard modifiers.hasAny, let selectedKey else {
      throw DesktopShellError.globalShortcutInvalid(rawAccelerator)
    }

    return GlobalShortcutDefinition(
      accelerator: canonicalAccelerator(modifiers: modifiers, key: selectedKey),
      keyCode: selectedKey.keyCode,
      modifierFlags: modifiers.carbonFlags
    )
  }

  private static func modifier(for token: String) -> Modifier? {
    switch normalized(token) {
    case "command", "cmd", "meta", "⌘":
      return .command
    case "option", "opt", "alt", "⌥":
      return .option
    case "control", "ctrl", "⌃":
      return .control
    case "shift", "⇧":
      return .shift
    default:
      return nil
    }
  }

  private static func keySpec(for token: String) -> KeySpec? {
    let normalizedToken = normalized(token)
    if let key = namedKeys[normalizedToken] {
      return key
    }
    if normalizedToken.count == 1, let character = normalizedToken.first {
      if let keyCode = letterKeyCodes[character] {
        return KeySpec(display: String(character).uppercased(), keyCode: keyCode)
      }
      if let keyCode = numberKeyCodes[character] {
        return KeySpec(display: String(character), keyCode: keyCode)
      }
    }
    return nil
  }

  private static func canonicalAccelerator(modifiers: ModifierSet, key: KeySpec) -> String {
    var parts: [String] = []
    if modifiers.contains(.command) {
      parts.append("Command")
    }
    if modifiers.contains(.option) {
      parts.append("Option")
    }
    if modifiers.contains(.control) {
      parts.append("Control")
    }
    if modifiers.contains(.shift) {
      parts.append("Shift")
    }
    parts.append(key.display)
    return parts.joined(separator: " + ")
  }

  private static func normalized(_ token: String) -> String {
    token
      .trimmingCharacters(in: .whitespacesAndNewlines)
      .lowercased()
      .replacingOccurrences(of: " ", with: "")
  }

  private struct KeySpec {
    let display: String
    let keyCode: UInt32
  }

  private enum Modifier {
    case command
    case option
    case control
    case shift
  }

  private struct ModifierSet {
    private var values: Set<Modifier> = []

    var hasAny: Bool {
      !values.isEmpty
    }

    var carbonFlags: UInt32 {
      var flags: UInt32 = 0
      if contains(.command) {
        flags |= UInt32(cmdKey)
      }
      if contains(.option) {
        flags |= UInt32(optionKey)
      }
      if contains(.control) {
        flags |= UInt32(controlKey)
      }
      if contains(.shift) {
        flags |= UInt32(shiftKey)
      }
      return flags
    }

    mutating func insert(_ modifier: Modifier) {
      values.insert(modifier)
    }

    func contains(_ modifier: Modifier) -> Bool {
      values.contains(modifier)
    }
  }

  private static let namedKeys: [String: KeySpec] = [
    "space": KeySpec(display: "Space", keyCode: UInt32(kVK_Space)),
    "tab": KeySpec(display: "Tab", keyCode: UInt32(kVK_Tab)),
    "return": KeySpec(display: "Return", keyCode: UInt32(kVK_Return)),
    "enter": KeySpec(display: "Return", keyCode: UInt32(kVK_Return)),
    "escape": KeySpec(display: "Escape", keyCode: UInt32(kVK_Escape)),
    "esc": KeySpec(display: "Escape", keyCode: UInt32(kVK_Escape)),
    "left": KeySpec(display: "Left", keyCode: UInt32(kVK_LeftArrow)),
    "arrowleft": KeySpec(display: "Left", keyCode: UInt32(kVK_LeftArrow)),
    "right": KeySpec(display: "Right", keyCode: UInt32(kVK_RightArrow)),
    "arrowright": KeySpec(display: "Right", keyCode: UInt32(kVK_RightArrow)),
    "up": KeySpec(display: "Up", keyCode: UInt32(kVK_UpArrow)),
    "arrowup": KeySpec(display: "Up", keyCode: UInt32(kVK_UpArrow)),
    "down": KeySpec(display: "Down", keyCode: UInt32(kVK_DownArrow)),
    "arrowdown": KeySpec(display: "Down", keyCode: UInt32(kVK_DownArrow)),
  ]

  private static let letterKeyCodes: [Character: UInt32] = [
    "a": UInt32(kVK_ANSI_A),
    "b": UInt32(kVK_ANSI_B),
    "c": UInt32(kVK_ANSI_C),
    "d": UInt32(kVK_ANSI_D),
    "e": UInt32(kVK_ANSI_E),
    "f": UInt32(kVK_ANSI_F),
    "g": UInt32(kVK_ANSI_G),
    "h": UInt32(kVK_ANSI_H),
    "i": UInt32(kVK_ANSI_I),
    "j": UInt32(kVK_ANSI_J),
    "k": UInt32(kVK_ANSI_K),
    "l": UInt32(kVK_ANSI_L),
    "m": UInt32(kVK_ANSI_M),
    "n": UInt32(kVK_ANSI_N),
    "o": UInt32(kVK_ANSI_O),
    "p": UInt32(kVK_ANSI_P),
    "q": UInt32(kVK_ANSI_Q),
    "r": UInt32(kVK_ANSI_R),
    "s": UInt32(kVK_ANSI_S),
    "t": UInt32(kVK_ANSI_T),
    "u": UInt32(kVK_ANSI_U),
    "v": UInt32(kVK_ANSI_V),
    "w": UInt32(kVK_ANSI_W),
    "x": UInt32(kVK_ANSI_X),
    "y": UInt32(kVK_ANSI_Y),
    "z": UInt32(kVK_ANSI_Z),
  ]

  private static let numberKeyCodes: [Character: UInt32] = [
    "0": UInt32(kVK_ANSI_0),
    "1": UInt32(kVK_ANSI_1),
    "2": UInt32(kVK_ANSI_2),
    "3": UInt32(kVK_ANSI_3),
    "4": UInt32(kVK_ANSI_4),
    "5": UInt32(kVK_ANSI_5),
    "6": UInt32(kVK_ANSI_6),
    "7": UInt32(kVK_ANSI_7),
    "8": UInt32(kVK_ANSI_8),
    "9": UInt32(kVK_ANSI_9),
  ]
}
