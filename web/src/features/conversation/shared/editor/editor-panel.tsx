"use client";

import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Eye,
  EyeOff,
  FileImage,
  FileSpreadsheet,
  FileText,
  FileWarning,
  LoaderCircle,
  Pencil,
  Save,
} from "lucide-react";

import {
  get_workspace_file_content_api,
  update_workspace_file_content_api,
  get_workspace_file_preview_url,
} from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import { TypewriterFileView } from "@/shared/ui/feedback/typewriter-file-view";
import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import { LazyMermaidView } from "@/features/conversation/shared/message/markdown/lazy-mermaid-view";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
  WorkspaceFileToolbarButton,
} from "./workspace-file-preview-chrome";

const SpreadsheetFilePreview = lazy(() => import("./spreadsheet-file-preview").then((module) => ({
  default: module.SpreadsheetFilePreview,
})));

const DocumentFilePreview = lazy(() => import("./document-file-preview").then((module) => ({
  default: module.DocumentFilePreview,
})));

const PresentationFilePreview = lazy(() => import("./presentation-file-preview").then((module) => ({
  default: module.PresentationFilePreview,
})));

// 文件类型检测
type WorkspaceFilePreviewKind =
  | "text"
  | "markdown"
  | "html"
  | "mermaid"
  | "pdf"
  | "image"
  | "spreadsheet"
  | "document"
  | "presentation"
  | "binary"
  | "unknown";

function get_file_type(path: string): WorkspaceFilePreviewKind {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  const textExtensions = new Set([
    "txt", "json", "jsonl", "yaml", "yml", "toml", "xml",
    "csv", "ts", "tsx", "js", "jsx", "mjs", "cjs", "py", "java", "go", "rs",
    "rb", "php", "sh", "bash", "zsh", "sql", "r", "css", "scss", "less",
    "log", "ini", "conf", "env", "dockerfile", "makefile", "cmake", "gradle",
    "proto", "graphql", "rst", "adoc"
  ]);
  const imageExtensions = new Set([
    "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif"
  ]);
  if (ext === "pdf") return "pdf";
  if (imageExtensions.has(ext)) return "image";
  if (ext === "xlsx") return "spreadsheet";
  if (ext === "docx") return "document";
  if (ext === "pptx") return "presentation";
  if (ext === "md" || ext === "markdown") return "markdown";
  if (ext === "html" || ext === "htm") return "html";
  if (ext === "mmd" || ext === "mermaid") return "mermaid";
  if (textExtensions.has(ext)) return "text";
  return "binary";
}

interface EditorPanelProps {
  agent_id: string;
  path: string | null;
  is_open: boolean;
  width_percent: number;
  embedded?: boolean;
  class_name?: string;
  is_preview_focused?: boolean;
  on_resize_start: () => void;
  on_toggle_preview_focus?: () => void;
}

function workspace_file_kind_label(file_type: WorkspaceFilePreviewKind): string {
  switch (file_type) {
    case "markdown":
      return "Markdown 预览";
    case "html":
      return "HTML 预览";
    case "mermaid":
      return "Mermaid 预览";
    case "text":
      return "文本预览";
    default:
      return "文件预览";
  }
}

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
    return content.replace(/<head(\s[^>]*)?>/i, (match) => `${match}${HTML_PREVIEW_STORAGE_SHIM}`);
  }
  if (/<html(\s[^>]*)?>/i.test(content)) {
    return content.replace(/<html(\s[^>]*)?>/i, (match) => `${match}<head>${HTML_PREVIEW_STORAGE_SHIM}</head>`);
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
  const [committed_content, setCommittedContent] = useState<string | null>(() => (
    is_streaming && should_defer_html_preview_commit(content) ? null : content
  ));
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
    () => committed_content === null ? "" : build_html_preview_document(committed_content),
    [committed_content],
  );

  return {
    has_committed_content: committed_content !== null,
    is_waiting_for_head: is_streaming && committed_content === null && should_defer_html_preview_commit(content),
    preview_document,
  };
}

function HtmlFilePreview({
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
  const { has_committed_content, is_waiting_for_head, preview_document } = useHtmlPreviewDocument(content, is_streaming);

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
        className="shrink-0 overflow-hidden rounded-[10px] border border-(--divider-subtle-color) bg-white shadow-[0_20px_60px_rgba(15,23,42,0.10)]"
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
            className="h-full w-full bg-white"
            sandbox="allow-downloads allow-forms allow-modals allow-popups allow-scripts"
            srcDoc={preview_document}
            title={title}
          />
        </div>
      </div>
    </div>
  );
}

