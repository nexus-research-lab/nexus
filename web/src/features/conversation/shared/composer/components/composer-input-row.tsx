import type {
  ClipboardEventHandler,
  KeyboardEventHandler,
  RefObject,
  WheelEvent,
} from "react";

import { cn } from "@/shared/ui/class-name";
import type { MentionTargetItem } from "@/shared/ui/mention/mention-target-model";
import { MentionTargetPopover } from "@/shared/ui/mention/mention-target-popover";

interface ComposerInputRowProps {
  input: {
    disabled: boolean;
    onChange: (value: string) => void;
    onCompositionEnd: (timeStamp: number) => void;
    onCompositionStart: () => void;
    onKeyDown: KeyboardEventHandler<HTMLTextAreaElement>;
    onPaste: ClipboardEventHandler<HTMLTextAreaElement>;
    placeholder: string;
    value: string;
  };
  layout: {
    paddingClassName: string;
  };
  mention: {
    active: boolean;
    filter: string;
    items: MentionTargetItem[];
    onClose: () => void;
    onSelect: (item: MentionTargetItem) => void;
  };
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

/** 中文注释：输入行只保留文字区；附件、模式与发送等动作统一收在底部工具行。 */
export function ComposerInputRow({
  input,
  layout,
  mention,
  textareaRef,
}: ComposerInputRowProps) {
  return (
    <div className={cn("flex items-end gap-2", layout.paddingClassName)}>
      {mention.active && mention.items.length > 0 ? (
        <MentionTargetPopover
          anchorRect={textareaRef.current?.getBoundingClientRect() ?? null}
          filter={mention.filter}
          items={mention.items}
          onClose={mention.onClose}
          onSelect={mention.onSelect}
          placement="above"
        />
      ) : null}
      <div className="relative min-w-0 flex-1">
        <textarea
          ref={textareaRef}
          aria-label={input.placeholder}
          className={cn(
            "multiline-cursor soft-scrollbar block min-h-8 w-full min-w-0 max-h-[200px] resize-none overflow-y-auto overscroll-contain bg-transparent px-1.5 py-1 text-base leading-6 text-(--text-strong) outline-none shadow-none ring-0",
            "placeholder:text-(--text-soft)",
            "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
            "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
          )}
          disabled={input.disabled}
          onChange={(event) => input.onChange(event.target.value)}
          onCompositionEnd={(event) => input.onCompositionEnd(event.timeStamp)}
          onCompositionStart={input.onCompositionStart}
          onKeyDown={input.onKeyDown}
          onPaste={input.onPaste}
          onWheel={stopNestedTextareaWheel}
          placeholder={input.placeholder}
          rows={1}
          value={input.value}
        />
      </div>
    </div>
  );
}

function stopNestedTextareaWheel(event: WheelEvent<HTMLTextAreaElement>) {
  const target = event.currentTarget;
  if (target.scrollHeight > target.clientHeight) {
    event.stopPropagation();
  }
}
