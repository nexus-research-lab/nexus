import {
  useEffect,
  useRef,
  useState,
  type ComponentType,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";

import { cn } from "@/shared/ui/class-name";
import { TypewriterFileView } from "@/shared/ui/feedback/typewriter-file-view";

import type { WorkspaceFilePreviewKind } from "../workspace-file-preview-kind";
import { TextFileContent } from "./text-file-content";
import type { TextEditorBodyMode } from "./text-file-editor-model";

interface TextEditorBodyViewProps {
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

interface TextFileEditorBodyProps extends Omit<
  TextEditorBodyViewProps,
  "containerWidth" | "textareaRef"
> {
  mode: TextEditorBodyMode;
}

function StreamingBody({
  containerWidth,
  content,
}: TextEditorBodyViewProps) {
  return (
    <TypewriterFileView
      className="h-full"
      containerWidth={containerWidth > 0 ? containerWidth - 40 : undefined}
      content={content}
    />
  );
}

function HtmlPreviewBody(props: TextEditorBodyViewProps) {
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

function PreviewBody(props: TextEditorBodyViewProps) {
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
}: TextEditorBodyViewProps) {
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
        "min-h-0 flex-1 overflow-hidden",
        mode === "html" ? "p-0" : "px-4 py-4",
      )}
      ref={editorAreaRef}
    >
      <Body
        {...props}
        containerWidth={editorWidth}
        textareaRef={textareaRef}
      />
    </div>
  );
}
