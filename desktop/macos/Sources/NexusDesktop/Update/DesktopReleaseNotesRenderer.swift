import AppKit
import Foundation

enum DesktopReleaseNotesRenderer {
  private static let bodyFont = NSFont.systemFont(ofSize: 12)
  private static let codeFont = NSFont.monospacedSystemFont(ofSize: 11, weight: .regular)
  private static let inlinePattern = try! NSRegularExpression(
    pattern: #"`[^`\n]+`|\*\*[^*\n]+\*\*|__[^_\n]+__|\*[^*\n]+\*|_[^_\n]+_|\[[^\]\n]+\]\([^\)\n]+\)"#
  )

  // 更新说明来自远端 Release，只做本地排版，不执行 HTML 或脚本。
  static func render(_ markdown: String) -> NSAttributedString {
    let normalized = markdown
      .replacingOccurrences(of: "\r\n", with: "\n")
      .replacingOccurrences(of: "\r", with: "\n")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let lines = normalized.components(separatedBy: "\n")
    let result = NSMutableAttributedString()
    var index = 0

    while index < lines.count {
      let line = lines[index]
      let trimmed = line.trimmingCharacters(in: .whitespaces)
      if trimmed.isEmpty {
        index += 1
        continue
      }

      if isFence(trimmed) {
        let fence = String(trimmed.prefix(3))
        index += 1
        var codeLines: [String] = []
        while index < lines.count {
          let codeLine = lines[index]
          if codeLine.trimmingCharacters(in: .whitespaces).hasPrefix(fence) {
            index += 1
            break
          }
          codeLines.append(codeLine)
          index += 1
        }
        appendCodeBlock(codeLines.joined(separator: "\n"), to: result)
        continue
      }

      if let heading = parseHeading(trimmed) {
        appendBlock(
          heading.text,
          to: result,
          font: NSFont.systemFont(ofSize: headingFontSize(heading.level), weight: .semibold),
          paragraphSpacing: 10
        )
        index += 1
        continue
      }

      if let bullet = parseBullet(trimmed) {
        appendBlock(
          bullet,
          to: result,
          prefix: "• ",
          headIndent: 18,
          firstLineHeadIndent: 0
        )
        index += 1
        continue
      }

      if let ordered = parseOrderedListItem(trimmed) {
        appendBlock(
          ordered.text,
          to: result,
          prefix: "\(ordered.marker) ",
          headIndent: 24,
          firstLineHeadIndent: 0
        )
        index += 1
        continue
      }

      if trimmed.hasPrefix(">") {
        let quote = String(trimmed.dropFirst()).trimmingCharacters(in: .whitespaces)
        appendBlock(
          quote,
          to: result,
          prefix: "│ ",
          font: NSFont.systemFont(ofSize: 12, weight: .regular),
          foregroundColor: .secondaryLabelColor,
          headIndent: 16,
          firstLineHeadIndent: 0
        )
        index += 1
        continue
      }

      if isHorizontalRule(trimmed) {
        appendBlock("────────", to: result, foregroundColor: .tertiaryLabelColor, paragraphSpacing: 6)
        index += 1
        continue
      }

      var paragraphLines: [String] = []
      while index < lines.count {
        let paragraphLine = lines[index]
        let paragraphTrimmed = paragraphLine.trimmingCharacters(in: .whitespaces)
        if paragraphTrimmed.isEmpty || isBlockStart(paragraphTrimmed) {
          break
        }
        paragraphLines.append(paragraphTrimmed)
        index += 1
      }
      if !paragraphLines.isEmpty {
        appendBlock(paragraphLines.joined(separator: " "), to: result)
      } else {
        index += 1
      }
    }

    if result.string.hasSuffix("\n") {
      result.deleteCharacters(in: NSRange(location: result.length - 1, length: 1))
    }
    return result
  }

