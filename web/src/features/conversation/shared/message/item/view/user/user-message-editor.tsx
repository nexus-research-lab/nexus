// INPUT: 用户消息编辑草稿、提交资格、键盘动作与 textarea 引用。
// OUTPUT: 可取消或提交的原位消息编辑器。
// POS: User message 编辑视图；不拥有按钮和输入框的跨页面视觉合同。

import type { KeyboardEvent, RefObject } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";

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
  const { t } = useI18n();
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
    <div className="input-shell ml-auto flex w-full max-w-full flex-col overflow-hidden">
      <textarea
        aria-label={t("message.edit_content")}
        className={cn(
          "soft-scrollbar min-h-0 resize-none appearance-none border-0 bg-transparent px-3 text-left text-base leading-6 text-(--text-strong)",
          compact ? "py-1.5" : "py-2",
          "outline-none shadow-none ring-0 transition-none placeholder:text-(--text-soft)",
          "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
        )}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={handleKeyDown}
        ref={textareaRef}
        rows={2}
        value={draftContent}
      />
      <div className="flex items-center justify-end gap-1.5 border-t border-(--divider-subtle-color) px-2 py-0.5">
        <UiButton
          onClick={onCancel}
          size="xs"
          variant="surface"
        >
          {t("common.cancel")}
        </UiButton>
        <UiButton
          disabled={!canSubmit}
          onClick={onSubmit}
          size="xs"
          variant="solid"
        >
          {t("composer.enter_send")}
        </UiButton>
      </div>
    </div>
  );
}
