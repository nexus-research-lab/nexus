import Darwin
import Foundation

enum SidecarPortAllocator {
  private static let desktopLoopbackPort = 34343

  static func allocate() throws -> Int {
    if isAvailable(desktopLoopbackPort) {
      return desktopLoopbackPort
    }
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
