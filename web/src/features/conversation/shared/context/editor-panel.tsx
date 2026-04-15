"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  FileText, GripVertical, LoaderCircle, Minimize2, Save, FileWarning, Download, Eye, EyeOff, FileImage,
} from "lucide-react";

import { get_workspace_file_content_api, update_workspace_file_content_api, get_workspace_file_download_url } from "@/lib/agent-manage-api";
import { cn } from "@/lib/utils";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import { TypewriterFileView } from "@/shared/ui/feedback/typewriter-file-view";

// 文件类型检测
function get_file_type(path: string): "text" | "pdf" | "image" | "binary" | "unknown" {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  const textExtensions = new Set([
    "txt", "md", "markdown", "json", "jsonl", "yaml", "yml", "toml", "xml",
    "csv", "ts", "tsx", "js", "jsx", "mjs", "cjs", "py", "java", "go", "rs",
    "rb", "php", "sh", "bash", "zsh", "sql", "r", "html", "css", "scss", "less",
    "log", "ini", "conf", "env", "dockerfile", "makefile", "cmake", "gradle",
    "proto", "graphql", "rst", "adoc"
  ]);
  const imageExtensions = new Set([
    "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif"
  ]);
  if (ext === "pdf") return "pdf";
  if (imageExtensions.has(ext)) return "image";
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
  on_close: () => void;
  on_resize_start: () => void;
}

