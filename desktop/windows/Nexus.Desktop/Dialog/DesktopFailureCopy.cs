// INPUT: A native failure stage and, for bridge failures, the requested operation kind.
// OUTPUT: Plain-language Chinese copy that states the failure, its data impact, and the next action.
// POS: Windows user-facing failure boundary; raw exceptions and filesystem paths stay in diagnostics only.

namespace Nexus.Desktop.Dialog;

internal static class DesktopFailureCopy
{
    internal const string StartupTitle = "Nexus 未能启动";
    internal const string UpdateCheckTitle = "无法检查 Nexus 更新";
    internal const string UpdateIncompleteTitle = "Nexus 更新未完成";
    internal const string AutomaticUpdateUnavailableTitle = "Nexus 暂时无法自动更新";

    internal static string StartupMessage(bool diagnosticsAvailable)
    {
        string recovery = diagnosticsAvailable
            ? "退出后重新打开 Nexus；如果仍然失败，联系支持人员并提供已生成的启动诊断报告。"
            : "退出后重新打开 Nexus；如果仍然失败，联系支持人员。";
        return string.Join(
            Environment.NewLine + Environment.NewLine,
            "Nexus 没有进入可用状态。现有本地数据是否受影响目前无法确认，不要删除或移动 Nexus 数据目录。",
            recovery);
    }

    internal static readonly string UpdateCheckMessage = string.Join(
        Environment.NewLine + Environment.NewLine,
        "Nexus 没有取得最新版本信息。当前安装、已有会话和文件未受影响，你可以继续使用 Nexus。",
        "稍后从应用菜单重新检查更新。");

    internal static readonly string UpdateIncompleteMessage = string.Join(
        Environment.NewLine + Environment.NewLine,
        "更新包没有完成下载、安全校验或安装准备，Nexus 没有切换到新版本。当前安装、已有会话和文件未受影响。",
        "可以稍后重新检查更新，或打开官方下载页手动安装。");

    internal static readonly string AutomaticUpdateUnavailableMessage = string.Join(
        Environment.NewLine + Environment.NewLine,
        "这个版本暂时不能通过应用内更新。自动安装没有开始，当前 Nexus、已有会话和文件未受影响。",
        "可以打开官方下载页手动安装，或稍后再处理。");

    internal static readonly string StateRootMigrationMessage = string.Join(
        Environment.NewLine + Environment.NewLine,
        "上次数据目录迁移没有完成，Nexus 已继续使用原目录，原有数据仍保留。新目录未启用，请勿手动合并或删除其中内容。",
        "先核对最近的会话和文件；需要重试时，从设置中选择一个空目录重新迁移。");

    internal static string BridgeMessage(string kind) => kind switch
    {
        "app.relocate_state_root" => string.Join(
            Environment.NewLine + Environment.NewLine,
            "数据目录迁移结果待确认。现有数据状态需要在重新打开 Nexus 后核对；不要移动或删除新旧数据目录。",
            "重新打开 Nexus，在设置中确认当前数据目录后，再决定是否重新迁移。"),
        "app.set_persistent_state" or "app.remove_persistent_state"
            or "app.set_global_shortcut_enabled" or "app.set_global_shortcut_accelerator"
            or "app.reset_global_shortcut_accelerator" => string.Join(
                Environment.NewLine + Environment.NewLine,
                "桌面设置结果待确认。对话、任务和文件未被这次设置操作修改。",
                "重新打开设置并核对当前状态，再决定是否重试。"),
        "app.export_logs" => string.Join(
            Environment.NewLine + Environment.NewLine,
            "日志导出没有完成。已有会话、任务和文件未受影响。",
            "检查所选位置是否已经生成日志文件；如果没有，再重新导出。"),
        "app.open_external_url" or "app.start_browser_extension_setup"
            or "app.open_workspace_file" or "app.open_route" => string.Join(
                Environment.NewLine + Environment.NewLine,
                "窗口或页面的打开结果待确认。Nexus 中已有的会话、任务和文件没有被修改。",
                "先检查屏幕上是否已经出现目标窗口或页面；没有出现时再重试。"),
        "app.get_app_version" or "app.get_state_root" or "app.choose_state_root"
            or "app.get_workspace_file_applications" or "app.get_persistent_state"
            or "app.get_global_shortcut_status" => string.Join(
                Environment.NewLine + Environment.NewLine,
                "Nexus 没有读取到这项桌面信息。读取失败不会修改已有设置、会话、任务或文件。",
                "保持 Nexus Desktop 运行并重新打开当前页面，然后再试一次。"),
        "app.start_update" => string.Join(
            Environment.NewLine + Environment.NewLine,
            "更新请求没有及时返回，更新状态待核对。当前版本、已有会话和文件没有因此改变。",
            "先等待原生更新窗口；没有出现时再从应用菜单检查更新。"),
        _ => string.Join(
            Environment.NewLine + Environment.NewLine,
            "桌面操作结果待确认。相关设置或内容需要到对应页面核对。",
            "返回相关页面核对当前状态，再决定是否重试。"),
    };
}
