import Darwin
import Foundation

enum SidecarPortAllocator {
  private static let desktopLoopbackPort = 34343
  private static let portRetryCount = 100
  private static let portRetryIntervalMicroseconds: useconds_t = 200_000

  static func allocate(startupTimeline: DesktopStartupTimeline? = nil) throws -> Int {
    if isAvailable(desktopLoopbackPort) {
      return desktopLoopbackPort
    }

    startupTimeline?.mark("sidecar.port_wait_begin", metadata: [
      "port": "\(desktopLoopbackPort)",
    ])
    // 旧 sidecar 正在响应退出信号时，给固定本地端口一个短暂释放窗口。
    for attempt in 1...portRetryCount {
      usleep(portRetryIntervalMicroseconds)
      if isAvailable(desktopLoopbackPort) {
        startupTimeline?.mark("sidecar.port_wait_ready", metadata: [
          "attempt": "\(attempt)",
          "port": "\(desktopLoopbackPort)",
        ])
        return desktopLoopbackPort
      }
    }

    startupTimeline?.mark("sidecar.port_unavailable", metadata: [
      "port": "\(desktopLoopbackPort)",
    ])
    throw DesktopShellError.portUnavailable(desktopLoopbackPort)
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
