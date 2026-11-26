// INPUT: Notification requests from the embedded web UI (title, body, tag, in-app route) and user clicks on them.
// OUTPUT: Native macOS notifications through UNUserNotificationCenter; a click hands the route back to whoever posted it.
// POS: The only owner of system notification authorization and delivery on macOS.

import Foundation
import UserNotifications

final class DesktopNotificationCenter: NSObject, UNUserNotificationCenterDelegate {
  static let shared = DesktopNotificationCenter()

  private static let routeKey = "nexus_route"
  // 点击交给最近一次发通知的页面；页面已经不在时由调用方兜底。
  private var onOpen: ((String) -> Void)?
  private var configured = false

  /// 只有作为 .app bundle 运行时系统才接受通知；swift run 等开发进程返回 false，不再请求授权。
  var isAvailable: Bool {
    Bundle.main.bundleIdentifier != nil && Bundle.main.bundleURL.pathExtension == "app"
  }

  func show(title: String, body: String, tag: String, route: String, onOpen: @escaping (String) -> Void) -> Bool {
    guard isAvailable else {
      return false
    }
    self.onOpen = onOpen
    let center = UNUserNotificationCenter.current()
    if !configured {
      configured = true
      center.delegate = self
    }
    // 首次调用触发系统授权弹窗；已拒绝时静默丢弃，站内未读不依赖通知。
    center.requestAuthorization(options: [.alert, .sound]) { granted, _ in
      guard granted else {
        return
      }
      let content = UNMutableNotificationContent()
      content.title = title
      content.body = body
      content.sound = .default
      content.userInfo = [Self.routeKey: route]
      // 同一 tag 的新通知替换旧通知。
      center.add(UNNotificationRequest(identifier: tag, content: content, trigger: nil))
    }
    return true
  }

  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification,
    withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
  ) {
    // 页面只在窗口不在前台时发通知；送达前用户切回来也照常展示，避免静默丢失。
    completionHandler([.banner, .list, .sound])
  }

  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse,
    withCompletionHandler completionHandler: @escaping () -> Void
  ) {
    let route = response.notification.request.content.userInfo[Self.routeKey] as? String ?? ""
    DispatchQueue.main.async {
      self.onOpen?(route)
      completionHandler()
    }
  }
}