  private static func appendBlock(
    _ text: String,
    to result: NSMutableAttributedString,
    prefix: String? = nil,
    font: NSFont = bodyFont,
    foregroundColor: NSColor = .labelColor,
    headIndent: CGFloat = 0,
    firstLineHeadIndent: CGFloat = 0,
    paragraphSpacing: CGFloat = 8
  ) {
    appendSeparator(to: result)
    let paragraphStyle = NSMutableParagraphStyle()
    paragraphStyle.lineSpacing = 2
    paragraphStyle.paragraphSpacing = paragraphSpacing
    paragraphStyle.headIndent = headIndent
    paragraphStyle.firstLineHeadIndent = firstLineHeadIndent

    if let prefix {
      appendText(
        prefix,
        to: result,
        font: font,
        foregroundColor: foregroundColor,
        paragraphStyle: paragraphStyle
      )
    }
    appendInline(
      text,
      to: result,
      font: font,
      foregroundColor: foregroundColor,
      paragraphStyle: paragraphStyle
    )
    appendText(
      "\n",
      to: result,
      font: font,
      foregroundColor: foregroundColor,
      paragraphStyle: paragraphStyle
    )
  }

  private static func appendCodeBlock(_ text: String, to result: NSMutableAttributedString) {
    appendSeparator(to: result)
    let paragraphStyle = NSMutableParagraphStyle()
    paragraphStyle.lineSpacing = 1
    paragraphStyle.paragraphSpacing = 8
    paragraphStyle.headIndent = 8
    paragraphStyle.firstLineHeadIndent = 8
    let attributes: [NSAttributedString.Key: Any] = [
      .font: codeFont,
      .foregroundColor: NSColor.labelColor,
      .backgroundColor: NSColor.textBackgroundColor,
      .paragraphStyle: paragraphStyle,
    ]
    result.append(NSAttributedString(string: "\(text)\n", attributes: attributes))
  }

  private static func appendInline(
    _ text: String,
    to result: NSMutableAttributedString,
    font: NSFont,
    foregroundColor: NSColor,
    paragraphStyle: NSParagraphStyle
  ) {
    let range = NSRange(location: 0, length: (text as NSString).length)
    var cursor = 0
    for match in inlinePattern.matches(in: text, range: range) {
      guard match.range.location >= cursor else {
        continue
      }
      if match.range.location > cursor {
        appendText(
          (text as NSString).substring(with: NSRange(
            location: cursor,
            length: match.range.location - cursor
          )),
          to: result,
          font: font,
          foregroundColor: foregroundColor,
          paragraphStyle: paragraphStyle
        )
      }

      let token = (text as NSString).substring(with: match.range)
      if token.hasPrefix("`") {
        appendText(
          String(token.dropFirst().dropLast()),
          to: result,
          font: codeFont,
          foregroundColor: foregroundColor,
          paragraphStyle: paragraphStyle,
          backgroundColor: NSColor.textBackgroundColor
        )
      } else if token.hasPrefix("**") || token.hasPrefix("__") {
        appendText(
          String(token.dropFirst(2).dropLast(2)),
          to: result,
          font: NSFontManager.shared.convert(font, toHaveTrait: .boldFontMask),
          foregroundColor: foregroundColor,
          paragraphStyle: paragraphStyle
        )
      } else if token.hasPrefix("*") || token.hasPrefix("_") {
        appendText(
          String(token.dropFirst().dropLast()),
          to: result,
          font: NSFontManager.shared.convert(font, toHaveTrait: .italicFontMask),
          foregroundColor: foregroundColor,
          paragraphStyle: paragraphStyle
        )
      } else if let link = parseLink(token) {
        appendText(
          link.label,
          to: result,
          font: font,
          foregroundColor: .linkColor,
          paragraphStyle: paragraphStyle,
          link: link.url
        )
      } else {
        appendText(
          token,
          to: result,
          font: font,
          foregroundColor: foregroundColor,
          paragraphStyle: paragraphStyle
        )
      }
      cursor = NSMaxRange(match.range)
    }

    if cursor < range.length {
      appendText(
        (text as NSString).substring(from: cursor),
        to: result,
        font: font,
        foregroundColor: foregroundColor,
        paragraphStyle: paragraphStyle
      )
    }
  }

