// INPUT: Kernel process identity and exact-target signal API.
// OUTPUT: Boot-bound audit identity, proven absence, or an explicit observation/signal error.
// POS: Never falls back from audit-token signaling to a bare PID signal.
import Darwin
import Foundation

struct SidecarProcessIdentity: Codable, Equatable {
  let pid: Int32
  let executablePath: String
  let bootSession: String
  let auditToken: [UInt32]

  func identifiesSameProcess(as other: SidecarProcessIdentity) -> Bool {
    pid == other.pid && bootSession == other.bootSession && auditToken == other.auditToken
  }
}

protocol SidecarProcessControlling {
  func inspect(pid: Int32) throws -> SidecarProcessIdentity?
  func signal(_ identity: SidecarProcessIdentity, _ signal: Int32) throws
}

struct SidecarRecoveryError: LocalizedError {
  let detail: String
  var errorDescription: String? {
    "无法确认旧的 Nexus 运行组件已安全退出。请退出旧版 Nexus 后重试；运行记录和本地数据已保留。"
  }
}

struct NativeSidecarProcessControl: SidecarProcessControlling {
  private typealias AuditSignal = @convention(c) (UnsafeMutablePointer<audit_token_t>?, Int32) -> Int32

  func inspect(pid: Int32) throws -> SidecarProcessIdentity? {
    guard pid > 1 else { throw failure("invalid process identifier") }
    if kill(pid, 0) != 0 {
      if errno == ESRCH { return nil }
      throw failure("process existence query: \(errno)")
    }
    do {
      let before = try auditToken(pid: pid)
      var buffer = [CChar](repeating: 0, count: 4096)
      guard proc_pidpath(pid, &buffer, UInt32(buffer.count)) > 0 else {
        // A failed path query is not evidence of absence.
        throw failure("process path query: \(errno)")
      }
      let after = try auditToken(pid: pid)
      guard before == after, before.count == 8, before[5] == UInt32(pid), before[7] != 0,
            before[1] == geteuid() else { throw failure("process identity changed or belongs to another user") }
      return SidecarProcessIdentity(pid: pid, executablePath: String(cString: buffer),
                                    bootSession: try bootSession(), auditToken: before)
    } catch {
      // A process can exit between existence and identity queries; only ESRCH proves absence.
      if kill(pid, 0) != 0 && errno == ESRCH { return nil }
      throw error
    }
  }

  func signal(_ identity: SidecarProcessIdentity, _ signal: Int32) throws {
    guard identity.pid > 1, identity.pid != getpid(), identity.auditToken.count == 8,
          identity.auditToken[5] == UInt32(identity.pid), identity.auditToken[7] != 0,
          identity.auditToken[1] == geteuid(), identity.bootSession == (try bootSession()) else {
      throw failure("invalid exact signal identity")
    }
    guard let symbol = dlsym(UnsafeMutableRawPointer(bitPattern: -2), "proc_signal_with_audittoken") else {
      throw failure("exact process signal API unavailable")
    }
    let send = unsafeBitCast(symbol, to: AuditSignal.self)
    var token = audit_token_t()
    withUnsafeMutableBytes(of: &token) { bytes in
      bytes.copyBytes(from: identity.auditToken.withUnsafeBytes { Array($0) })
    }
    errno = 0
    let result = send(&token, signal)
    try Self.validateSignalResult(result, errorNumber: errno)
  }

  // libproc returns positive errno values; only negative results consult errno.
  // A stale errno must not hide a failure or turn an absent exact target into an error.
  static func validateSignalResult(_ result: Int32, errorNumber: Int32) throws {
    let failureCode = result >= 0 ? result : (errorNumber != 0 ? errorNumber : EIO)
    if failureCode != 0 && failureCode != ESRCH {
      throw SidecarRecoveryError(detail: "exact process signal failed: \(failureCode)")
    }
  }

  private func auditToken(pid: Int32) throws -> [UInt32] {
    var port: mach_port_t = 0
    let acquired = task_name_for_pid(mach_task_self_, pid, &port)
    defer { if port != 0 { mach_port_deallocate(mach_task_self_, port) } }
    guard acquired == KERN_SUCCESS else { throw failure("task identity access: \(acquired)") }
    var token = audit_token_t()
    var count = mach_msg_type_number_t(MemoryLayout<audit_token_t>.size / MemoryLayout<integer_t>.size)
    let result = withUnsafeMutablePointer(to: &token) { pointer in
      pointer.withMemoryRebound(to: integer_t.self, capacity: Int(count)) {
        task_info(port, task_flavor_t(TASK_AUDIT_TOKEN), $0, &count)
      }
    }
    guard result == KERN_SUCCESS, count == 8 else { throw failure("audit identity query: \(result)") }
    return withUnsafeBytes(of: token) { Array($0.bindMemory(to: UInt32.self)) }
  }

  private func bootSession() throws -> String {
    var size = 128
    var buffer = [CChar](repeating: 0, count: size)
    guard sysctlbyname("kern.bootsessionuuid", &buffer, &size, nil, 0) == 0,
          size > 1, size <= buffer.count else { throw failure("boot identity query: \(errno)") }
    return String(cString: buffer)
  }

  private func failure(_ detail: String) -> SidecarRecoveryError {
    SidecarRecoveryError(detail: detail)
  }
}