function TextFilePreview({
  content,
  file_name,
  file_type,
  is_loading,
  is_streaming = false,
}: {
  content: string;
  file_name: string;
  file_type: WorkspaceFilePreviewKind;
  is_loading: boolean;
  is_streaming?: boolean;
}) {
  if (is_loading) {
    return <div className="font-mono text-sm leading-6 text-(--text-muted)">加载中...</div>;
  }

  if (file_type === "markdown") {
    return (
      <MarkdownRendererContent
        class_name="min-h-full"
        content={content}
        mermaid_show_header={false}
      />
    );
  }

  if (file_type === "mermaid") {
    return (
      <LazyMermaidView
        chart={content}
        class_name="min-h-full"
        constrain_height={false}
        show_header={false}
      />
    );
  }

  if (file_type === "html") {
    return <HtmlFilePreview content={content} is_streaming={is_streaming} title={file_name} />;
  }

  return (
    <pre className="message-cjk-code-font min-h-full whitespace-pre-wrap break-words text-sm leading-6 text-(--text-default)">
      {content}
    </pre>
  );
}

// PDF 预览组件
function PdfPreview({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  on_resize_start,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  is_preview_focused?: boolean;
  on_toggle_preview_focus?: () => void;
  on_resize_start: () => void;
  embedded?: boolean;
}) {
  const [is_loaded, setIsLoaded] = useState(false);
  const preview_url = get_workspace_file_preview_url(agent_id, path);

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileText className="h-3 w-3" />
              PDF 预览
            </span>
            {is_loaded ? (
              <span className="flex items-center gap-1 text-emerald-600">
                <Eye className="h-3 w-3" />
                已加载
              </span>
            ) : (
              <span className="flex items-center gap-1">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                加载中
              </span>
            )}
          </>
        )}
        title={file_name}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        <iframe
          className="h-full w-full"
          src={preview_url}
          title={file_name}
          onLoad={() => setIsLoaded(true)}
        />
      </div>
    </>
  );
}

// 图片预览组件
function ImagePreview({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  on_resize_start,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  is_preview_focused?: boolean;
  on_toggle_preview_focus?: () => void;
  on_resize_start: () => void;
  embedded?: boolean;
}) {
  const [is_loaded, setIsLoaded] = useState(false);
  const [has_error, setHasError] = useState(false);
  const preview_url = get_workspace_file_preview_url(agent_id, path);

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileImage className="h-3 w-3" />
              图片预览
            </span>
            {has_error ? (
              <span className="flex items-center gap-1 text-destructive">
                <EyeOff className="h-3 w-3" />
                加载失败
              </span>
            ) : is_loaded ? (
              <span className="flex items-center gap-1 text-emerald-600">
                <Eye className="h-3 w-3" />
                已加载
              </span>
            ) : (
              <span className="flex items-center gap-1">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                加载中
              </span>
            )}
          </>
        )}
        title={file_name}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)] p-6">
        {has_error ? (
          <div className="m-auto text-center">
            <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
            <p className="mt-4 text-sm font-medium text-(--text-strong)">图片加载失败</p>
            <p className="mt-2 text-xs text-(--text-soft)">请尝试下载文件</p>
          </div>
        ) : (
          <img
            className="max-h-full max-w-full rounded-lg object-contain"
            src={preview_url}
            alt={file_name}
            onLoad={() => setIsLoaded(true)}
            onError={() => { setIsLoaded(true); setHasError(true); }}
          />
        )}
      </div>
    </>
  );
}

// 二进制文件提示组件
function BinaryFilePlaceholder({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  is_preview_focused?: boolean;
  on_toggle_preview_focus?: () => void;
  embedded?: boolean;
}) {
  return (
    <>
      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <span className="flex items-center gap-1">
            <FileWarning className="h-3 w-3" />
            此文件类型不支持预览
          </span>
        )}
        title={file_name}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)] p-8">
        <div className="m-auto max-w-xs text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl border border-(--surface-panel-subtle-border) bg-(--card-default-background)">
            <FileWarning className="h-8 w-8 text-(--icon-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-strong)">不支持预览此文件</p>
          <p className="mt-2 text-xs leading-5 text-(--text-soft)">
            当前预览仅支持文本、PDF、图片、xlsx、docx 和 pptx 文件。您可以点击上方"下载"按钮来获取此文件。
          </p>
        </div>
      </div>
    </>
  );
}

