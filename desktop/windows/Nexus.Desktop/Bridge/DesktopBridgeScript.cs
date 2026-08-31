// INPUT: Embedded-page desktop bridge calls and native callbacks.
// OUTPUT: Promise settlement with safe delivery/timeout copy; raw JavaScript errors stay in diagnostics.
// POS: Windows bridge client boundary injected at document creation.

namespace Nexus.Desktop.Bridge;

internal static class DesktopBridgeScript
{
    public static string Make()
    {
        return """
(() => {
  if (window.__NEXUS_DESKTOP_BRIDGE__) {
    return;
  }

  const pending = new Map();
  const bridgeUnavailableMessage = "桌面操作无法发送，因为 Nexus 的本地连接尚未就绪。已有设置、会话、任务和文件未被修改。重新加载当前页面；如果仍然失败，重启 Nexus Desktop。";
  const bridgeFailedMessage = "桌面操作没有完成，是否已经生效目前无法确认。相关设置或内容是否发生变化也需要核对。返回相关页面确认当前状态，再决定是否重试。";

  function timeoutMessage(kind) {
    if ([
      "app.get_app_version",
      "app.get_state_root",
      "app.get_workspace_file_applications",
      "app.get_persistent_state",
      "app.get_global_shortcut_status",
    ].includes(kind)) {
      return "桌面信息没有及时返回。这次读取不会修改已有设置、会话、任务或文件。保持 Nexus Desktop 运行并重新加载当前页面。";
    }
    if (kind === "app.relocate_state_root") {
      return "数据目录迁移请求没有及时返回，是否已经开始目前无法确认。不要移动或删除新旧数据目录。重新打开 Nexus 并确认当前数据目录后，再决定是否重试。";
    }
    if ([
      "app.set_persistent_state",
      "app.remove_persistent_state",
      "app.set_global_shortcut_enabled",
      "app.set_global_shortcut_accelerator",
      "app.reset_global_shortcut_accelerator",
    ].includes(kind)) {
      return "桌面设置没有及时返回，是否已经生效目前无法确认。对话、任务和文件未被这项设置修改。重新打开设置并核对当前状态，再决定是否重试。";
    }
    if (kind === "app.start_update") {
      return "更新请求没有及时返回，更新流程可能已经开始。当前版本、已有会话和文件尚未因此改变。先等待原生更新窗口；没有出现时再从应用菜单检查更新。";
    }
    if (kind === "app.export_logs") {
      return "日志导出没有及时返回，无法确认文件是否已经生成。已有会话、任务和文件未受影响。先检查所选位置；没有日志文件时再重新导出。";
    }
    return "无法确认请求的窗口或页面是否已经打开。Nexus 中已有的会话、任务和文件没有被修改。先检查屏幕上是否已经出现目标窗口或页面；没有出现时再重试。";
  }

  function makeRequestID() {
    if (window.crypto && typeof window.crypto.randomUUID === "function") {
      return window.crypto.randomUUID();
    }
    return `desktop_${Date.now()}_${Math.random().toString(16).slice(2)}`;
  }

  function postToNative(channel, payload) {
    if (!window.chrome?.webview?.postMessage) {
      throw new Error(bridgeUnavailableMessage);
    }
    window.chrome.webview.postMessage({ channel, payload });
  }

  function rejectPending(requestID, message) {
    const callback = pending.get(requestID);
    if (!callback) {
      return;
    }
    pending.delete(requestID);
    callback.reject(new Error(message || bridgeFailedMessage));
  }

  window.webkit = window.webkit || {};
  window.webkit.messageHandlers = window.webkit.messageHandlers || {};
  window.webkit.messageHandlers.nexusDesktopLifecycle = {
    postMessage(message) {
      postToNative("nexusDesktopLifecycle", message);
    },
  };
  window.webkit.messageHandlers.nexusDesktop = {
    postMessage(message) {
      postToNative("nexusDesktop", message);
    },
  };

  window.__NEXUS_DESKTOP_BRIDGE__ = {
    invoke(message) {
      const request = {
        schema_version: 1,
        request_id: message?.request_id || makeRequestID(),
        kind: message?.kind || "",
        payload: message?.payload || {},
      };
      return new Promise((resolve, reject) => {
        pending.set(request.request_id, { resolve, reject });
        try {
          postToNative("nexusDesktop", request);
        } catch (error) {
          pending.delete(request.request_id);
          console.warn("[Nexus DesktopBridge] request delivery failed", error);
          reject(new Error(bridgeUnavailableMessage));
          return;
        }
        if (request.kind !== "app.choose_state_root") {
          window.setTimeout(() => {
            rejectPending(request.request_id, timeoutMessage(request.kind));
          }, 60000);
        }
      });
    },
    resolve(requestID, payload) {
      const callback = pending.get(requestID);
      if (!callback) {
        return;
      }
      pending.delete(requestID);
      callback.resolve(payload || {});
    },
    reject(requestID, message) {
      rejectPending(requestID, message);
    },
  };
})();
""";
    }
}
