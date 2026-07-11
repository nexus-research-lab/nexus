"use client";

import {
  useEffect,
  useRef,
  useState,
  type ComponentType,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";
import { Eye, FileText, LoaderCircle, Pencil, Save } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { TypewriterFileView } from "@/shared/ui/feedback/typewriter-file-view";

import { ConversationResizeHandle } from "../conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
  WorkspaceFileToolbarButton,
} from "../workspace-file-preview-chrome";
import {
  workspaceFileKindLabel,
  type WorkspaceFilePreviewKind,
} from "../workspace-file-preview-kind";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { TextFileContent } from "./text-file-content";
import { useTextFileEditor } from "./use-text-file-editor";

type TextEditorBodyMode = "editing" | "html" | "preview" | "streaming";

interface TextEditorBodyProps {
  containerWidth: number;
  content: string;
  fileName: string;
  fileType: WorkspaceFilePreviewKind;
  isLoading: boolean;
  isStreaming: boolean;
  setContent: Dispatch<SetStateAction<string>>;
  setIsEditing: (value: boolean) => void;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

function StreamingBody({
  containerWidth,
  content,
}: TextEditorBodyProps) {
  return (
    <TypewriterFileView
      className="h-full"
      containerWidth={containerWidth > 0 ? containerWidth - 40 : undefined}
      content={content}
    />
  );
}

function HtmlPreviewBody(props: TextEditorBodyProps) {
  return (
    <TextFileContent
      content={props.content}
      fileName={props.fileName}
      fileType={props.fileType}
      isLoading={props.isLoading}
      isStreaming={props.isStreaming}
    />
  );
}

function PreviewBody(props: TextEditorBodyProps) {
  return (
    <div className="soft-scrollbar h-full overflow-auto">
      <TextFileContent
        content={props.content}
        fileName={props.fileName}
        fileType={props.fileType}
        isLoading={props.isLoading}
        isStreaming={false}
      />
    </div>
  );
}

function EditingBody({
  content,
  isLoading,
  setContent,
  setIsEditing,
  textareaRef,
}: TextEditorBodyProps) {
  return (
    <textarea
      aria-label="编辑文件内容"
      className="soft-scrollbar h-full w-full resize-none border-0 bg-transparent p-0 font-mono text-sm leading-6 text-(--text-default) outline-none disabled:opacity-70"
      disabled={isLoading}
      onBlur={() => setIsEditing(false)}
      onChange={(event) => setContent(event.target.value)}
      ref={textareaRef}
      value={isLoading ? "加载中..." : content}
    />
  );
}

const TEXT_EDITOR_BODIES: Record<
  TextEditorBodyMode,
  ComponentType<TextEditorBodyProps>
> = {
  editing: EditingBody,
  html: HtmlPreviewBody,
  preview: PreviewBody,
  streaming: StreamingBody,
};

function resolveTextEditorBodyMode(
  fileType: WorkspaceFilePreviewKind,
  isEditing: boolean,
  isExternalWriting: boolean,
): TextEditorBodyMode {
  const rules: Array<[boolean, TextEditorBodyMode]> = [
    [isExternalWriting && fileType !== "html", "streaming"],
    [isEditing, "editing"],
    [fileType === "html", "html"],
  ];
  return rules.find(([matches]) => matches)?.[1] ?? "preview";
}

function useElementWidth(ref: RefObject<HTMLDivElement | null>): number {
  const [width, setWidth] = useState(0);
  useEffect(() => {
    const element = ref.current;
    if (!element) {
      return;
    }
    const observer = new ResizeObserver(([entry]) => {
      if (entry) {
        setWidth(entry.contentRect.width);
      }
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [ref]);
  return width;
}

export function TextFileEditor({
  agentId,
  embedded,
  fileName,
  fileType,
  isPreviewFocused,
  onResizeStart,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps & { fileType: WorkspaceFilePreviewKind }) {
  const editorAreaRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const editorWidth = useElementWidth(editorAreaRef);
  const editor = useTextFileEditor({ agentId, path });
  const bodyMode = resolveTextEditorBodyMode(
    fileType,
    editor.isEditing,
    editor.isExternalWriting,
  );
  const Body = TEXT_EDITOR_BODIES[bodyMode];

  useEffect(() => {
    if (editor.isEditing) {
      textareaRef.current?.focus();
    }
  }, [editor.isEditing]);

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          ariaLabel="调整编辑器宽度"
          className="flex"
          onMouseDown={onResizeStart}
        />
      ) : null}
      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton
              agentId={agentId}
              fileName={fileName}
              path={path}
            />
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
            />
            <WorkspaceFileToolbarButton
              onClick={editor.toggleEditing}
              title={editor.isEditing ? "预览" : "编辑"}
            >
              {editor.isEditing
                ? <Eye className="h-4 w-4" />
                : <Pencil className="h-4 w-4" />}
              <span className="max-xl:hidden">
                {editor.isEditing ? "预览" : "编辑"}
              </span>
            </WorkspaceFileToolbarButton>
            <WorkspaceFileToolbarButton
              disabled={
                !editor.isDirty ||
                editor.isSaving ||
                editor.isExternalWriting
              }
              onClick={() => void editor.save()}
              title={editor.isSaving ? "保存中" : "保存"}
            >
              <Save className="h-4 w-4" />
              <span className="max-xl:hidden">
                {editor.isSaving ? "保存中" : "保存"}
              </span>
            </WorkspaceFileToolbarButton>
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileText className="h-3 w-3" />
              {workspaceFileKindLabel(fileType)}
            </span>
            {editor.liveState?.source !== "api" ? (
              editor.isExternalWriting ? (
                <>
                  <LoaderCircle className="h-3 w-3 shrink-0 animate-spin text-primary" />
                  <span className="truncate">模型正在实时写入该文件</span>
                </>
              ) : (
                <>
                  <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-(--success)" />
                  <span className="truncate">
                    已同步最新内容
                    {editor.liveState?.diff_stats
                      ? ` · +${editor.liveState.diff_stats.additions} -${editor.liveState.diff_stats.deletions}`
                      : ""}
                  </span>
                </>
              )
            ) : null}
          </>
        )}
        title={fileName}
      />
      {editor.error ? (
        <div className="px-4 py-3 text-sm text-destructive">
          {editor.error}
        </div>
      ) : null}
      <div
        className={cn(
          "min-h-0 flex-1 overflow-hidden",
          bodyMode === "html" ? "p-0" : "px-4 py-4",
        )}
        ref={editorAreaRef}
      >
        <Body
          containerWidth={editorWidth}
          content={editor.draftContent}
          fileName={fileName}
          fileType={fileType}
          isLoading={editor.isLoading}
          isStreaming={editor.isExternalWriting}
          setContent={editor.setDraftContent}
          setIsEditing={editor.setIsEditing}
          textareaRef={textareaRef}
        />
      </div>
    </>
  );
}
