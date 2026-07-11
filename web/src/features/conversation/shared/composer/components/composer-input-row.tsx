import type {
  ClipboardEventHandler,
  KeyboardEventHandler,
  RefObject,
  WheelEvent,
} from "react";

import { cn } from "@/shared/ui/class-name";
import type { MentionTargetItem } from "@/shared/ui/mention/mention-target-model";
import { MentionTargetPopover } from "@/shared/ui/mention/mention-target-popover";

import { COMPOSER_SHORTCUT_KEY_CLASS_NAME } from "../composer-model";
import {
  ComposerSubmitButton,
  type ComposerSubmitButtonProps,
} from "./composer-submit-button";

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
    enterLabel: string;
    newLineLabel: string;
    paddingClassName: string;
    showShortcuts: boolean;
  };
  mention: {
    active: boolean;
    filter: string;
    items: MentionTargetItem[];
    onClose: () => void;
    onSelect: (item: MentionTargetItem) => void;
  };
  submit: ComposerSubmitButtonProps;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export function ComposerInputRow({
  input,
  layout,
  mention,
  submit,
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
            "multiline-cursor soft-scrollbar min-h-6 w-full min-w-0 max-h-[200px] resize-none overflow-y-auto overscroll-contain bg-transparent text-[14px] leading-6 text-(--text-strong) outline-none shadow-none ring-0",
            "placeholder:text-(--text-soft)",
            "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
            "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
            layout.showShortcuts && "min-[760px]:pr-[210px]",
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
        {layout.showShortcuts ? (
          <ComposerInlineShortcuts
            enterLabel={layout.enterLabel}
            newLineLabel={layout.newLineLabel}
          />
        ) : null}
      </div>
      <ComposerSubmitButton {...submit} />
    </div>
  );
}

function stopNestedTextareaWheel(event: WheelEvent<HTMLTextAreaElement>) {
  const target = event.currentTarget;
  if (target.scrollHeight > target.clientHeight) {
    event.stopPropagation();
  }
}

function ComposerInlineShortcuts({
  enterLabel,
  newLineLabel,
}: {
  enterLabel: string;
  newLineLabel: string;
}) {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none absolute right-0 top-1/2 hidden -translate-y-1/2 items-center gap-1.5 text-[11px] leading-none text-(--text-soft) min-[760px]:flex"
    >
      <span className="inline-flex items-center gap-1">
        <kbd className={COMPOSER_SHORTCUT_KEY_CLASS_NAME}>Enter</kbd>
        <span>{enterLabel}</span>
      </span>
      <span className="text-(--text-faint)">·</span>
      <span className="inline-flex items-center gap-1">
        <kbd className={COMPOSER_SHORTCUT_KEY_CLASS_NAME}>Shift</kbd>
        <span className="text-(--text-faint)">+</span>
        <kbd className={COMPOSER_SHORTCUT_KEY_CLASS_NAME}>Enter</kbd>
        <span>{newLineLabel}</span>
      </span>
    </div>
  );
}
