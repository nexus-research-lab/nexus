/**
 * INPUT: Nexus 主题与模型生成的 HTML fragment。
 * OUTPUT: 增量渲染、终态脚本执行与 ready/error 回报的稳定 iframe 文档。
 * POS: 生成式 UI 的唯一 HTML 装配边界；宿主页不直接接触模型生成 DOM。
 */

export const GENERATIVE_UI_MESSAGE_SOURCE = "nexus-generative-ui";
export const GENERATIVE_UI_ERROR_MESSAGE = "nexus-widget-error";
export const GENERATIVE_UI_READY_MESSAGE = "nexus-widget-ready";
export const GENERATIVE_UI_RESIZE_MESSAGE = "nexus-widget-resize";
export const GENERATIVE_UI_UPDATE_MESSAGE = "nexus-widget-update";

export type GenerativeUITheme = "dark" | "light" | "rain";

const HTML_PARSER_MODULE = "https://esm.sh/htmlparser2@9.1.0";
const DOM_HANDLER_MODULE = "https://esm.sh/domhandler@5.0.3";
const DOM_SERIALIZER_MODULE = "https://esm.sh/dom-serializer@2.0.0";
const MORPHDOM_MODULE = "https://esm.sh/morphdom@2.7.4";

const BRIDGE_SCRIPT = `<script>
(() => {
  const root = () => document.getElementById("nexus-widget-root");
  let finalized = false;
  let finalizedHtml = "";
  let parserState = null;
  let previousHtml = "";
  let rendererPromise = null;
  let renderError = "";

  const postStatus = (type, detail = {}) => {
    window.parent.postMessage({
      source: "${GENERATIVE_UI_MESSAGE_SOURCE}",
      type,
      ...detail,
    }, "*");
  };

  const reportError = (error) => {
    const message = error instanceof Error
      ? error.message
      : typeof error === "string"
        ? error
        : "Unknown widget error";
    renderError = message;
    console.error("[Nexus widget] Render failed", error);
    postStatus("${GENERATIVE_UI_ERROR_MESSAGE}", { message });
  };

  window.addEventListener("error", (event) => {
    reportError(event.error ?? event.message);
  });
  window.addEventListener("unhandledrejection", (event) => {
    reportError(event.reason);
  });

  const reportSize = () => {
    postStatus("${GENERATIVE_UI_RESIZE_MESSAGE}", {
      height: Math.ceil(document.documentElement.scrollHeight),
    });
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

  const loadRenderer = () => {
    rendererPromise ??= Promise.all([
      import("${HTML_PARSER_MODULE}"),
      import("${DOM_HANDLER_MODULE}"),
      import("${DOM_SERIALIZER_MODULE}"),
      import("${MORPHDOM_MODULE}"),
    ]).then(([htmlparser, domhandler, serializer, morphdom]) => ({
      DomHandler: domhandler.DomHandler,
      Parser: htmlparser.Parser,
      morphdom: morphdom.default,
      serialize: serializer.default,
    }));
    return rendererPromise;
  };

  const removeScripts = (container) => {
    container.querySelectorAll("script").forEach((element) => element.remove());
  };

  const renderPartial = async (html) => {
    const current = root();
    if (!current || finalized) return;
    current.classList.add("streaming");
    current.classList.toggle("scripts-loading", html.includes("<script"));

    let renderer;
    try {
      renderer = await loadRenderer();
    } catch (error) {
      console.warn("[Nexus widget] Incremental renderer unavailable", error);
      if (finalized) return;
      current.innerHTML = html;
      removeScripts(current);
      requestAnimationFrame(reportSize);
      return;
    }
    if (finalized) return;

    if (!parserState || !html.startsWith(previousHtml)) {
      const handler = new renderer.DomHandler();
      parserState = {
        handler,
        parser: new renderer.Parser(handler, {
          decodeEntities: false,
          lowerCaseAttributeNames: false,
          lowerCaseTags: false,
          recognizeSelfClosing: true,
        }),
      };
      previousHtml = "";
    }

    const chunk = html.slice(previousHtml.length);
    if (chunk) {
      parserState.parser.write(chunk);
      previousHtml = html;
    }
    const next = current.cloneNode(false);
    next.innerHTML = renderer.serialize(parserState.handler.root.children, {
      encodeEntities: false,
    });
    removeScripts(next);
    renderer.morphdom(current, next, { childrenOnly: true });
    requestAnimationFrame(reportSize);
  };

  const executeScripts = async (container) => {
    const javascriptTypes = /^(?:text\\/(?:javascript(?:1\\.[0-5])?|x-javascript|ecmascript|x-ecmascript|jscript|livescript)|application\\/(?:javascript|x-javascript|ecmascript|x-ecmascript))$/;
    for (const script of Array.from(container.querySelectorAll("script:not([src])"))) {
      const type = (script.getAttribute("type") ?? "").trim().toLowerCase();
      if (type === "" || javascriptTypes.test(type)) {
        new Function(script.textContent ?? "");
      }
    }
    const phases = [
      container.querySelectorAll("script[src]:not([data-executed])"),
      container.querySelectorAll("script:not([src]):not([data-executed])"),
    ].map((scripts) => Array.from(scripts));

    for (const scripts of phases) {
      for (const previous of scripts) {
        if (!previous.isConnected) continue;
        const script = document.createElement("script");
        for (const attribute of Array.from(previous.attributes)) {
          script.setAttribute(attribute.name, attribute.value);
        }
        script.textContent = previous.textContent;
        script.dataset.executed = "";
        const type = (script.getAttribute("type") ?? "").trim().toLowerCase();
        const classic = type === "" || javascriptTypes.test(type);
        const waitsForLoad = script.hasAttribute("src")
          && (type === "module" || (classic && !script.hasAttribute("nomodule")));
        if (!waitsForLoad) {
          previous.replaceWith(script);
          continue;
        }
        const loaded = new Promise((resolve, reject) => {
          script.addEventListener("load", resolve, { once: true });
          script.addEventListener("error", () => {
            reject(new Error("Failed to load " + (previous.getAttribute("src") ?? "script")));
          }, { once: true });
        });
        previous.replaceWith(script);
        await loaded;
      }
    }
  };

  const renderFinal = async (html) => {
    if (finalized && finalizedHtml === html) {
      postStatus(
        renderError ? "${GENERATIVE_UI_ERROR_MESSAGE}" : "${GENERATIVE_UI_READY_MESSAGE}",
        renderError ? { message: renderError } : {},
      );
      return;
    }
    finalized = true;
    finalizedHtml = html;
    renderError = "";
    const current = root();
    if (!current) return;
    current.classList.remove("streaming");
    current.innerHTML = html;
    try {
      await executeScripts(current);
      if (!renderError) {
        postStatus("${GENERATIVE_UI_READY_MESSAGE}");
      }
    } finally {
      current.classList.remove("scripts-loading");
      requestAnimationFrame(reportSize);
    }
  };

  window.addEventListener("message", (event) => {
    const data = event.data;
    if (event.source !== window.parent || data?.type !== "${GENERATIVE_UI_UPDATE_MESSAGE}" || typeof data.html !== "string") {
      return;
    }
    if (data.final === true) {
      void renderFinal(data.html).catch(reportError);
      return;
    }
    void renderPartial(data.html);
  });
})();
</script>`;

