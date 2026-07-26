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

  function makeRequestID() {
    if (window.crypto && typeof window.crypto.randomUUID === "function") {
      return window.crypto.randomUUID();
    }
    return `desktop_${Date.now()}_${Math.random().toString(16).slice(2)}`;
  }

  function postToNative(channel, payload) {
    if (!window.chrome?.webview?.postMessage) {
      throw new Error("WebView2 bridge is unavailable");
    }
    window.chrome.webview.postMessage({ channel, payload });
  }

  function rejectPending(requestID, message) {
    const callback = pending.get(requestID);
    if (!callback) {
      return;
    }
    pending.delete(requestID);
    callback.reject(new Error(message || "Desktop bridge request failed"));
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
          reject(error);
          return;
        }
        window.setTimeout(() => {
          rejectPending(request.request_id, "Desktop bridge request timed out");
        }, 60000);
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
