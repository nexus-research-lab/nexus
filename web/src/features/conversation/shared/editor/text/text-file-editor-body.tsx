// INPUT: exact Agent 与文件正文、模式、编辑命令和布局观察。
// OUTPUT: 保留文件归属的预览或编辑视图。
// POS: 文本编辑器正文装配；所有渲染模式透传同一 Agent scope。
import {
  useEffect,
  useRef,
  useState,
  type ComponentType,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { TypewriterFileView } from "@/shared/ui/feedback/typewriter-file-view";

import type { WorkspaceFilePreviewKind } from "../workspace-file-preview-kind";
import { TextFileContent } from "./text-file-content";
import type { TextEditorBodyMode } from "./text-file-editor-model";

interface TextEditorBodyViewProps {
  agentId: string;
  containerWidth: number;
  content: string;
  exitEditingOnBlur: boolean;
  fileName: string;
  fileType: WorkspaceFilePreviewKind;
  isLoading: boolean;
  isStreaming: boolean;
  setContent: Dispatch<SetStateAction<string>>;
  setIsEditing: (value: boolean) => void;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

interface TextFileEditorBodyProps extends Omit<
  TextEditorBodyViewProps,
  "containerWidth" | "exitEditingOnBlur" | "textareaRef"
> {
  exitEditingOnBlur?: boolean;
  mode: TextEditorBodyMode;
}

function StreamingBody({
  containerWidth,
  content,
}: TextEditorBodyViewProps) {
  return (
    <TypewriterFileView
      className="h-full min-h-0"
      containerWidth={containerWidth > 0 ? containerWidth - 40 : undefined}
      content={content}
    />
  );
}

function HtmlPreviewBody(props: TextEditorBodyViewProps) {
  return (
    <TextFileContent
      agentId={props.agentId}
      content={props.content}
      fileName={props.fileName}
      fileType={props.fileType}
      isLoading={props.isLoading}
      isStreaming={props.isStreaming}
    />
  );
}

function PreviewBody(props: TextEditorBodyViewProps) {
  return (
    <div className="soft-scrollbar h-full min-h-0 min-w-0 overscroll-contain overflow-auto">
      <TextFileContent
      agentId={props.agentId}
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
  exitEditingOnBlur,
  isLoading,
  setContent,
  setIsEditing,
  textareaRef,
}: TextEditorBodyViewProps) {
  const { t } = useI18n();
  return (
    <textarea
      aria-label={t("workspace_file.edit_content")}
      className="soft-scrollbar h-full min-h-0 w-full resize-none border-0 bg-transparent p-0 font-mono text-sm leading-6 text-(--text-default) shadow-none outline-none ring-0 focus:border-0 focus:bg-transparent focus:shadow-none focus:outline-none focus:ring-0 focus-visible:border-0 focus-visible:bg-transparent focus-visible:shadow-none focus-visible:outline-none focus-visible:ring-0 disabled:opacity-70"
      disabled={isLoading}
      onBlur={exitEditingOnBlur ? () => setIsEditing(false) : undefined}
      onChange={(event) => setContent(event.target.value)}
      ref={textareaRef}
      value={isLoading ? t("workspace_file.loading") : content}
    />
  );
}

const TEXT_EDITOR_BODIES: Record<
  TextEditorBodyMode,
  ComponentType<TextEditorBodyViewProps>
> = {
  editing: EditingBody,
  html: HtmlPreviewBody,
  preview: PreviewBody,
  streaming: StreamingBody,
};

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

export function TextFileEditorBody({
  exitEditingOnBlur = true,
  mode,
  ...props
}: TextFileEditorBodyProps) {
  const editorAreaRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const editorWidth = useElementWidth(editorAreaRef);
  const Body = TEXT_EDITOR_BODIES[mode];

  useEffect(() => {
    if (mode === "editing") {
      textareaRef.current?.focus();
    }
  }, [mode]);

  return (
    <div
      className={cn(
        "h-full min-h-0 min-w-0 flex-1 overflow-hidden",
        mode === "html" ? "p-0" : "px-4 py-4",
      )}
      ref={editorAreaRef}
    >
      <Body
        {...props}
        containerWidth={editorWidth}
        exitEditingOnBlur={exitEditingOnBlur}
        textareaRef={textareaRef}
      />
    </div>
  );
}
