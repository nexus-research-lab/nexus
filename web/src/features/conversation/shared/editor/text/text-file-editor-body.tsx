// INPUT: exact Agent 与文件正文、模式和编辑命令。
// OUTPUT: 保留文件归属的具名可键盘滚动预览或公共源码编辑，透传可选字段身份。
// POS: 文本编辑器正文装配；所有渲染模式透传同一 Agent scope。
import {
  useEffect,
  useRef,
  type ComponentType,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { TypewriterFileView } from "@/shared/ui/feedback/typewriter-file-view";
import { UiSourceEditor } from "@/shared/ui/form/source-editor";
import { UI_SOURCE_PREVIEW_SCROLL_CLASS_NAME } from "@/shared/ui/form/source-text-styles";

import type { WorkspaceFilePreviewKind } from "../workspace-file-preview-kind";
import { TextFileContent } from "./text-file-content";
import type { TextEditorBodyMode } from "./text-file-editor-model";

interface TextEditorBodyViewProps {
  agentId: string;
  content: string;
  exitEditingOnBlur: boolean;
  editorId?: string;
  editorLabel?: string;
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
  "exitEditingOnBlur" | "textareaRef"
> {
  exitEditingOnBlur?: boolean;
  mode: TextEditorBodyMode;
}

function StreamingBody({
  content,
}: TextEditorBodyViewProps) {
  return (
    <TypewriterFileView
      className="h-full min-h-0"
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
    // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- This named read-only scroll region needs a Tab stop for native keyboard scrolling.
    <div aria-label={props.fileName} className={cn("h-full", UI_SOURCE_PREVIEW_SCROLL_CLASS_NAME)} role="region" tabIndex={0}>
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
  editorId,
  editorLabel,
  exitEditingOnBlur,
  isLoading,
  setContent,
  setIsEditing,
  textareaRef,
}: TextEditorBodyViewProps) {
  const { t } = useI18n();
  return (
    <UiSourceEditor
      aria-label={editorLabel ?? t("workspace_file.edit_content")}
      className="h-full"
      disabled={isLoading}
      id={editorId}
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

export function TextFileEditorBody({
  exitEditingOnBlur = true,
  mode,
  ...props
}: TextFileEditorBodyProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
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
    >
      <Body
        {...props}
        exitEditingOnBlur={exitEditingOnBlur}
        textareaRef={textareaRef}
      />
    </div>
  );
}
