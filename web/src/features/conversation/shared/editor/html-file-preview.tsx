"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const HTML_PREVIEW_WIDTH = 1920;
const HTML_PREVIEW_HEIGHT = 1080;
const HTML_PREVIEW_PADDING = 32;
const HTML_PREVIEW_COMMIT_INTERVAL_MS = 250;

const HTML_PREVIEW_STORAGE_SHIM = `<script>
(() => {
  const createStorage = () => {
    const values = new Map();
    return {
      get length() { return values.size; },
      clear: () => values.clear(),
      getItem: (key) => values.has(String(key)) ? values.get(String(key)) : null,
      key: (index) => Array.from(values.keys())[Number(index)] ?? null,
      removeItem: (key) => values.delete(String(key)),
      setItem: (key, value) => values.set(String(key), String(value)),
    };
  };
  const installStorage = (name) => {
    try {
      const storage = window[name];
      const testKey = "__nexus_preview_storage_test__";
      storage.setItem(testKey, "1");
      storage.removeItem(testKey);
    } catch (_) {
      Object.defineProperty(window, name, {
        configurable: true,
        value: createStorage(),
      });
    }
  };
  installStorage("localStorage");
  installStorage("sessionStorage");
})();
</script>`;

function build_html_preview_document(content: string): string {
  if (/<head(\s[^>]*)?>/i.test(content)) {
    return content.replace(
      /<head(\s[^>]*)?>/i,
      (match) => `${match}${HTML_PREVIEW_STORAGE_SHIM}`,
    );
  }
  if (/<html(\s[^>]*)?>/i.test(content)) {
    return content.replace(
      /<html(\s[^>]*)?>/i,
      (match) => `${match}<head>${HTML_PREVIEW_STORAGE_SHIM}</head>`,
    );
  }
  return `${HTML_PREVIEW_STORAGE_SHIM}${content}`;
}

function is_html_preview_head_ready(content: string): boolean {
  const normalized = content.trim().toLowerCase();
  if (!/<(?:head|style)(?:\s|>)/i.test(normalized)) {
    return true;
  }

  return (
    normalized.includes("</head>") ||
    normalized.includes("</style>") ||
    normalized.includes("<body") ||
    normalized.includes("</body>") ||
    normalized.includes("</html>")
  );
}

function should_defer_html_preview_commit(content: string): boolean {
  return content.trim().length > 0 && !is_html_preview_head_ready(content);
}

function useHtmlPreviewDocument(content: string, is_streaming: boolean) {
  const [committed_content, setCommittedContent] = useState<string | null>(
    () => (is_streaming && should_defer_html_preview_commit(content)
      ? null
      : content),
  );
  const latest_content_ref = useRef(content);
  const last_commit_ts_ref = useRef(0);
  const pending_timer_ref = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clear_pending_timer = useCallback(() => {
    if (pending_timer_ref.current) {
      clearTimeout(pending_timer_ref.current);
      pending_timer_ref.current = null;
    }
  }, []);

  const commit_content = useCallback((next_content: string) => {
    clear_pending_timer();
    last_commit_ts_ref.current = Date.now();
    setCommittedContent(next_content);
  }, [clear_pending_timer]);

  useEffect(() => {
    latest_content_ref.current = content;
  }, [content]);

  useEffect(() => {
    if (!is_streaming) {
      commit_content(content);
      return;
    }

    if (should_defer_html_preview_commit(content)) {
      return;
    }

    const elapsed = Date.now() - last_commit_ts_ref.current;
    if (elapsed >= HTML_PREVIEW_COMMIT_INTERVAL_MS) {
      commit_content(content);
      return;
    }

    if (pending_timer_ref.current) {
      return;
    }

    pending_timer_ref.current = setTimeout(() => {
      pending_timer_ref.current = null;
      const latest_content = latest_content_ref.current;
      if (!should_defer_html_preview_commit(latest_content)) {
        commit_content(latest_content);
      }
    }, HTML_PREVIEW_COMMIT_INTERVAL_MS - elapsed);
  }, [commit_content, content, is_streaming]);

  useEffect(() => () => clear_pending_timer(), [clear_pending_timer]);

  const preview_document = useMemo(
    () => committed_content === null
      ? ""
      : build_html_preview_document(committed_content),
    [committed_content],
  );

  return {
    has_committed_content: committed_content !== null,
    is_waiting_for_head:
      is_streaming &&
      committed_content === null &&
      should_defer_html_preview_commit(content),
    preview_document,
  };
}

export function HtmlFilePreview({
  content,
  is_streaming = false,
  title,
}: {
  content: string;
  is_streaming?: boolean;
  title: string;
}) {
  const container_ref = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(1);
  const { has_committed_content, is_waiting_for_head, preview_document } =
    useHtmlPreviewDocument(content, is_streaming);

  useEffect(() => {
    const el = container_ref.current;
    if (!el) {
      return;
    }

    const update_scale = (width: number, height: number) => {
      const available_width = Math.max(width - HTML_PREVIEW_PADDING, 1);
      const available_height = Math.max(height - HTML_PREVIEW_PADDING, 1);
      setScale(
        Math.min(
          available_width / HTML_PREVIEW_WIDTH,
          available_height / HTML_PREVIEW_HEIGHT,
          1,
        ),
      );
    };

    const bounds = el.getBoundingClientRect();
    update_scale(bounds.width, bounds.height);

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry) {
        return;
      }
      update_scale(entry.contentRect.width, entry.contentRect.height);
    });

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  if (!has_committed_content && is_waiting_for_head) {
    return (
      <div className="soft-scrollbar h-full min-h-0 w-full overflow-auto bg-(--surface-panel-subtle-background) p-4">
        <pre className="message-cjk-code-font whitespace-pre-wrap break-words text-sm leading-6 text-(--text-muted)">
          {content}
        </pre>
      </div>
    );
  }

  return (
    <div
      ref={container_ref}
      className="soft-scrollbar flex h-full min-h-0 w-full items-start justify-center overflow-auto bg-(--surface-panel-subtle-background) p-4"
    >
      <div
        className="shrink-0 overflow-hidden rounded-[10px] border border-(--surface-paper-border) bg-(--surface-paper-background) shadow-(--surface-paper-shadow)"
        style={{
          height: HTML_PREVIEW_HEIGHT * scale,
          width: HTML_PREVIEW_WIDTH * scale,
        }}
      >
        <div
          style={{
            height: HTML_PREVIEW_HEIGHT,
            transform: `scale(${scale})`,
            transformOrigin: "top left",
            width: HTML_PREVIEW_WIDTH,
          }}
        >
          <iframe
            className="h-full w-full bg-(--surface-paper-background)"
            sandbox="allow-downloads allow-forms allow-modals allow-popups allow-scripts"
            srcDoc={preview_document}
            title={title}
          />
        </div>
      </div>
    </div>
  );
}