function EditorPanelHeader({
  actions,
  embedded,
  meta,
  title,
}: {
  actions: React.ReactNode;
  embedded?: boolean;
  meta?: React.ReactNode;
  title: string;
}) {
  if (embedded) {
    return (
      <div className="overflow-hidden border-b divider-subtle px-3 pt-0 pb-2">
        <div className="flex min-w-0 items-center justify-between gap-3">
          <p
            className="min-w-0 flex-1 truncate text-xs font-semibold uppercase leading-5 tracking-[0.16em] text-muted-foreground"
            title={title}
          >
            {title}
          </p>
          <div className="flex shrink-0 items-center gap-2 self-start">
            {actions}
          </div>
        </div>
        {meta ? (
          <div className="mt-1 flex min-w-0 items-center gap-2 text-[10px] text-muted-foreground">
            {meta}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className="flex h-14 min-w-0 items-center justify-between overflow-hidden border-b divider-subtle px-4">
      <div className="min-w-0 flex-1 overflow-hidden pr-3">
        <p
          className="w-full truncate text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground"
          title={title}
        >
          {title}
        </p>
        {meta ? (
          <div className="mt-1 flex min-w-0 items-center gap-2 text-[10px] text-muted-foreground">
            {meta}
          </div>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {actions}
      </div>
    </div>
  );
}

// PDF 预览组件
function PdfPreview({
  agent_id,
  path,
  file_name,
  on_close,
  on_resize_start,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  on_close: () => void;
  on_resize_start: () => void;
  embedded?: boolean;
}) {
  const [is_loaded, setIsLoaded] = useState(false);
  const download_url = get_workspace_file_download_url(agent_id, path);

  const handle_download = () => {
    window.open(download_url, "_blank");
  };

  return (
    <>
      {!embedded ? (
        <button
          aria-label="调整编辑器宽度"
          className="absolute -left-3 top-0 z-20 flex h-full w-6 cursor-col-resize items-center justify-center text-muted-foreground/60 transition-colors hover:text-primary"
          onMouseDown={on_resize_start}
          type="button"
        >
          <GripVertical className="h-4 w-4" />
        </button>
      ) : null}

      <EditorPanelHeader
        actions={(
          <>
            <button
              aria-label="下载 PDF"
              className="inline-flex h-7 items-center gap-1.5 rounded-lg border px-2.5 text-[11px] font-medium transition-all hover:bg-(--surface-interactive-hover-background)"
              style={{
                background: "var(--card-default-background)",
                borderColor: "var(--card-default-border)",
              }}
              onClick={handle_download}
              type="button"
            >
              <Download className="h-3.5 w-3.5" />
              <span>下载</span>
            </button>
            <button
              aria-label="关闭预览"
              className="inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
              onClick={on_close}
              type="button"
            >
              <Minimize2 className="h-4 w-4" />
            </button>
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

      <div className="flex-1 bg-[var(--surface-panel-subtle-background)]">
        <iframe
          className="h-full w-full"
          src={download_url}
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
  on_close,
  on_resize_start,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  on_close: () => void;
  on_resize_start: () => void;
  embedded?: boolean;
}) {
  const [is_loaded, setIsLoaded] = useState(false);
  const [has_error, setHasError] = useState(false);
  const download_url = get_workspace_file_download_url(agent_id, path);

  const handle_download = () => {
    window.open(download_url, "_blank");
  };

  return (
    <>
      {!embedded ? (
        <button
          aria-label="调整编辑器宽度"
          className="absolute -left-3 top-0 z-20 flex h-full w-6 cursor-col-resize items-center justify-center text-muted-foreground/60 transition-colors hover:text-primary"
          onMouseDown={on_resize_start}
          type="button"
        >
          <GripVertical className="h-4 w-4" />
        </button>
      ) : null}

      <EditorPanelHeader
        actions={(
          <>
            <button
              aria-label="下载图片"
              className="inline-flex h-7 items-center gap-1.5 rounded-lg border px-2.5 text-[11px] font-medium transition-all hover:bg-(--surface-interactive-hover-background)"
              style={{
                background: "var(--card-default-background)",
                borderColor: "var(--card-default-border)",
              }}
              onClick={handle_download}
              type="button"
            >
              <Download className="h-3.5 w-3.5" />
              <span>下载</span>
            </button>
            <button
              aria-label="关闭预览"
              className="inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
              onClick={on_close}
              type="button"
            >
              <Minimize2 className="h-4 w-4" />
            </button>
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

      <div className="flex-1 flex items-center justify-center bg-[var(--surface-panel-subtle-background)] p-6">
        {has_error ? (
          <div className="text-center">
            <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
            <p className="mt-4 text-sm font-medium text-(--text-strong)">图片加载失败</p>
            <p className="mt-2 text-xs text-(--text-soft)">请尝试下载文件</p>
          </div>
        ) : (
          <img
            className="max-h-full max-w-full rounded-lg object-contain"
            src={download_url}
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
  on_close,
  embedded,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  on_close: () => void;
  embedded?: boolean;
}) {
  const download_url = get_workspace_file_download_url(agent_id, path);

  const handle_download = () => {
    window.open(download_url, "_blank");
  };

  return (
    <>
      <EditorPanelHeader
        actions={(
          <>
            <button
              aria-label="下载文件"
              className="inline-flex h-7 items-center gap-1.5 rounded-lg border px-2.5 text-[11px] font-medium transition-all hover:bg-(--surface-interactive-hover-background)"
              style={{
                background: "var(--card-default-background)",
                borderColor: "var(--card-default-border)",
              }}
              onClick={handle_download}
              type="button"
            >
              <Download className="h-3.5 w-3.5" />
              <span>下载</span>
            </button>
            <button
              aria-label="关闭"
              className="inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
              onClick={on_close}
              type="button"
            >
              <Minimize2 className="h-4 w-4" />
            </button>
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

      <div className="flex-1 flex items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8">
        <div className="max-w-xs text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl border border-(--surface-panel-subtle-border) bg-(--card-default-background)">
            <FileWarning className="h-8 w-8 text-(--icon-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-strong)">不支持预览此文件</p>
          <p className="mt-2 text-xs leading-5 text-(--text-soft)">
            当前预览仅支持文本、PDF 和图片文件。您可以点击上方"下载"按钮来获取此文件。
          </p>
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
  on_close,
  on_resize_start,
}: EditorPanelProps) {
  const [draft_content, setDraftContent] = useState("");
  const [saved_content, setSavedContent] = useState("");
  const [is_loading, setIsLoading] = useState(false);
  const [is_saving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editor_width, setEditorWidth] = useState(0);
  const editor_area_ref = useRef<HTMLDivElement>(null);
  const file_states = useWorkspaceLiveStore((state) => state.file_states);

  // 检测文件类型
  const file_type = path ? get_file_type(path) : "unknown";
  const is_pdf = file_type === "pdf";
  const is_image = file_type === "image";
  const is_text = file_type === "text";
  const is_binary = !is_text && !is_pdf && !is_image && file_type !== "unknown";
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
              on_close={on_close}
              on_resize_start={on_resize_start}
              embedded={embedded}
            />
          ) : is_image ? (
            <ImagePreview
              agent_id={agent_id}
              path={path}
              file_name={file_name}
              on_close={on_close}
              on_resize_start={on_resize_start}
              embedded={embedded}
            />
          ) : is_binary ? (
            <BinaryFilePlaceholder
              agent_id={agent_id}
              path={path}
              file_name={file_name}
              on_close={on_close}
              embedded={embedded}
            />
          ) : (
            // 文本文件编辑器
            <>
              {!embedded ? (
                <button
                  aria-label="调整编辑器宽度"
                  className="absolute -left-3 top-0 z-20 flex h-full w-6 cursor-col-resize items-center justify-center text-muted-foreground/60 transition-colors hover:text-primary"
                  onMouseDown={on_resize_start}
                  type="button"
                >
                  <GripVertical className="h-4 w-4" />
                </button>
              ) : null}

              <EditorPanelHeader
                actions={(
                  <>
                    <button
                      disabled={!is_dirty || is_saving || is_external_writing}
                      className="inline-flex items-center gap-1.5 text-[11px] font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-[color:color-mix(in_srgb,var(--primary)_86%,var(--foreground)_14%)] disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                      onClick={() => void handle_save()}
                      type="button"
                    >
                      <Save className="h-4 w-4" />
                      {is_saving ? "保存中" : "保存"}
                    </button>
                    <button
                      aria-label="关闭编辑器"
                      className="inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
                      onClick={on_close}
                      type="button"
                    >
                      <Minimize2 className="h-4 w-4" />
                    </button>
                  </>
                )}
                embedded={embedded}
                meta={live_state && live_state.source !== "api" ? (
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
                ) : undefined}
                title={file_name}
              />

              {error ? (
                <div className="px-4 py-3 text-sm text-destructive">{error}</div>
              ) : null}

              <div ref={editor_area_ref} className="flex-1 px-4 py-4">
                {is_external_writing ? (
                  <TypewriterFileView
                    content={draft_content}
                    container_width={editor_width > 0 ? editor_width - 40 : undefined}
                    class_name="h-full"
                  />
                ) : (
                  <textarea
                    className="soft-scrollbar h-full w-full resize-none border-0 bg-transparent p-0 font-mono text-sm leading-6 text-(--text-default) outline-none disabled:opacity-70"
                    disabled={is_loading}
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
