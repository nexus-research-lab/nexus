import Darwin
import Foundation

enum SidecarPortAllocator {
  private static let preferredLoopbackPort = 34343
  private static let fallbackLoopbackPorts = 34344...34443

  static func allocate(startupTimeline: DesktopStartupTimeline? = nil) throws -> Int {
    if isAvailable(preferredLoopbackPort) {
      startupTimeline?.mark("sidecar.port_selected", metadata: [
        "port": "\(preferredLoopbackPort)",
        "source": "preferred",
      ])
      return preferredLoopbackPort
    }

    // A packaged build and an installed build can legitimately coexist while
    // they use different state roots. Do not make the second instance wait
    // twenty seconds and then fail just because the preferred port is busy.
    startupTimeline?.mark("sidecar.port_preferred_unavailable", metadata: [
      "port": "\(preferredLoopbackPort)",
    ])

    for port in fallbackLoopbackPorts where isAvailable(port) {
      startupTimeline?.mark("sidecar.port_selected", metadata: [
        "port": "\(port)",
        "source": "fallback",
        "preferred_port": "\(preferredLoopbackPort)",
      ])
      return port
    }

    startupTimeline?.mark("sidecar.port_unavailable", metadata: [
      "preferred_port": "\(preferredLoopbackPort)",
      "fallback_start": "\(fallbackLoopbackPorts.lowerBound)",
      "fallback_end": "\(fallbackLoopbackPorts.upperBound)",
    ])
    throw DesktopShellError.portUnavailable(preferredLoopbackPort)
  }

  private static func isAvailable(_ port: Int) -> Bool {
    let socketFD = socket(AF_INET, SOCK_STREAM, 0)
    if socketFD < 0 {
      return false
    }
    defer {
      close(socketFD)
    }

    // macOS 刚关闭的 HTTP 连接可能让不带 SO_REUSEADDR 的探测 socket 误判端口仍被占用。
    var reuseAddress: Int32 = 1
    _ = setsockopt(
      socketFD,
      SOL_SOCKET,
      SO_REUSEADDR,
      &reuseAddress,
      socklen_t(MemoryLayout<Int32>.size)
    )

    var address = sockaddr_in()
    address.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
    address.sin_family = sa_family_t(AF_INET)
    address.sin_port = in_port_t(port).bigEndian
    address.sin_addr = in_addr(s_addr: inet_addr("127.0.0.1"))

    let result = withUnsafePointer(to: &address) { pointer in
      pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { socketAddress in
        bind(socketFD, socketAddress, socklen_t(MemoryLayout<sockaddr_in>.size))
      }
    }
    return result == 0
  }
}
