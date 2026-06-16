import Foundation

enum DesktopShellError: LocalizedError {
  case projectRootNotFound
  case webDistNotFound(String)
  case sidecarExecutableNotFound
  case portUnavailable(Int)
  case sidecarExited
  case invalidRuntimeConfig
  case sessionTokenUnavailable
  case appAlreadyRunning
  case singleInstanceLockUnavailable
  case keychainReadFailed(OSStatus)
  case keychainWriteFailed(OSStatus)
  case keychainPayloadInvalid
  case keychainReadTimedOut
  case globalShortcutInvalid(String)
  case globalShortcutInstallFailed(OSStatus)
  case globalShortcutRegisterFailed(OSStatus)

  var errorDescription: String? {
    switch self {
    case .projectRootNotFound:
      return "未找到 Nexus 仓库根目录。请从仓库内运行 desktop/macos。"
    case .webDistNotFound(let path):
      return "未找到 Web 产物：\(path)。请先执行 scripts/desktop/run-macos-dev.sh 或构建 web/dist。"
    case .sidecarExecutableNotFound:
      return "未找到 Go sidecar。开发模式需要可用的 go 命令，打包模式需要 bundle 内的 nexus-server。"
    case .portUnavailable(let port):
      return "Nexus 桌面端本地端口 \(port) 已被占用，请关闭占用该端口的进程后重试。"
    case .sidecarExited:
      return "Go sidecar 在健康检查前退出。"
    case .invalidRuntimeConfig:
      return "无法生成桌面运行时配置。"
    case .sessionTokenUnavailable:
      return "无法生成桌面本地会话 token。"
    case .appAlreadyRunning:
      return "Nexus 已经在运行。"
    case .singleInstanceLockUnavailable:
      return "无法创建桌面应用单实例锁。"
    case .keychainReadFailed(let status):
      return "读取 macOS Keychain 失败：\(status)。"
    case .keychainWriteFailed(let status):
      return "写入 macOS Keychain 失败：\(status)。"
    case .keychainPayloadInvalid:
      return "macOS Keychain 中的桌面加密密钥格式不正确。"
    case .keychainReadTimedOut:
      return "读取 macOS Keychain 超时。"
    case .globalShortcutInvalid(let accelerator):
      return "全局快捷键格式无效：\(accelerator)。"
    case .globalShortcutInstallFailed(let status):
      return "安装全局快捷键事件处理器失败：\(status)。"
    case .globalShortcutRegisterFailed(let status):
      return "注册全局快捷键失败：\(status)。"
    }
  }
}
