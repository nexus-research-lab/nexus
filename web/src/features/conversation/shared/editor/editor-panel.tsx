"use client";

import { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import {
  Eye,
  FileText,
  LoaderCircle,
  Pencil,
  Save,
} from "lucide-react";

import {
  get_workspace_file_content_api,
  update_workspace_file_content_api,
} from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import { TypewriterFileView } from "@/shared/ui/feedback/typewriter-file-view";
import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import { LazyMermaidView } from "@/features/conversation/shared/message/markdown/lazy-mermaid-view";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import { HtmlFilePreview } from "./html-file-preview";
import {
  BinaryFilePlaceholder,
  ImagePreview,
  PdfPreview,
} from "./media-file-preview";
import {
  DocumentPreviewFallback,
  PresentationPreviewFallback,
  SpreadsheetPreviewFallback,
} from "./office-preview-fallbacks";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
  WorkspaceFileToolbarButton,
} from "./workspace-file-preview-chrome";
import {
  get_workspace_file_preview_kind,
  is_workspace_text_preview_kind,
  workspace_file_kind_label,
  type WorkspaceFilePreviewKind,
} from "./workspace-file-preview-kind";

const SpreadsheetFilePreview = lazy(() => import("./spreadsheet-file-preview").then((module) => ({
  default: module.SpreadsheetFilePreview,
})));

const DocumentFilePreview = lazy(() => import("./document-file-preview").then((module) => ({
  default: module.DocumentFilePreview,
})));

const PresentationFilePreview = lazy(() => import("./presentation-file-preview").then((module) => ({
  default: module.PresentationFilePreview,
})));

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

  const file_type = path ? get_workspace_file_preview_kind(path) : "unknown";
  const is_pdf = file_type === "pdf";
  const is_image = file_type === "image";
  const is_spreadsheet = file_type === "spreadsheet";
  const is_document = file_type === "document";
  const is_presentation = file_type === "presentation";
  const is_text = is_workspace_text_preview_kind(file_type);
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
                      title={is_editing ? "预览" : "编辑"}
                    >
                      {is_editing ? <Eye className="h-4 w-4" /> : <Pencil className="h-4 w-4" />}
                      <span className="max-xl:hidden">{is_editing ? "预览" : "编辑"}</span>
                    </WorkspaceFileToolbarButton>
                    <WorkspaceFileToolbarButton
                      disabled={!is_dirty || is_saving || is_external_writing}
                      on_click={() => void handle_save()}
                      title={is_saving ? "保存中" : "保存"}
                    >
                      <Save className="h-4 w-4" />
                      <span className="max-xl:hidden">{is_saving ? "保存中" : "保存"}</span>
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
                          <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-(--success)" />
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
