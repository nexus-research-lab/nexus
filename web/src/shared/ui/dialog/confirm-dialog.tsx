"use client";

import { type KeyboardEvent, type RefObject, useEffect, useRef, useState } from "react";
import { AlertTriangle } from "lucide-react";

import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  get_dialog_action_class_name,
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";

interface ConfirmDialogProps {
  is_open: boolean;
  title: string;
  message: string;
  confirm_text?: string;
  cancel_text?: string;
  on_confirm: () => void;
  on_cancel: () => void;
  variant?: "danger" | "default";
}

interface PromptDialogProps {
  is_open: boolean;
  title: string;
  message?: string;
  placeholder?: string;
  default_value?: string;
  multiline?: boolean;
  rows?: number;
  on_confirm: (value: string) => void;
  on_cancel: () => void;
}

export function ConfirmDialog({
  is_open: isOpen,
  title,
  message,
  confirm_text: confirmText = "确认",
  cancel_text: cancelText = "取消",
  on_confirm: onConfirm,
  on_cancel: onCancel,
  variant = "default",
}: ConfirmDialogProps) {
  const confirmButtonRef = useRef<HTMLButtonElement>(null);

  if (!isOpen) return null;

  const isDanger = variant === "danger";

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9999]"
        described_by="confirm-dialog-message"
        initial_focus_ref={confirmButtonRef}
        labelled_by="confirm-dialog-title"
        on_close={onCancel}
      >
        <UiDialogShell size="sm">
          <UiDialogHeader class_name="items-center">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <div
                className={DIALOG_HEADER_ICON_CLASS_NAME}
                style={
                  isDanger
                    ? {
                        background:
                          "color-mix(in srgb, var(--destructive) 12%, var(--modal-dialog-body-background))",
                        border:
                          "1px solid color-mix(in srgb, var(--destructive) 22%, var(--modal-card-border))",
                        color: "var(--destructive)",
                      }
                    : undefined
                }
              >
                <AlertTriangle className="h-4.5 w-4.5" />
              </div>

              <div className="min-w-0 flex-1">
                <h3 id="confirm-dialog-title" className="dialog-title">
                  {title}
                </h3>
                <p className="mt-1 text-[12px] leading-5 text-(--text-soft)">
                  {isDanger
                    ? "此操作会立即生效，且不可恢复。"
                    : "请确认是否继续执行该操作。"}
                </p>
              </div>
            </div>
            <UiDialogCloseButton on_close={onCancel} />
          </UiDialogHeader>

          <UiDialogBody>
            <div
              className={get_dialog_note_class_name(
                isDanger ? "danger" : "default",
              )}
              id="confirm-dialog-message"
              style={get_dialog_note_style(isDanger ? "danger" : "default")}
            >
              {message}
            </div>
          </UiDialogBody>

          <UiDialogFooter>
            <button
              className={get_dialog_action_class_name("default")}
              onClick={onCancel}
              type="button"
            >
              {cancelText}
            </button>
            <button
              className={get_dialog_action_class_name(
                isDanger ? "danger" : "primary",
                "min-w-[110px]",
              )}
              ref={confirmButtonRef}
              onClick={onConfirm}
              type="button"
            >
              {confirmText}
            </button>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

export function PromptDialog({
  is_open: isOpen,
  title,
  message,
  placeholder = "",
  default_value: defaultValue = "",
  multiline = false,
  rows = 8,
  on_confirm: onConfirm,
  on_cancel: onCancel,
}: PromptDialogProps) {
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement>(null);
  const [value, setValue] = useState(defaultValue);
  const initialFocusRef = inputRef as unknown as RefObject<HTMLElement | null>;

  const cancel = () => {
    setValue(defaultValue);
    onCancel();
  };

  const handleInputKeyDown = (
    event: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>,
  ) => {
    if (!multiline && event.key === "Enter") {
      event.preventDefault();
      onConfirm(value);
      return;
    }

    if (multiline && (event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      onConfirm(value);
    }
  };

  // 当对话框打开时重置值
  useEffect(() => {
    if (isOpen) {
      setValue(defaultValue);
    }
  }, [isOpen, defaultValue]);

  useEffect(() => {
    if (isOpen && inputRef.current) {
      inputRef.current.focus();
      if (!multiline) {
        inputRef.current.select();
      } else {
        inputRef.current.setSelectionRange(
          inputRef.current.value.length,
          inputRef.current.value.length,
        );
      }
    }
  }, [isOpen, multiline]);

  if (!isOpen) return null;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9999]"
        initial_focus_ref={initialFocusRef}
        labelled_by="prompt-dialog-title"
        on_close={cancel}
      >
        <UiDialogShell size="sm">
          <UiDialogHeader>
            <div className="min-w-0 flex-1">
              <h3 id="prompt-dialog-title" className="dialog-title">
                {title}
              </h3>
            </div>
            <UiDialogCloseButton on_close={cancel} />
          </UiDialogHeader>

          <UiDialogBody>
            {message ? (
              <p className="pb-3 text-sm leading-6 text-muted-foreground">
                {message}
              </p>
            ) : null}

            {multiline ? (
              <>
                <textarea
                  aria-label={placeholder || "输入内容"}
                  ref={inputRef as RefObject<HTMLTextAreaElement>}
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  onKeyDown={handleInputKeyDown}
                  placeholder={placeholder}
                  rows={rows}
                  className="dialog-input surface-radius-sm min-h-[180px] w-full resize-y px-4 py-3 text-sm leading-6 text-foreground placeholder:text-muted-foreground/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
                />
                <p className="pt-2 text-xs text-(--text-soft)">
                  按 <kbd className="rounded bg-black/5 px-1 py-0.5 text-[11px]">Cmd/Ctrl + Enter</kbd> 可直接保存。
                </p>
              </>
            ) : (
              <input
                aria-label={placeholder || "输入内容"}
                ref={inputRef as RefObject<HTMLInputElement>}
                type="text"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                onKeyDown={handleInputKeyDown}
                placeholder={placeholder}
                className="dialog-input surface-radius-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
              />
            )}
          </UiDialogBody>

          <UiDialogFooter>
            <button
              className={get_dialog_action_class_name("default")}
              onClick={cancel}
              type="button"
            >
              取消
            </button>
            <button
              className={get_dialog_action_class_name("primary")}
              onClick={() => onConfirm(value)}
              type="button"
            >
              确认
            </button>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
