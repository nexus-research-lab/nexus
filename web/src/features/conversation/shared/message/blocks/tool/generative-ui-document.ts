/**
 * INPUT: Nexus 主题与模型生成的 HTML fragment。
 * OUTPUT: 流式空壳或最终可执行的完整 iframe 文档。
 * POS: 生成式 UI 的唯一 HTML 装配边界；宿主页不直接接触模型生成 DOM。
 */

export const GENERATIVE_UI_MESSAGE_SOURCE = "nexus-generative-ui";
export const GENERATIVE_UI_RESIZE_MESSAGE = "nexus-widget-resize";
export const GENERATIVE_UI_UPDATE_MESSAGE = "nexus-widget-update";

const MORPHDOM_SCRIPT =
  "https://cdn.jsdelivr.net/npm/morphdom@2.7.4/dist/morphdom-umd.min.js";

const BRIDGE_SCRIPT = `<script>
(() => {
  const root = () => document.getElementById("nexus-widget-root");
  const reportSize = () => {
    window.parent.postMessage({
      source: "${GENERATIVE_UI_MESSAGE_SOURCE}",
      type: "${GENERATIVE_UI_RESIZE_MESSAGE}",
      height: Math.ceil(document.documentElement.scrollHeight),
    }, "*");
  };
  const observe = () => {
    if (!document.body) return;
    new ResizeObserver(reportSize).observe(document.body);
    reportSize();
  };
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", observe, { once: true });
  } else {
    observe();
  }
  window.addEventListener("load", reportSize);
  window.addEventListener("message", (event) => {
    const data = event.data;
    if (event.source !== window.parent || data?.type !== "${GENERATIVE_UI_UPDATE_MESSAGE}" || typeof data.html !== "string") {
      return;
    }
    const current = root();
    if (!current) return;
    const next = document.createElement("main");
    next.id = "nexus-widget-root";
    next.inert = true;
    next.innerHTML = data.html;
    next.querySelectorAll("script, iframe, object, embed").forEach((element) => element.remove());
    next.querySelectorAll("*").forEach((element) => {
      Array.from(element.attributes).forEach((attribute) => {
        if (attribute.name.toLowerCase().startsWith("on")) {
          element.removeAttribute(attribute.name);
        }
      });
    });
    if (typeof window.morphdom === "function") {
      window.morphdom(current, next, { childrenOnly: true });
    } else {
      current.replaceChildren(...next.childNodes);
    }
    requestAnimationFrame(reportSize);
  });
})();
</script>`;

function themeStyle(dark: boolean): string {
  const palette = dark
    ? {
      accent: "#8ea4ff",
      accentContrast: "#08111d",
      background: "#0e151f",
      border: "rgba(171, 189, 214, 0.18)",
      muted: "#97a5b8",
      surface: "#151f2b",
      text: "#ebf0f6",
    }
    : {
      accent: "#5266e6",
      accentContrast: "#ffffff",
      background: "#fcfcfb",
      border: "rgba(11, 11, 11, 0.12)",
      muted: "#6d6b67",
      surface: "#ffffff",
      text: "#1d1d1f",
    };
  return `<style>
:root {
  color-scheme: ${dark ? "dark" : "light"};
  --nexus-background: ${palette.background};
  --nexus-surface: ${palette.surface};
  --nexus-text: ${palette.text};
  --nexus-muted: ${palette.muted};
  --nexus-border: ${palette.border};
  --nexus-accent: ${palette.accent};
  --nexus-accent-contrast: ${palette.accentContrast};
  --nexus-font-sans: Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
* { box-sizing: border-box; }
html { background: var(--nexus-background); }
body {
  margin: 0;
  overflow-x: hidden;
  background: var(--nexus-background);
  color: var(--nexus-text);
  font-family: var(--nexus-font-sans);
}
button, input, select, textarea { color: inherit; font: inherit; }
#nexus-widget-root { min-height: 180px; width: 100%; padding: 16px; }
#nexus-widget-root[inert] { pointer-events: none; }
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    scroll-behavior: auto !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
</style>`;
}

function documentHead(dark: boolean, streaming: boolean): string {
  return `<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
${themeStyle(dark)}
${streaming ? `<script defer src="${MORPHDOM_SCRIPT}"></script>` : ""}
${BRIDGE_SCRIPT}`;
}

export function buildGenerativeUIShellDocument(dark: boolean): string {
  return `<!doctype html><html><head>${documentHead(dark, true)}</head><body><main id="nexus-widget-root" inert></main></body></html>`;
}

export function buildGenerativeUIFinalDocument(
  widgetCode: string,
  dark: boolean,
): string {
  return `<!doctype html><html><head>${documentHead(dark, false)}</head><body><main id="nexus-widget-root">${widgetCode}</main></body></html>`;
}
