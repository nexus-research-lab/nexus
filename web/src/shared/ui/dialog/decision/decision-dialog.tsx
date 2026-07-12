"use client";

import {
  type FocusEvent,
  type KeyboardEvent,
  type RefObject,
  useId,
  useRef,
  useState,
} from "react";
import { AlertTriangle } from "lucide-react";

import {
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  getDialogNoteClassName,
  getDialogNoteStyle,
} from "@/shared/ui/dialog/dialog-styles";

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
  title: string;
  variant?: ConfirmDialogVariant;
}

interface PromptDialogProps {
  defaultValue?: string;
  isOpen: boolean;
  message?: string;
  multiline?: boolean;
  onCancel: () => void;
  onConfirm: (value: string) => void;
  placeholder?: string;
  rows?: number;
  title: string;
}

export function ConfirmDialog({
  cancelText = "取消",
  confirmText = "确认",
  isOpen,
  message,
  onCancel,
  onConfirm,
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
      <UiDialogHeader className="items-center">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <div
            className={DIALOG_HEADER_ICON_CLASS_NAME}
            style={presentation.iconStyle}
          >
            <AlertTriangle className="h-4.5 w-4.5" />
          </div>
          <div className="min-w-0 flex-1">
            <h3 className="dialog-title" id={titleId}>{title}</h3>
            <p className="mt-1 text-[12px] leading-5 text-(--text-soft)">
              {presentation.subtitle}
            </p>
          </div>
        </div>
        <UiDialogCloseButton onClose={onCancel} />
      </UiDialogHeader>
      <UiDialogBody>
        <div
          className={getDialogNoteClassName(presentation.noteTone)}
          id={messageId}
          style={getDialogNoteStyle(presentation.noteTone)}
        >
          {message}
        </div>
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
  defaultValue = "",
  isOpen,
  message,
  multiline = false,
  onCancel,
  onConfirm,
  placeholder = "",
  rows = 8,
  title,
}: PromptDialogProps) {
  if (!isOpen) {
    return null;
  }
  return (
    <PromptDialogContent
      defaultValue={defaultValue}
      key={defaultValue}
      message={message}
      multiline={multiline}
      onCancel={onCancel}
      onConfirm={onConfirm}
      placeholder={placeholder}
      rows={rows}
      title={title}
    />
  );
}

function PromptDialogContent({
  defaultValue,
  message,
  multiline,
  onCancel,
  onConfirm,
  placeholder,
  rows,
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
      <UiDialogHeader>
        <div className="min-w-0 flex-1">
          <h3 className="dialog-title" id={titleId}>{title}</h3>
        </div>
        <UiDialogCloseButton onClose={cancel} />
      </UiDialogHeader>
      <UiDialogBody>
        {message ? (
          <p className="pb-3 text-sm leading-6 text-muted-foreground">{message}</p>
        ) : null}
        <PromptInput
          inputRef={inputRef}
          mode={mode}
          onChange={setValue}
          onKeyDown={handleInputKeyDown}
          placeholder={placeholder}
          rows={rows}
          textareaRef={textareaRef}
          value={value}
        />
      </UiDialogBody>
      <DecisionDialogActions
        cancelText="取消"
        confirmText="确认"
        onCancel={cancel}
        onConfirm={submit}
      />
    </DecisionDialogFrame>
  );
}

interface PromptDialogContentProps {
  defaultValue: string;
  message?: string;
  multiline: boolean;
  onCancel: () => void;
  onConfirm: (value: string) => void;
  placeholder: string;
  rows: number;
  title: string;
}

interface PromptInputProps {
  inputRef: RefObject<HTMLInputElement | null>;
  mode: PromptInputMode;
  onChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
  placeholder: string;
  rows?: number;
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
        <p className="pt-2 text-xs text-(--text-soft)">
          按 <kbd className="rounded bg-black/5 px-1 py-0.5 text-[11px]">Cmd/Ctrl + Enter</kbd> 可直接保存。
        </p>
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
