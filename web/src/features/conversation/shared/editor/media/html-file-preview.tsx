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

function buildHtmlPreviewDocument(content: string): string {
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

function isHtmlPreviewHeadReady(content: string): boolean {
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

function shouldDeferHtmlPreviewCommit(content: string): boolean {
  return content.trim().length > 0 && !isHtmlPreviewHeadReady(content);
}

function useHtmlPreviewDocument(content: string, isStreaming: boolean) {
  const [committedContent, setCommittedContent] = useState<string | null>(
    () => (isStreaming && shouldDeferHtmlPreviewCommit(content)
      ? null
      : content),
  );
  const latestContentRef = useRef(content);
  const lastCommitTsRef = useRef(0);
  const pendingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearPendingTimer = useCallback(() => {
    if (pendingTimerRef.current) {
      clearTimeout(pendingTimerRef.current);
      pendingTimerRef.current = null;
    }
  }, []);

  const commitContent = useCallback((nextContent: string) => {
    clearPendingTimer();
    lastCommitTsRef.current = Date.now();
    setCommittedContent(nextContent);
  }, [clearPendingTimer]);

  useEffect(() => {
    latestContentRef.current = content;
  }, [content]);

  useEffect(() => {
    if (!isStreaming) {
      commitContent(content);
      return;
    }

    if (shouldDeferHtmlPreviewCommit(content)) {
      return;
    }

    const elapsed = Date.now() - lastCommitTsRef.current;
    if (elapsed >= HTML_PREVIEW_COMMIT_INTERVAL_MS) {
      commitContent(content);
      return;
    }

    if (pendingTimerRef.current) {
      return;
    }

    pendingTimerRef.current = setTimeout(() => {
      pendingTimerRef.current = null;
      const latestContent = latestContentRef.current;
      if (!shouldDeferHtmlPreviewCommit(latestContent)) {
        commitContent(latestContent);
      }
    }, HTML_PREVIEW_COMMIT_INTERVAL_MS - elapsed);

    return () => clearPendingTimer();
  }, [clearPendingTimer, commitContent, content, isStreaming]);

  useEffect(() => () => clearPendingTimer(), [clearPendingTimer]);

  const previewDocument = useMemo(
    () => committedContent === null
      ? ""
      : buildHtmlPreviewDocument(committedContent),
    [committedContent],
  );

  return {
    has_committedContent: committedContent !== null,
    is_waiting_for_head:
      isStreaming &&
      committedContent === null &&
      shouldDeferHtmlPreviewCommit(content),
    preview_document: previewDocument,
  };
}

export function HtmlFilePreview({
  content,
  isStreaming: isStreaming = false,
  title,
}: {
  content: string;
  isStreaming?: boolean;
  title: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(1);
  const { has_committedContent, is_waiting_for_head: isWaitingForHead, preview_document: previewDocument } =
    useHtmlPreviewDocument(content, isStreaming);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) {
      return;
    }

    const updateScale = (width: number, height: number) => {
      const availableWidth = Math.max(width - HTML_PREVIEW_PADDING, 1);
      const availableHeight = Math.max(height - HTML_PREVIEW_PADDING, 1);
      setScale(
        Math.min(
          availableWidth / HTML_PREVIEW_WIDTH,
          availableHeight / HTML_PREVIEW_HEIGHT,
          1,
        ),
      );
    };

    const bounds = el.getBoundingClientRect();
    updateScale(bounds.width, bounds.height);

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry) {
        return;
      }
      updateScale(entry.contentRect.width, entry.contentRect.height);
    });

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  if (!has_committedContent && isWaitingForHead) {
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
      ref={containerRef}
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
            srcDoc={previewDocument}
            title={title}
          />
        </div>
      </div>
    </div>
  );
}