const THEME_PALETTES = {
  dark: {
    accent: "#8ea4ff",
    accentContrast: "#08111d",
    background: "#0e151f",
    border: "rgba(171, 189, 214, 0.10)",
    chart1: "#8ea4ff",
    chart2: "#5fd3c5",
    chart3: "#f3b65f",
    chart4: "#f08aaa",
    chart5: "#aab7c6",
    muted: "rgba(165, 180, 198, 0.82)",
    surface: "rgba(255, 255, 255, 0.05)",
    surfaceHover: "rgba(255, 255, 255, 0.10)",
    text: "#edf3fb",
  },
  light: {
    accent: "#5b72ff",
    accentContrast: "#ffffff",
    background: "#fcfcfb",
    border: "rgba(11, 11, 11, 0.08)",
    chart1: "#5b72ff",
    chart2: "#0f8f83",
    chart3: "#b7791f",
    chart4: "#c75b7a",
    chart5: "#64748b",
    muted: "#5f5e5a",
    surface: "#f3f3f0",
    surfaceHover: "#f0efec",
    text: "#131313",
  },
  rain: {
    accent: "#9ab7ff",
    accentContrast: "#08111d",
    background: "#303a47",
    border: "rgba(205, 220, 240, 0.10)",
    chart1: "#9ab7ff",
    chart2: "#68d4ca",
    chart3: "#f2bd70",
    chart4: "#ee91ad",
    chart5: "#b7c5d6",
    muted: "rgba(183, 197, 214, 0.84)",
    surface: "rgba(255, 255, 255, 0.06)",
    surfaceHover: "rgba(255, 255, 255, 0.11)",
    text: "#eef3f8",
  },
} as const satisfies Record<GenerativeUITheme, Record<string, string>>;

function themeStyle(theme: GenerativeUITheme): string {
  const palette = THEME_PALETTES[theme];
  const dark = theme !== "light";
  return `<style>
:root {
  color-scheme: ${dark ? "dark" : "light"};
  --nexus-background: ${palette.background};
  --nexus-surface: ${palette.surface};
  --nexus-surface-hover: ${palette.surfaceHover};
  --nexus-text: ${palette.text};
  --nexus-muted: ${palette.muted};
  --nexus-border: ${palette.border};
  --nexus-accent: ${palette.accent};
  --nexus-accent-contrast: ${palette.accentContrast};
  --nexus-chart-1: ${palette.chart1};
  --nexus-chart-2: ${palette.chart2};
  --nexus-chart-3: ${palette.chart3};
  --nexus-chart-4: ${palette.chart4};
  --nexus-chart-5: ${palette.chart5};
  --nexus-font-sans: Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  --nexus-font-mono: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  --nexus-radius-md: 8px;
  --nexus-radius-lg: 12px;
}
* { box-sizing: border-box; }
html, body { background: transparent; }
body {
  margin: 0;
  overflow-x: hidden;
  color: var(--nexus-text);
  font-family: var(--nexus-font-sans);
}
button, input, select, textarea { color: inherit; font: inherit; }
#nexus-widget-root {
  min-height: 180px;
  position: relative;
  width: 100%;
  padding: 16px;
  transition: opacity 0.15s ease;
}
#nexus-widget-root.scripts-loading {
  opacity: 0.7;
  pointer-events: none;
}
#nexus-widget-root.scripts-loading::after {
  content: "";
  position: absolute;
  inset: 0;
  background: linear-gradient(100deg, transparent 25%, color-mix(in srgb, var(--nexus-text) 8%, transparent) 50%, transparent 75%);
  background-size: 200% 100%;
  animation: nexus-widget-loading 1.8s ease-in-out infinite;
  pointer-events: none;
}
@keyframes nexus-widget-loading {
  from { background-position: 200% 0; }
  to { background-position: -200% 0; }
}
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

function documentHead(theme: GenerativeUITheme): string {
  return `<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
${themeStyle(theme)}
${BRIDGE_SCRIPT}`;
}

export function buildGenerativeUIShellDocument(theme: GenerativeUITheme): string {
  return `<!doctype html><html><head>${documentHead(theme)}</head><body><main id="nexus-widget-root"></main></body></html>`;
}
