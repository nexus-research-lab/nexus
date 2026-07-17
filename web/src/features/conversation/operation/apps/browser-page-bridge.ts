/**
 * INPUT: postMessage payload emitted by a sandboxed Navi proxy page.
 * OUTPUT: A validated internal navigation or load-error message.
 * POS: Trust boundary between untrusted page HTML and the Navi React controller.
 */

export type BrowserPageBridgeMessage =
  | { type: "navigate"; url: string }
  | { status: number; type: "load-error" };

export function readBrowserPageBridgeMessage(value: unknown): BrowserPageBridgeMessage | null {
  if (!is_record(value) || value.source !== "nexus-navi-proxy") {
    return null;
  }
  if (value.type === "navigate" && typeof value.url === "string") {
    try {
      const url = new URL(value.url);
      return url.protocol === "http:" || url.protocol === "https:"
        ? { type: "navigate", url: url.href }
        : null;
    } catch {
      return null;
    }
  }
  if (value.type === "load-error" && typeof value.status === "number" && Number.isInteger(value.status)) {
    return { status: Number(value.status), type: "load-error" };
  }
  return null;
}

function is_record(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