function SpreadsheetPreviewFallback({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  on_resize_start,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  is_preview_focused?: boolean;
  on_toggle_preview_focus?: () => void;
  on_resize_start: () => void;
  embedded?: boolean;
}) {
  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileSpreadsheet className="h-3 w-3" />
              xlsx 预览
            </span>
            <span className="flex items-center gap-1">
              <LoaderCircle className="h-3 w-3 animate-spin" />
              加载预览组件中
            </span>
          </>
        )}
        title={file_name}
      />

      <div className="flex min-h-0 flex-1 items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center">
        <div className="max-w-xs">
          <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
          <p className="mt-3 text-sm font-medium text-(--text-strong)">正在加载 xlsx 预览组件</p>
        </div>
      </div>
    </>
  );
}

function DocumentPreviewFallback({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  on_resize_start,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  is_preview_focused?: boolean;
  on_toggle_preview_focus?: () => void;
  on_resize_start: () => void;
  embedded?: boolean;
}) {
  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileText className="h-3 w-3" />
              docx 预览
            </span>
            <span className="flex items-center gap-1">
              <LoaderCircle className="h-3 w-3 animate-spin" />
              加载预览组件中
            </span>
          </>
        )}
        title={file_name}
      />

      <div className="flex min-h-0 flex-1 items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center">
        <div className="max-w-xs">
          <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
          <p className="mt-3 text-sm font-medium text-(--text-strong)">正在加载 docx 预览组件</p>
        </div>
      </div>
    </>
  );
}

function PresentationPreviewFallback({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  on_resize_start,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  is_preview_focused?: boolean;
  on_toggle_preview_focus?: () => void;
  on_resize_start: () => void;
  embedded?: boolean;
}) {
  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileText className="h-3 w-3" />
              pptx 预览
            </span>
            <span className="flex items-center gap-1">
              <LoaderCircle className="h-3 w-3 animate-spin" />
              加载预览组件中
            </span>
          </>
        )}
        title={file_name}
      />

      <div className="flex min-h-0 flex-1 items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center">
        <div className="max-w-xs">
          <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
          <p className="mt-3 text-sm font-medium text-(--text-strong)">正在加载 pptx 预览组件</p>
        </div>
      </div>
    </>
  );
}

