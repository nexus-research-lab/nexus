/**
 * INPUT: 业务提供的明确标题、后果文案、输入值与确认/取消动作。
 * OUTPUT: 无默认警告套话、无装饰图标和消息卡片的紧凑决策弹窗。
 * POS: 全站轻量确认框与输入框；业务风险只能由调用方具体说明。
 */
"use client";

import {
  type FocusEvent,
  type KeyboardEvent,
  type RefObject,
  useId,
  useRef,
  useState,
} from "react";
import {
  UiDialogBody,
  UiDialogCloseButton,
} from "@/shared/ui/dialog/dialog";

import {
  DecisionDialogActions,
  DecisionDialogFrame,
} from "./decision-dialog-frame";
import {
  type ConfirmDialogVariant,
  getConfirmDialogPresentation,
  type PromptInputMode,
  resolvePromptKeyboardAction,
} from "./decision-dialog-model";

interface ConfirmDialogProps {
  cancelText?: string;
  confirmText?: string;
  isOpen: boolean;
  message: string;
  onCancel: () => void;
  onConfirm: () => void;
  subtitle?: string;
  title: string;
  variant?: ConfirmDialogVariant;
}

interface PromptDialogProps {
  cancelText?: string;
  confirmText?: string;
  defaultValue?: string;
  isOpen: boolean;
  message?: string;
  multiline?: boolean;
  onCancel: () => void;
  onConfirm: (value: string) => void;
  placeholder?: string;
  rows?: number;
  shortcutHint?: string;
  title: string;
}

export function ConfirmDialog({
  cancelText = "取消",
  confirmText = "确认",
  isOpen,
  message,
  onCancel,
  onConfirm,
  subtitle,
  title,
  variant = "default",
}: ConfirmDialogProps) {
  const confirmButtonRef = useRef<HTMLButtonElement>(null);
  const messageId = useId();
  const titleId = useId();
  if (!isOpen) {
    return null;
  }
  const presentation = getConfirmDialogPresentation(variant);
  return (
    <DecisionDialogFrame
      describedBy={messageId}
      initialFocusRef={confirmButtonRef}
      labelledBy={titleId}
      onClose={onCancel}
    >
      <UiDialogCloseButton
        className="absolute right-3 top-3 z-10"
        onClose={onCancel}
      />
      <UiDialogBody className="px-5 pb-4 pt-5 pr-14">
        <h3 className="dialog-title" id={titleId}>{title}</h3>
        {subtitle ? (
          <p className="mt-1.5 text-compact leading-5 text-(--text-soft)">
            {subtitle}
          </p>
        ) : null}
        <p
          className="mt-3 whitespace-pre-wrap text-sm leading-6 text-(--text-default)"
          id={messageId}
        >
          {message}
        </p>
      </UiDialogBody>
      <DecisionDialogActions
        cancelText={cancelText}
        confirmButtonRef={confirmButtonRef}
        confirmClassName="min-w-[110px]"
        confirmText={confirmText}
        confirmTone={presentation.actionTone}
        onCancel={onCancel}
        onConfirm={onConfirm}
      />
    </DecisionDialogFrame>
  );
}

export function PromptDialog({
  cancelText = "取消",
  confirmText = "确认",
  defaultValue = "",
  isOpen,
  message,
  multiline = false,
  onCancel,
  onConfirm,
  placeholder = "",
  rows = 8,
  shortcutHint,
  title,
}: PromptDialogProps) {
  if (!isOpen) {
    return null;
  }
  return (
    <PromptDialogContent
      cancelText={cancelText}
      confirmText={confirmText}
      defaultValue={defaultValue}
      key={defaultValue}
      message={message}
      multiline={multiline}
      onCancel={onCancel}
      onConfirm={onConfirm}
      placeholder={placeholder}
      rows={rows}
      shortcutHint={shortcutHint}
      title={title}
    />
  );
}