  private static func appendText(
    _ text: String,
    to result: NSMutableAttributedString,
    font: NSFont,
    foregroundColor: NSColor,
    paragraphStyle: NSParagraphStyle,
    backgroundColor: NSColor? = nil,
    link: URL? = nil
  ) {
    var attributes: [NSAttributedString.Key: Any] = [
      .font: font,
      .foregroundColor: foregroundColor,
      .paragraphStyle: paragraphStyle,
    ]
    if let backgroundColor {
      attributes[.backgroundColor] = backgroundColor
    }
    if let link {
      attributes[.link] = link
      attributes[.underlineStyle] = NSUnderlineStyle.single.rawValue
    }
    result.append(NSAttributedString(string: text, attributes: attributes))
  }

  private static func appendSeparator(to result: NSMutableAttributedString) {
    guard result.length > 0, !result.string.hasSuffix("\n") else {
      return
    }
    result.append(NSAttributedString(string: "\n"))
  }

  private static func parseHeading(_ line: String) -> (level: Int, text: String)? {
    let hashes = line.prefix { $0 == "#" }
    guard (1 ... 6).contains(hashes.count), line.dropFirst(hashes.count).first == " " else {
      return nil
    }
    let text = line.dropFirst(hashes.count).trimmingCharacters(in: .whitespaces)
    return text.isEmpty ? nil : (hashes.count, text.trimmingTrailingHeadingMarker())
  }

  private static func parseBullet(_ line: String) -> String? {
    guard let marker = line.first, "-*+".contains(marker), line.dropFirst().first == " " else {
      return nil
    }
    let text = line.dropFirst().trimmingCharacters(in: .whitespaces)
    return text.isEmpty ? nil : text
  }

  private static func parseOrderedListItem(_ line: String) -> (marker: String, text: String)? {
    guard let dot = line.firstIndex(of: ".") else {
      return nil
    }
    let marker = String(line[..<dot])
    guard !marker.isEmpty,
          dot < line.index(before: line.endIndex),
          marker.allSatisfy({ $0.isNumber }),
          line[line.index(after: dot)] == " "
    else {
      return nil
    }
    let text = line[line.index(after: dot)...].trimmingCharacters(in: .whitespaces)
    return text.isEmpty ? nil : (marker, text)
  }

  private static func parseLink(_ token: String) -> (label: String, url: URL)? {
    guard token.first == "[",
          let closingLabel = token.firstIndex(of: "]"),
          token[token.index(after: closingLabel)] == "(",
          token.last == ")"
    else {
      return nil
    }
    let label = String(token[token.index(after: token.startIndex) ..< closingLabel])
    let urlStart = token.index(closingLabel, offsetBy: 2)
    let urlEnd = token.index(before: token.endIndex)
    guard let url = URL(string: String(token[urlStart ..< urlEnd])),
          ["http", "https"].contains(url.scheme?.lowercased())
    else {
      return nil
    }
    return (label, url)
  }

  private static func isFence(_ line: String) -> Bool {
    line.hasPrefix("```") || line.hasPrefix("~~~")
  }

  private static func isHorizontalRule(_ line: String) -> Bool {
    let compact = line.replacingOccurrences(of: " ", with: "")
    guard compact.count >= 3 else {
      return false
    }
    return compact.allSatisfy { $0 == "-" || $0 == "_" || $0 == "*" }
  }

  private static func isBlockStart(_ line: String) -> Bool {
    isFence(line) ||
      parseHeading(line) != nil ||
      parseBullet(line) != nil ||
      parseOrderedListItem(line) != nil ||
      line.hasPrefix(">") ||
      isHorizontalRule(line)
  }

  private static func headingFontSize(_ level: Int) -> CGFloat {
    switch level {
    case 1: return 18
    case 2: return 16
    case 3: return 14
    default: return 13
    }
  }
}

private extension String {
  func trimmingTrailingHeadingMarker() -> String {
    var value = trimmingCharacters(in: .whitespaces)
    while value.last == "#" {
      value.removeLast()
      value = value.trimmingCharacters(in: .whitespaces)
    }
    return value
  }
}
