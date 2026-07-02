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
import { useResettableState } from "@/hooks/ui/use-resettable-state";
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
  file_name: fileName,
  file_type: fileType,
  is_loading: isLoading,
  is_streaming: isStreaming = false,
}: {
  content: string;
  file_name: string;
  file_type: WorkspaceFilePreviewKind;
  is_loading: boolean;
  is_streaming?: boolean;
}) {
  if (isLoading) {
    return <div className="font-mono text-sm leading-6 text-(--text-muted)">加载中...</div>;
  }

  if (fileType === "markdown") {
    return (
      <MarkdownRendererContent
        class_name="min-h-full"
        content={content}
        mermaid_show_header={false}
      />
    );
  }

  if (fileType === "mermaid") {
    return (
      <LazyMermaidView
        chart={content}
        class_name="min-h-full"
        constrain_height={false}
        show_header={false}
      />
    );
  }

  if (fileType === "html") {
    return <HtmlFilePreview content={content} is_streaming={isStreaming} title={fileName} />;
  }

  return (
    <pre className="message-cjk-code-font min-h-full whitespace-pre-wrap break-words text-sm leading-6 text-(--text-default)">
      {content}
    </pre>
  );
}

export function EditorPanel({
  agent_id: agentId,
  path,
  is_open: isOpen,
  width_percent: widthPercent,
  embedded = false,
  class_name: className,
  is_preview_focused: isPreviewFocused = false,
  on_resize_start: onResizeStart,
  on_toggle_preview_focus: onTogglePreviewFocus,
}: EditorPanelProps) {
  const [draftContent, setDraftContent] = useState("");
  const [savedContent, setSavedContent] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isEditing, setIsEditing] = useResettableState(false, path ?? "");
  const [error, setError] = useState<string | null>(null);
  const [editorWidth, setEditorWidth] = useState(0);
  const editorAreaRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileStates = useWorkspaceLiveStore((state) => state.file_states);

  const fileType = path ? get_workspace_file_preview_kind(path) : "unknown";
  const isPdf = fileType === "pdf";
  const isImage = fileType === "image";
  const isSpreadsheet = fileType === "spreadsheet";
  const isDocument = fileType === "document";
  const isPresentation = fileType === "presentation";
  const isText = is_workspace_text_preview_kind(fileType);
  const isBinary = !isText && !isPdf && !isImage && !isSpreadsheet && !isDocument && !isPresentation && fileType !== "unknown";
  const fileName = path ? path.split("/").at(-1) || "" : "";

  const liveState = path ? fileStates[`${agentId}:${path}`] : undefined;
  const isExternalWriting = !!liveState && liveState.source !== "api" && liveState.status === "writing";
  const hasLiveContent = typeof liveState?.live_content === "string";
  const isDirty = draftContent !== savedContent;

  const loadContentRef = useRef(false);

  // Track editor container width for pretext line measurement
  useEffect(() => {
    const el = editorAreaRef.current;
    if (!el) return;
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) setEditorWidth(entry.contentRect.width);
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const loadContent = useCallback(async () => {
    if (!isOpen || !path || !isText) {
      return;
    }

    loadContentRef.current = false;
    setIsLoading(true);
    setError(null);
    try {
      const response = await get_workspace_file_content_api(agentId, path);
      if (loadContentRef.current) return;
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (loadError) {
      if (loadContentRef.current) return;
      setError(loadError instanceof Error ? loadError.message : "读取文件失败");
    } finally {
      if (!loadContentRef.current) {
        setIsLoading(false);
      }
    }
  }, [agentId, isOpen, path, isText]);

  // 首次打开 / 切换文件时加载内容
  useEffect(() => {
    loadContent();
    return () => { loadContentRef.current = true; };
  }, [loadContent]);

  useEffect(() => {
    if (!isEditing) {
      return;
    }
    textareaRef.current?.focus();
  }, [isEditing]);

  useEffect(() => {
    if (!isOpen || !path || !liveState || !hasLiveContent || !isText) {
      return;
    }

    if (liveState.source === "api" && isSaving) {
      return;
    }

    setDraftContent(liveState.live_content || "");
    if (liveState.status === "updated") {
      setSavedContent(liveState.live_content || "");
    }
  }, [hasLiveContent, isOpen, isSaving, liveState, path, isText]);

  useEffect(() => {
    if (!isOpen || !path || !liveState || !isText) {
      return;
    }

    if (liveState.status !== "updated" || typeof liveState.live_content === "string") {
      return;
    }

    void loadContent();
  }, [isOpen, liveState, loadContent, path, isText]);

  const enableEditing = useCallback(() => {
    if (isExternalWriting) {
      return;
    }
    setIsEditing(true);
  }, [isExternalWriting]);

  const handleSave = async () => {
    if (!path || !isDirty || isSaving || !isText) {
      return;
    }

    setIsSaving(true);
    setError(null);
    try {
      const response = await update_workspace_file_content_api(agentId, path, draftContent);
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存文件失败");
    } finally {
      setIsSaving(false);
    }
  };

  if (!embedded && !isOpen) {
    return null;
  }

  return (
    <section
      className={cn(
        "relative flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden transition-[width,opacity,transform,border-color] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
        embedded ? "border-0 bg-transparent shadow-none" : "border-l divider-subtle bg-transparent shadow-none",
        isOpen ? "translate-x-0 opacity-100" : "pointer-events-none -translate-x-3 opacity-0",
        className,
      )}
      style={
        embedded
          ? { width: "100%" }
          : { width: isOpen ? `${widthPercent}%` : "0px" }
      }
    >
      {embedded && (!isOpen || !path) ? (
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
      ) : isOpen && path ? (
        <>
          {isPdf ? (
            <PdfPreview
              agent_id={agentId}
              path={path}
              file_name={fileName}
              is_preview_focused={isPreviewFocused}
              on_toggle_preview_focus={onTogglePreviewFocus}
              on_resize_start={onResizeStart}
              embedded={embedded}
            />
          ) : isImage ? (
            <ImagePreview
              agent_id={agentId}
              path={path}
              file_name={fileName}
              is_preview_focused={isPreviewFocused}
              on_toggle_preview_focus={onTogglePreviewFocus}
              on_resize_start={onResizeStart}
              embedded={embedded}
            />
          ) : isSpreadsheet ? (
            <Suspense
              fallback={(
                <SpreadsheetPreviewFallback
                  agent_id={agentId}
                  path={path}
                  file_name={fileName}
                  is_preview_focused={isPreviewFocused}
                  on_toggle_preview_focus={onTogglePreviewFocus}
                  on_resize_start={onResizeStart}
                  embedded={embedded}
                />
              )}
            >
              <SpreadsheetFilePreview
                agent_id={agentId}
                path={path}
                file_name={fileName}
                is_preview_focused={isPreviewFocused}
                on_toggle_preview_focus={onTogglePreviewFocus}
                on_resize_start={onResizeStart}
                embedded={embedded}
              />
            </Suspense>
          ) : isDocument ? (
            <Suspense
              fallback={(
                <DocumentPreviewFallback
                  agent_id={agentId}
                  path={path}
                  file_name={fileName}
                  is_preview_focused={isPreviewFocused}
                  on_toggle_preview_focus={onTogglePreviewFocus}
                  on_resize_start={onResizeStart}
                  embedded={embedded}
                />
              )}
            >
              <DocumentFilePreview
                agent_id={agentId}
                path={path}
                file_name={fileName}
                is_preview_focused={isPreviewFocused}
                on_toggle_preview_focus={onTogglePreviewFocus}
                on_resize_start={onResizeStart}
                embedded={embedded}
              />
            </Suspense>
          ) : isPresentation ? (
            <Suspense
              fallback={(
                <PresentationPreviewFallback
                  agent_id={agentId}
                  path={path}
                  file_name={fileName}
                  is_preview_focused={isPreviewFocused}
                  on_toggle_preview_focus={onTogglePreviewFocus}
                  on_resize_start={onResizeStart}
                  embedded={embedded}
                />
              )}
            >
              <PresentationFilePreview
                agent_id={agentId}
                path={path}
                file_name={fileName}
                is_preview_focused={isPreviewFocused}
                on_toggle_preview_focus={onTogglePreviewFocus}
                on_resize_start={onResizeStart}
                embedded={embedded}
              />
            </Suspense>
          ) : isBinary ? (
            <BinaryFilePlaceholder
              agent_id={agentId}
              path={path}
              file_name={fileName}
              is_preview_focused={isPreviewFocused}
              on_toggle_preview_focus={onTogglePreviewFocus}
              embedded={embedded}
            />
          ) : (
            // 文本文件编辑器
            <>
              {!embedded ? (
                <ConversationResizeHandle
                  aria_label="调整编辑器宽度"
                  class_name="flex"
                  on_mouse_down={onResizeStart}
                />
              ) : null}

              <WorkspaceFilePreviewHeader
                actions={(
                  <>
                    <WorkspaceFileDownloadButton agent_id={agentId} file_name={fileName} path={path} />
                    <WorkspaceFilePreviewFocusButton
                      is_preview_focused={isPreviewFocused}
                      on_toggle_preview_focus={onTogglePreviewFocus}
                    />
                    <WorkspaceFileToolbarButton
                      on_click={() => {
                        if (isEditing) {
                          setIsEditing(false);
                          return;
                        }
                        enableEditing();
                      }}
                      title={isEditing ? "预览" : "编辑"}
                    >
                      {isEditing ? <Eye className="h-4 w-4" /> : <Pencil className="h-4 w-4" />}
                      <span className="max-xl:hidden">{isEditing ? "预览" : "编辑"}</span>
                    </WorkspaceFileToolbarButton>
                    <WorkspaceFileToolbarButton
                      disabled={!isDirty || isSaving || isExternalWriting}
                      on_click={() => void handleSave()}
                      title={isSaving ? "保存中" : "保存"}
                    >
                      <Save className="h-4 w-4" />
                      <span className="max-xl:hidden">{isSaving ? "保存中" : "保存"}</span>
                    </WorkspaceFileToolbarButton>
                  </>
                )}
                embedded={embedded}
                meta={(
                  <>
                    <span className="flex items-center gap-1">
                      <FileText className="h-3 w-3" />
                      {workspace_file_kind_label(fileType)}
                    </span>
                    {liveState && liveState.source !== "api" ? (
                      isExternalWriting ? (
                        <>
                          <LoaderCircle className="h-3 w-3 shrink-0 animate-spin text-primary" />
                          <span className="truncate">模型正在实时写入该文件</span>
                        </>
                      ) : (
                        <>
                          <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-(--success)" />
                          <span className="truncate">
                            已同步最新内容
                            {liveState.diff_stats
                              ? ` · +${liveState.diff_stats.additions} -${liveState.diff_stats.deletions}`
                              : ""}
                          </span>
                        </>
                      )
                    ) : null}
                  </>
                )}
                title={fileName}
              />

              {error ? (
                <div className="px-4 py-3 text-sm text-destructive">{error}</div>
              ) : null}

              <div
                ref={editorAreaRef}
                className={cn(
                  "min-h-0 flex-1 overflow-hidden",
                  fileType === "html" && !isEditing ? "p-0" : "px-4 py-4",
                )}
              >
                {isExternalWriting && fileType !== "html" ? (
                  <TypewriterFileView
                    content={draftContent}
                    container_width={editorWidth > 0 ? editorWidth - 40 : undefined}
                    class_name="h-full"
                  />
                ) : !isEditing && fileType === "html" ? (
                  <TextFilePreview
                    content={draftContent}
                    file_name={fileName}
                    file_type={fileType}
                    is_loading={isLoading}
                    is_streaming={isExternalWriting}
                  />
                ) : !isEditing ? (
                  <div className="soft-scrollbar h-full overflow-auto">
                    <TextFilePreview
                      content={draftContent}
                      file_name={fileName}
                      file_type={fileType}
                      is_loading={isLoading}
                    />
                  </div>
                ) : (
                  <textarea
                    aria-label="编辑文件内容"
                    ref={textareaRef}
                    className="soft-scrollbar h-full w-full resize-none border-0 bg-transparent p-0 font-mono text-sm leading-6 text-(--text-default) outline-none disabled:opacity-70"
                    disabled={isLoading}
                    onBlur={() => setIsEditing(false)}
                    onChange={(event) => setDraftContent(event.target.value)}
                    value={isLoading ? "加载中..." : draftContent}
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
