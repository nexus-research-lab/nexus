import type { KeyboardEvent, RefObject } from "react";

import { cn } from "@/shared/ui/class-name";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

interface UserMessageEditorProps {
  canSubmit: boolean;
  compact: boolean;
  draftContent: string;
  onCancel: () => void;
  onChange: (content: string) => void;
  onSubmit: () => void;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export function UserMessageEditor({
  canSubmit,
  compact,
  draftContent,
  onCancel,
  onChange,
  onSubmit,
  textareaRef,
}: UserMessageEditorProps) {
  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      onCancel();
      return;
    }
    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      onSubmit();
    }
  };

  return (
    <div className="input-shell workbench-input-shell ml-auto flex w-full max-w-full flex-col overflow-hidden rounded-[10px]">
      <textarea
        aria-label="编辑消息内容"
        className={cn(
          "soft-scrollbar min-h-0 resize-none appearance-none border-0 bg-transparent px-3 text-left text-[14px] leading-6 text-(--text-strong)",
          compact ? "py-1.5" : "py-2",
          "outline-none shadow-none ring-0 transition-none placeholder:text-(--text-faint)",
          "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
        )}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={handleKeyDown}
        ref={textareaRef}
        rows={2}
        value={draftContent}
      />
      <div className="flex items-center justify-end gap-1.5 border-t border-(--divider-subtle-color) px-2 py-0.5">
        <button
          className={getUiButtonClassName({ size: "xs", variant: "surface" })}
          onClick={onCancel}
          type="button"
        >
          取消
        </button>
        <button
          className={getUiButtonClassName({ size: "xs", variant: "solid" })}
          disabled={!canSubmit}
          onClick={onSubmit}
          type="button"
        >
          发送
        </button>
      </div>
    </div>
  );
}