function PromptDialogContent({
  cancelText,
  confirmText,
  defaultValue,
  message,
  multiline,
  onCancel,
  onConfirm,
  placeholder,
  rows,
  shortcutHint,
  title,
}: PromptDialogContentProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [value, setValue] = useState(defaultValue);
  const titleId = useId();
  const mode: PromptInputMode = multiline ? "multiline" : "single";
  const initialFocusRef: RefObject<HTMLElement | null> = multiline
    ? textareaRef
    : inputRef;

  const cancel = () => {
    setValue(defaultValue);
    onCancel();
  };
  const submit = () => onConfirm(value);
  const handleInputKeyDown = (
    event: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>,
  ) => {
    const action = resolvePromptKeyboardAction({
      ctrlKey: event.ctrlKey,
      key: event.key,
      metaKey: event.metaKey,
      mode,
    });
    if (action === "ignore") {
      return;
    }
    event.preventDefault();
    submit();
  };

  return (
    <DecisionDialogFrame
      initialFocusRef={initialFocusRef}
      labelledBy={titleId}
      onClose={cancel}
    >
      <UiDialogCloseButton
        className="absolute right-3 top-3 z-10"
        onClose={cancel}
      />
      <UiDialogBody className="px-5 pb-4 pt-5 pr-14">
        <h3 className="dialog-title" id={titleId}>{title}</h3>
        {message ? (
          <p className="pb-3 pt-2 text-sm leading-6 text-(--text-muted)">{message}</p>
        ) : null}
        <PromptInput
          inputRef={inputRef}
          mode={mode}
          onChange={setValue}
          onKeyDown={handleInputKeyDown}
          placeholder={placeholder}
          rows={rows}
          shortcutHint={shortcutHint}
          textareaRef={textareaRef}
          value={value}
        />
      </UiDialogBody>
      <DecisionDialogActions
        cancelText={cancelText}
        confirmText={confirmText}
        onCancel={cancel}
        onConfirm={submit}
      />
    </DecisionDialogFrame>
  );
}

interface PromptDialogContentProps {
  cancelText: string;
  confirmText: string;
  defaultValue: string;
  message?: string;
  multiline: boolean;
  onCancel: () => void;
  onConfirm: (value: string) => void;
  placeholder: string;
  rows: number;
  shortcutHint?: string;
  title: string;
}

interface PromptInputProps {
  inputRef: RefObject<HTMLInputElement | null>;
  mode: PromptInputMode;
  onChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
  placeholder: string;
  rows?: number;
  shortcutHint?: string;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  value: string;
}

function PromptInput({
  inputRef,
  mode,
  onChange,
  onKeyDown,
  placeholder,
  rows,
  shortcutHint,
  textareaRef,
  value,
}: PromptInputProps) {
  if (mode === "multiline") {
    return (
      <>
        <textarea
          aria-label={placeholder || "输入内容"}
          className="dialog-input surface-radius-sm min-h-[180px] w-full resize-y px-4 py-3 text-sm leading-6 text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
          onChange={(event) => onChange(event.target.value)}
          onFocus={movePromptTextareaCursorToEnd}
          onKeyDown={onKeyDown}
          placeholder={placeholder}
          ref={textareaRef}
          rows={rows}
          value={value}
        />
        {shortcutHint ? (
          <p className="pt-2 text-xs text-(--text-soft)">{shortcutHint}</p>
        ) : (
          <p className="pt-2 text-xs text-(--text-soft)">
            按 <kbd className="rounded bg-black/5 px-1 py-0.5 text-xs">Cmd/Ctrl + Enter</kbd> 可直接保存。
          </p>
        )}
      </>
    );
  }
  return (
    <input
      aria-label={placeholder || "输入内容"}
      className="dialog-input surface-radius-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
      onChange={(event) => onChange(event.target.value)}
      onFocus={selectPromptInput}
      onKeyDown={onKeyDown}
      placeholder={placeholder}
      ref={inputRef}
      type="text"
      value={value}
    />
  );
}

function selectPromptInput(event: FocusEvent<HTMLInputElement>): void {
  event.currentTarget.select();
}

function movePromptTextareaCursorToEnd(
  event: FocusEvent<HTMLTextAreaElement>,
): void {
  const end = event.currentTarget.value.length;
  event.currentTarget.setSelectionRange(end, end);
}