export function EditorPanel({
  agent_id,
  path,
  is_open,
  width_percent,
  embedded = false,
  class_name,
  is_preview_focused = false,
  on_resize_start,
  on_toggle_preview_focus,
}: EditorPanelProps) {
  const [draft_content, setDraftContent] = useState("");
  const [saved_content, setSavedContent] = useState("");
  const [is_loading, setIsLoading] = useState(false);
  const [is_saving, setIsSaving] = useState(false);
  const [is_editing, setIsEditing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editor_width, setEditorWidth] = useState(0);
  const editor_area_ref = useRef<HTMLDivElement>(null);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const file_states = useWorkspaceLiveStore((state) => state.file_states);

  // 检测文件类型
  const file_type = path ? get_file_type(path) : "unknown";
  const is_pdf = file_type === "pdf";
  const is_image = file_type === "image";
  const is_spreadsheet = file_type === "spreadsheet";
  const is_document = file_type === "document";
  const is_presentation = file_type === "presentation";
  const is_text = file_type === "text" || file_type === "markdown" || file_type === "html" || file_type === "mermaid";
  const is_binary = !is_text && !is_pdf && !is_image && !is_spreadsheet && !is_document && !is_presentation && file_type !== "unknown";
  const file_name = path ? path.split("/").at(-1) || "" : "";

  const live_state = path ? file_states[`${agent_id}:${path}`] : undefined;
  const is_external_writing = !!live_state && live_state.source !== "api" && live_state.status === "writing";
  const has_live_content = typeof live_state?.live_content === "string";
  const is_dirty = draft_content !== saved_content;

  const load_content_ref = useRef(false);

  // Track editor container width for pretext line measurement
  useEffect(() => {
    const el = editor_area_ref.current;
    if (!el) return;
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) setEditorWidth(entry.contentRect.width);
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const load_content = useCallback(async () => {
    if (!is_open || !path || !is_text) {
      return;
    }

    load_content_ref.current = false;
    setIsLoading(true);
    setError(null);
    try {
      const response = await get_workspace_file_content_api(agent_id, path);
      if (load_content_ref.current) return;
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (load_error) {
      if (load_content_ref.current) return;
      setError(load_error instanceof Error ? load_error.message : "读取文件失败");
    } finally {
      if (!load_content_ref.current) {
        setIsLoading(false);
      }
    }
  }, [agent_id, is_open, path, is_text]);

  // 首次打开 / 切换文件时加载内容
  useEffect(() => {
    load_content();
    return () => { load_content_ref.current = true; };
  }, [load_content]);

  useEffect(() => {
    setIsEditing(false);
  }, [path]);

  useEffect(() => {
    if (!is_editing) {
      return;
    }
    textarea_ref.current?.focus();
  }, [is_editing]);

  useEffect(() => {
    if (!is_open || !path || !live_state || !has_live_content || !is_text) {
      return;
    }

    if (live_state.source === "api" && is_saving) {
      return;
    }

    setDraftContent(live_state.live_content || "");
    if (live_state.status === "updated") {
      setSavedContent(live_state.live_content || "");
    }
  }, [has_live_content, is_open, is_saving, live_state, path, is_text]);

  useEffect(() => {
    if (!is_open || !path || !live_state || !is_text) {
      return;
    }

    if (live_state.status !== "updated" || typeof live_state.live_content === "string") {
      return;
    }

    void load_content();
  }, [is_open, live_state, load_content, path, is_text]);

  const enable_editing = useCallback(() => {
    if (is_external_writing) {
      return;
    }
    setIsEditing(true);
  }, [is_external_writing]);

  const handle_save = async () => {
    if (!path || !is_dirty || is_saving || !is_text) {
      return;
    }

    setIsSaving(true);
    setError(null);
    try {
      const response = await update_workspace_file_content_api(agent_id, path, draft_content);
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (save_error) {
      setError(save_error instanceof Error ? save_error.message : "保存文件失败");
    } finally {
      setIsSaving(false);
    }
  };

  if (!embedded && !is_open) {
    return null;
  }

  return (
    <section
      className={cn(
        "relative flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden transition-[width,opacity,transform,border-color] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
        embedded ? "border-0 bg-transparent shadow-none" : "border-l divider-subtle bg-transparent shadow-none",
        is_open ? "translate-x-0 opacity-100" : "pointer-events-none -translate-x-3 opacity-0",
        class_name,
      )}
      style={
        embedded
          ? { width: "100%" }
          : { width: is_open ? `${width_percent}%` : "0px" }
      }
    >
      {embedded && (!is_open || !path) ? (
        <div className="flex h-full flex-1 items-center justify-center px-8 text-center">
          <div className="max-w-sm">
            <p className="text-sm font-semibold uppercase tracking-[0.18em] text-muted-foreground">
              Workspace Preview
            </p>
            <p className="mt-3 text-sm leading-6 text-muted-foreground">
              从左侧选择一个文件，这里会显示对应内容。模型写入时，也会在这里实时同步。
            </p>
          </div>
        </div>
      ) : is_open && path ? (
        <>
          {is_pdf ? (
            <PdfPreview
              agent_id={agent_id}
              path={path}
              file_name={file_name}
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
              on_resize_start={on_resize_start}
              embedded={embedded}
            />
          ) : is_image ? (
            <ImagePreview
              agent_id={agent_id}
              path={path}
              file_name={file_name}
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
              on_resize_start={on_resize_start}
              embedded={embedded}
            />
          ) : is_spreadsheet ? (
            <Suspense
              fallback={(
                <SpreadsheetPreviewFallback
                  agent_id={agent_id}
                  path={path}
                  file_name={file_name}
                  is_preview_focused={is_preview_focused}
                  on_toggle_preview_focus={on_toggle_preview_focus}
                  on_resize_start={on_resize_start}
                  embedded={embedded}
                />
              )}
            >
              <SpreadsheetFilePreview
                agent_id={agent_id}
                path={path}
                file_name={file_name}
                is_preview_focused={is_preview_focused}
                on_toggle_preview_focus={on_toggle_preview_focus}
                on_resize_start={on_resize_start}
                embedded={embedded}
              />
            </Suspense>
          ) : is_document ? (
            <Suspense
              fallback={(
                <DocumentPreviewFallback
                  agent_id={agent_id}
                  path={path}
                  file_name={file_name}
                  is_preview_focused={is_preview_focused}
                  on_toggle_preview_focus={on_toggle_preview_focus}
                  on_resize_start={on_resize_start}
                  embedded={embedded}
                />
              )}
            >
              <DocumentFilePreview
                agent_id={agent_id}
                path={path}
                file_name={file_name}
                is_preview_focused={is_preview_focused}
                on_toggle_preview_focus={on_toggle_preview_focus}
                on_resize_start={on_resize_start}
                embedded={embedded}
              />
            </Suspense>
          ) : is_presentation ? (
            <Suspense
              fallback={(
                <PresentationPreviewFallback
                  agent_id={agent_id}
                  path={path}
                  file_name={file_name}
                  is_preview_focused={is_preview_focused}
                  on_toggle_preview_focus={on_toggle_preview_focus}
                  on_resize_start={on_resize_start}
                  embedded={embedded}
                />
              )}
            >
              <PresentationFilePreview
                agent_id={agent_id}
                path={path}
                file_name={file_name}
                is_preview_focused={is_preview_focused}
                on_toggle_preview_focus={on_toggle_preview_focus}
                on_resize_start={on_resize_start}
                embedded={embedded}
              />
            </Suspense>
          ) : is_binary ? (
            <BinaryFilePlaceholder
              agent_id={agent_id}
              path={path}
              file_name={file_name}
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
              embedded={embedded}
            />
          ) : (
            // 文本文件编辑器
            <>
              {!embedded ? (
                <ConversationResizeHandle
                  aria_label="调整编辑器宽度"
                  class_name="flex"
                  on_mouse_down={on_resize_start}
                />
              ) : null}

              <WorkspaceFilePreviewHeader
                actions={(
                  <>
                    <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
                    <WorkspaceFilePreviewFocusButton
                      is_preview_focused={is_preview_focused}
                      on_toggle_preview_focus={on_toggle_preview_focus}
                    />
                    <WorkspaceFileToolbarButton
                      on_click={() => {
                        if (is_editing) {
                          setIsEditing(false);
                          return;
                        }
                        enable_editing();
                      }}
                    >
                      {is_editing ? <Eye className="h-4 w-4" /> : <Pencil className="h-4 w-4" />}
                      <span>{is_editing ? "预览" : "编辑"}</span>
                    </WorkspaceFileToolbarButton>
                    <WorkspaceFileToolbarButton
                      disabled={!is_dirty || is_saving || is_external_writing}
                      on_click={() => void handle_save()}
                    >
                      <Save className="h-4 w-4" />
                      <span>{is_saving ? "保存中" : "保存"}</span>
                    </WorkspaceFileToolbarButton>
                  </>
                )}
                embedded={embedded}
                meta={(
                  <>
                    <span className="flex items-center gap-1">
                      <FileText className="h-3 w-3" />
                      {workspace_file_kind_label(file_type)}
                    </span>
                    {live_state && live_state.source !== "api" ? (
                      is_external_writing ? (
                        <>
                          <LoaderCircle className="h-3 w-3 shrink-0 animate-spin text-primary" />
                          <span className="truncate">模型正在实时写入该文件</span>
                        </>
                      ) : (
                        <>
                          <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500" />
                          <span className="truncate">
                            已同步最新内容
                            {live_state.diff_stats
                              ? ` · +${live_state.diff_stats.additions} -${live_state.diff_stats.deletions}`
                              : ""}
                          </span>
                        </>
                      )
                    ) : null}
                  </>
                )}
                title={file_name}
              />

              {error ? (
                <div className="px-4 py-3 text-sm text-destructive">{error}</div>
              ) : null}

              <div
                ref={editor_area_ref}
                className={cn(
                  "min-h-0 flex-1 overflow-hidden",
                  file_type === "html" && !is_editing ? "p-0" : "px-4 py-4",
                )}
              >
                {is_external_writing && file_type !== "html" ? (
                  <TypewriterFileView
                    content={draft_content}
                    container_width={editor_width > 0 ? editor_width - 40 : undefined}
                    class_name="h-full"
                  />
                ) : !is_editing && file_type === "html" ? (
                  <TextFilePreview
                    content={draft_content}
                    file_name={file_name}
                    file_type={file_type}
                    is_loading={is_loading}
                    is_streaming={is_external_writing}
                  />
                ) : !is_editing ? (
                  <div className="soft-scrollbar h-full overflow-auto">
                    <TextFilePreview
                      content={draft_content}
                      file_name={file_name}
                      file_type={file_type}
                      is_loading={is_loading}
                    />
                  </div>
                ) : (
                  <textarea
                    ref={textarea_ref}
                    className="soft-scrollbar h-full w-full resize-none border-0 bg-transparent p-0 font-mono text-sm leading-6 text-(--text-default) outline-none disabled:opacity-70"
                    disabled={is_loading}
                    onBlur={() => setIsEditing(false)}
                    onChange={(event) => setDraftContent(event.target.value)}
                    value={is_loading ? "加载中..." : draft_content}
                  />
                )}
              </div>
            </>
          )}
        </>
      ) : null}
    </section>
  );
}
