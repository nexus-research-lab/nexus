// INPUT: A native failure stage and whether startup diagnostics were persisted.
// OUTPUT: Plain-language Chinese copy that states the failure, its data impact, and the next action.
// POS: macOS user-facing failure boundary; raw errors and filesystem paths stay in diagnostics only.

enum DesktopFailureCopy {
  static let startupTitle = "Nexus 未能启动"
  static let mainWindowTitle = "Nexus 主窗口未能打开"
  static let updateCheckTitle = "无法检查 Nexus 更新"
  static let updateIncompleteTitle = "Nexus 更新未完成"
  static let automaticUpdateUnavailableTitle = "Nexus 暂时无法自动更新"

  static func startupMessage(diagnosticsAvailable: Bool) -> String {
    let recovery = diagnosticsAvailable
      ? "退出后重新打开 Nexus；如果仍然失败，联系支持人员并提供已生成的启动诊断报告。"
      : "退出后重新打开 Nexus；如果仍然失败，联系支持人员。"
    return """
    Nexus 没有进入可用状态。现有本地数据是否受影响目前无法确认，不要删除或移动 Nexus 数据目录。

    \(recovery)
    """
  }

  static let mainWindowMessage = """
  主窗口没有完成创建。这次打开窗口没有提交对话、任务或文件修改，已有内容未受影响。

  关闭此提示后再次从 Dock 或应用菜单打开主窗口；如果仍然失败，退出并重新打开 Nexus。
  """

  static let updateCheckMessage = """
  Nexus 没有取得最新版本信息。当前安装、已有会话和文件未受影响，你可以继续使用 Nexus。

  稍后从应用菜单重新检查更新。
  """

  static let updateIncompleteMessage = """
  更新包没有完成下载、安全校验或安装准备，Nexus 没有切换到新版本。当前安装、已有会话和文件未受影响。

  可以稍后重新检查更新，或打开官方下载页手动安装。
  """

  static let automaticUpdateUnavailableMessage = """
  这个版本暂时不能通过应用内更新。自动安装没有开始，当前 Nexus、已有会话和文件未受影响。

  可以打开官方下载页手动安装，或稍后再处理。
  """

  static let globalShortcutMessage = """
  这次全局快捷键设置没有生效。原有快捷键状态需要在设置中核对；Nexus 的其他功能和已有内容未受影响。

  在设置中换一个组合键，或关闭全局快捷键后再打开。
  """

  static let stateRootMigrationMessage = """
  上次数据目录迁移没有完成，Nexus 已继续使用原目录，原有数据仍保留。新目录未启用，请勿手动合并或删除其中内容。

  先核对最近的会话和文件；需要重试时，从设置中选择一个空目录重新迁移。
  """

  static let bridgeSecurityMessage = """
  这项桌面操作被安全检查阻止。已有内容没有被修改。

  返回 Nexus 界面后重新发起操作。
  """

  static func bridgeMessage(for kind: String) -> String {
    switch kind {
    case "app.relocate_state_root":
      return """
      数据目录迁移结果待确认。现有数据状态需要在重新打开 Nexus 后核对；不要移动或删除新旧数据目录。

      重新打开 Nexus，在设置中确认当前数据目录后，再决定是否重新迁移。
      """
    case "app.set_persistent_state", "app.remove_persistent_state",
         "app.set_global_shortcut_enabled", "app.set_global_shortcut_accelerator",
         "app.reset_global_shortcut_accelerator":
      return """
      桌面设置结果待确认。对话、任务和文件未被这次设置操作修改。

      重新打开设置并核对当前状态，再决定是否重试。
      """
    case "app.export_logs":
      return """
      日志导出没有完成。已有会话、任务和文件未受影响。

      检查所选位置是否已经生成日志文件；如果没有，再重新导出。
      """
    case "app.open_external_url", "app.start_browser_extension_setup",
         "app.open_workspace_file", "app.open_route":
      return """
      窗口或页面的打开结果待确认。Nexus 中已有的会话、任务和文件没有被修改。

      先检查屏幕上是否已经出现目标窗口或页面；没有出现时再重试。
      """
    case "app.get_app_version", "app.get_state_root", "app.choose_state_root",
         "app.get_workspace_file_applications", "app.get_persistent_state",
         "app.get_global_shortcut_status":
      return """
      Nexus 没有读取到这项桌面信息。读取失败不会修改已有设置、会话、任务或文件。

      保持 Nexus Desktop 运行并重新打开当前页面，然后再试一次。
      """
    case "app.start_update":
      return """
      更新请求没有及时返回，更新状态待核对。当前版本、已有会话和文件没有因此改变。

      先等待原生更新窗口；没有出现时再从应用菜单检查更新。
      """
    default:
      return """
      桌面操作结果待确认。相关设置或内容需要到对应页面核对。

      返回相关页面核对当前状态，再决定是否重试。
      """
    }
  }
}
