import Foundation

enum DesktopBridgeScript {
  static func make() -> String {
    """
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

      function rejectPending(requestID, message) {
        const callback = pending.get(requestID);
        if (!callback) {
          return;
        }
        pending.delete(requestID);
        callback.reject(new Error(message || "Desktop bridge request failed"));
      }

      window.__NEXUS_DESKTOP_BRIDGE__ = {
        invoke(message) {
          const handler = window.webkit?.messageHandlers?.nexusDesktop;
          if (!handler) {
            return Promise.reject(new Error("Desktop bridge is unavailable"));
          }
          const request = {
            schema_version: 1,
            request_id: message?.request_id || makeRequestID(),
            kind: message?.kind || "",
            payload: message?.payload || {},
          };
          return new Promise((resolve, reject) => {
            pending.set(request.request_id, { resolve, reject });
            try {
              handler.postMessage(request);
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
    """
  }
}
