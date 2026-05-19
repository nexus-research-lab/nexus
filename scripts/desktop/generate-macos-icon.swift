import AppKit
import Foundation

let rootURL = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
let resourcesURL = rootURL.appendingPathComponent("desktop/macos/Resources", isDirectory: true)
let buildURL = rootURL.appendingPathComponent("desktop/macos/.build/icon", isDirectory: true)
let iconsetURL = buildURL.appendingPathComponent("AppIcon.iconset", isDirectory: true)
let sourceURL = resourcesURL.appendingPathComponent("AppIconSource.png")
let outputURL = resourcesURL.appendingPathComponent("AppIcon.icns")

try FileManager.default.createDirectory(at: resourcesURL, withIntermediateDirectories: true)
try? FileManager.default.removeItem(at: buildURL)
try FileManager.default.createDirectory(at: iconsetURL, withIntermediateDirectories: true)

struct IconSize {
  let fileName: String
  let pixels: CGFloat
}

let iconSizes = [
  IconSize(fileName: "icon_16x16.png", pixels: 16),
  IconSize(fileName: "icon_16x16@2x.png", pixels: 32),
  IconSize(fileName: "icon_32x32.png", pixels: 32),
  IconSize(fileName: "icon_32x32@2x.png", pixels: 64),
  IconSize(fileName: "icon_128x128.png", pixels: 128),
  IconSize(fileName: "icon_128x128@2x.png", pixels: 256),
  IconSize(fileName: "icon_256x256.png", pixels: 256),
  IconSize(fileName: "icon_256x256@2x.png", pixels: 512),
  IconSize(fileName: "icon_512x512.png", pixels: 512),
  IconSize(fileName: "icon_512x512@2x.png", pixels: 1024),
]

guard let sourceImage = NSImage(contentsOf: sourceURL) else {
  fatalError("missing app icon source: \(sourceURL.path)")
}

func drawIcon(pixels: CGFloat) -> NSImage {
  let image = NSImage(size: NSSize(width: pixels, height: pixels))
  image.lockFocus()

  NSColor.clear.setFill()
  NSRect(x: 0, y: 0, width: pixels, height: pixels).fill()
  sourceImage.draw(
    in: NSRect(x: 0, y: 0, width: pixels, height: pixels),
    from: .zero,
    operation: .sourceOver,
    fraction: 1
  )

  image.unlockFocus()
  return image
}

func writePNG(image: NSImage, pixels: CGFloat, to url: URL) throws {
  let bitmap = NSBitmapImageRep(
    bitmapDataPlanes: nil,
    pixelsWide: Int(pixels),
    pixelsHigh: Int(pixels),
    bitsPerSample: 8,
    samplesPerPixel: 4,
    hasAlpha: true,
    isPlanar: false,
    colorSpaceName: .deviceRGB,
    bytesPerRow: 0,
    bitsPerPixel: 0
  )!
  bitmap.size = NSSize(width: pixels, height: pixels)

  NSGraphicsContext.saveGraphicsState()
  NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
  NSColor.clear.setFill()
  NSRect(x: 0, y: 0, width: pixels, height: pixels).fill()
  image.draw(in: NSRect(x: 0, y: 0, width: pixels, height: pixels))
  NSGraphicsContext.restoreGraphicsState()

  guard let data = bitmap.representation(using: .png, properties: [:]) else {
    throw NSError(domain: "NexusIcon", code: 1, userInfo: [NSLocalizedDescriptionKey: "无法生成 PNG"])
  }
  try data.write(to: url)
}

for iconSize in iconSizes {
  try writePNG(
    image: drawIcon(pixels: iconSize.pixels),
    pixels: iconSize.pixels,
    to: iconsetURL.appendingPathComponent(iconSize.fileName)
  )
}

let process = Process()
process.executableURL = URL(fileURLWithPath: "/usr/bin/iconutil")
process.arguments = [
  "-c",
  "icns",
  iconsetURL.path,
  "-o",
  outputURL.path,
]
try process.run()
process.waitUntilExit()
guard process.terminationStatus == 0 else {
  throw NSError(domain: "NexusIcon", code: Int(process.terminationStatus), userInfo: [NSLocalizedDescriptionKey: "iconutil 生成失败"])
}

print("Generated \(outputURL.path)")
