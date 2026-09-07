// INPUT: Current HTML content, streaming state and an accessible file title.
// OUTPUT: A throttled opaque-origin iframe, or shared source preview while its head is incomplete.
// POS: HTML presentation; retains the storage shim and sandbox, without file I/O or parent access.
"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { cn } from "@/shared/ui/class-name";
import { UI_SOURCE_TEXT_CLASS_NAME } from "@/shared/ui/form/source-text-styles";
import { UI_PREVIEW_VIEWPORT_CLASS_NAME } from "@/shared/ui/layout/preview-viewport-styles";

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
  const lastCommitTsRef = useRef(0);

  useEffect(() => {
    if (isStreaming && shouldDeferHtmlPreviewCommit(content)) return;
    const commit = () => {
      lastCommitTsRef.current = Date.now();
      setCommittedContent(content);
    };
    const delay = isStreaming
      ? Math.max(0, HTML_PREVIEW_COMMIT_INTERVAL_MS - (Date.now() - lastCommitTsRef.current))
      : 0;
    if (delay === 0) {
      commit();
      return;
    }
    const timer = setTimeout(commit, delay);
    return () => clearTimeout(timer);
  }, [content, isStreaming]);

  const previewDocument = useMemo(
    () => committedContent === null
      ? ""
      : buildHtmlPreviewDocument(committedContent),
    [committedContent],
  );

  return {
    isWaitingForHead:
      isStreaming &&
      committedContent === null &&
      shouldDeferHtmlPreviewCommit(content),
    previewDocument,
  };
}

export function HtmlFilePreview({
  content,
  isStreaming = false,
  title,
}: {
  content: string;
  isStreaming?: boolean;
  title: string;
}) {
  const { isWaitingForHead, previewDocument } =
    useHtmlPreviewDocument(content, isStreaming);

  if (isWaitingForHead) {
    return (
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- The named source region needs a Tab stop for native keyboard scrolling while HTML is incomplete.
      <div aria-label={title} className={cn("h-full w-full bg-(--surface-panel-subtle-background) p-4", UI_PREVIEW_VIEWPORT_CLASS_NAME)} role="region" tabIndex={0}>
        <pre className={cn("whitespace-pre-wrap break-words text-(--text-muted)", UI_SOURCE_TEXT_CLASS_NAME)}>
          {content}
        </pre>
      </div>
    );
  }

  return (
    <iframe
      className="block h-full min-h-0 w-full border-0 bg-(--surface-paper-background)"
      sandbox="allow-downloads allow-forms allow-modals allow-popups allow-scripts"
      srcDoc={previewDocument}
      title={title}
    />
  );
}
