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
  is_open,
  title,
  message,
  confirm_text = "确认",
  cancel_text = "取消",
  on_confirm,
  on_cancel,
  variant = "default",
}: ConfirmDialogProps) {
  const confirmButtonRef = useRef<HTMLButtonElement>(null);

  if (!is_open) return null;

  const is_danger = variant === "danger";

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9999]"
        described_by="confirm-dialog-message"
        initial_focus_ref={confirmButtonRef}
        labelled_by="confirm-dialog-title"
        on_close={on_cancel}
      >
        <UiDialogShell size="sm">
          <UiDialogHeader class_name="items-center">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <div
                className={DIALOG_HEADER_ICON_CLASS_NAME}
                style={
                  is_danger
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
                  {is_danger
                    ? "此操作会立即生效，且不可恢复。"
                    : "请确认是否继续执行该操作。"}
                </p>
              </div>
            </div>
            <UiDialogCloseButton on_close={on_cancel} />
          </UiDialogHeader>

          <UiDialogBody>
            <div
              className={get_dialog_note_class_name(
                is_danger ? "danger" : "default",
              )}
              id="confirm-dialog-message"
              style={get_dialog_note_style(is_danger ? "danger" : "default")}
            >
              {message}
            </div>
          </UiDialogBody>

          <UiDialogFooter>
            <button
              className={get_dialog_action_class_name("default")}
              onClick={on_cancel}
              type="button"
            >
              {cancel_text}
            </button>
            <button
              className={get_dialog_action_class_name(
                is_danger ? "danger" : "primary",
                "min-w-[110px]",
              )}
              ref={confirmButtonRef}
              onClick={on_confirm}
              type="button"
            >
              {confirm_text}
            </button>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

export function PromptDialog({
  is_open,
  title,
  message,
  placeholder = "",
  default_value = "",
  multiline = false,
  rows = 8,
  on_confirm,
  on_cancel,
}: PromptDialogProps) {
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement>(null);
  const [value, setValue] = useState(default_value);
  const initial_focus_ref = inputRef as unknown as RefObject<HTMLElement | null>;

  const cancel = () => {
    setValue(default_value);
    on_cancel();
  };

  const handle_input_key_down = (
    event: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>,
  ) => {
    if (!multiline && event.key === "Enter") {
      event.preventDefault();
      on_confirm(value);
      return;
    }

    if (multiline && (event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      on_confirm(value);
    }
  };

  // 当对话框打开时重置值
  useEffect(() => {
    if (is_open) {
      setValue(default_value);
    }
  }, [is_open, default_value]);

  useEffect(() => {
    if (is_open && inputRef.current) {
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
  }, [is_open, multiline]);

  if (!is_open) return null;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9999]"
        initial_focus_ref={initial_focus_ref}
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
                  ref={inputRef as RefObject<HTMLTextAreaElement>}
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  onKeyDown={handle_input_key_down}
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
                ref={inputRef as RefObject<HTMLInputElement>}
                type="text"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                onKeyDown={handle_input_key_down}
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
              onClick={() => on_confirm(value)}
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
