"use client";

import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { AlertTriangle, X } from "lucide-react";

import {
  DIALOG_BACKDROP_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
  get_dialog_action_class_name,
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";

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
  on_confirm: (value: string) => void;
  on_cancel: () => void;
}

interface TextareaDialogProps {
  is_open: boolean;
  title: string;
  message?: string;
  placeholder?: string;
  default_value?: string;
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

  useEffect(() => {
    if (is_open && confirmButtonRef.current) {
      confirmButtonRef.current.focus();
    }
  }, [is_open]);

  useEffect(() => {
    const handle_key_down = (e: KeyboardEvent) => {
      if (!is_open) return;
      if (e.key === "Escape") {
        e.preventDefault();
        on_cancel();
      }
    };
    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_open, on_cancel]);

  if (!is_open) return null;

  const is_danger = variant === "danger";

  const dialog = (
    <div
      className={`${DIALOG_BACKDROP_CLASS_NAME} z-[9999]`}
      data-modal-root="true"
      onClick={on_cancel}
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-dialog-title"
      aria-describedby="confirm-dialog-message"
    >
      <section
        className="dialog-shell radius-shell-xl flex w-full max-w-md flex-col overflow-hidden animate-in zoom-in-95 duration-(--motion-duration-fast)"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="dialog-header">
          <div className="flex min-w-0 flex-1 items-start gap-3.5">
            <div
              className={DIALOG_HEADER_ICON_CLASS_NAME}
              style={
                is_danger
                  ? {
                      background:
                        "color-mix(in srgb, var(--destructive) 12%, var(--modal-card-background))",
                      border:
                        "1px solid color-mix(in srgb, var(--destructive) 22%, var(--modal-card-border))",
                      color: "var(--destructive)",
                    }
                  : undefined
              }
            >
              <AlertTriangle className="h-4.5 w-4.5" />
            </div>

            <div className="min-w-0 flex-1 pt-0.5">
              <h3 id="confirm-dialog-title" className="dialog-title">
                {title}
              </h3>
              <p className="mt-1 text-[12px] text-(--text-soft)">
                {is_danger
                  ? "此操作会立即生效，且不可恢复。"
                  : "请确认是否继续执行该操作。"}
              </p>
            </div>
          </div>
          <button
            aria-label="关闭"
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            onClick={on_cancel}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="dialog-body">
          <div
            className={get_dialog_note_class_name(
              is_danger ? "danger" : "default",
            )}
            id="confirm-dialog-message"
            style={get_dialog_note_style(is_danger ? "danger" : "default")}
          >
            {message}
          </div>
        </div>

        <div className="dialog-footer border-t border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--modal-card-background)_84%,transparent)]">
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
        </div>
      </section>
    </div>
  );

  if (typeof document === "undefined") {
    return null;
  }

  return createPortal(dialog, document.body);
}

export function PromptDialog({
  is_open,
  title,
  message,
  placeholder = "",
  default_value = "",
  on_confirm,
  on_cancel,
}: PromptDialogProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [value, setValue] = useState(default_value);

  // 当对话框打开时重置值
  useEffect(() => {
    if (is_open) {
      setValue(default_value);
    }
  }, [is_open, default_value]);

  useEffect(() => {
    if (is_open && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [is_open]);

  useEffect(() => {
    const handle_key_down = (e: KeyboardEvent) => {
      if (!is_open) return;
      if (e.key === "Escape") {
        e.preventDefault();
        on_cancel();
        setValue(default_value);
      }
      if (e.key === "Enter") {
        e.preventDefault();
        on_confirm(value);
      }
    };
    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_open, on_cancel, on_confirm, value, default_value]);

  if (!is_open) return null;

  const dialog = (
    <div
      className="dialog-backdrop z-[9999] animate-in fade-in duration-(--motion-duration-fast)"
      data-modal-root="true"
      role="dialog"
      aria-modal="true"
      aria-labelledby="prompt-dialog-title"
    >
      <section className="dialog-shell radius-shell-lg flex w-full max-w-md flex-col overflow-hidden animate-in zoom-in-95 duration-(--motion-duration-fast)">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 id="prompt-dialog-title" className="dialog-title">
              {title}
            </h3>
          </div>
          <button
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            aria-label="关闭"
            onClick={() => {
              setValue(default_value);
              on_cancel();
            }}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="dialog-body">
          {message ? (
            <p className="pb-3 text-sm leading-6 text-muted-foreground">
              {message}
            </p>
          ) : null}

          <input
            ref={inputRef}
            type="text"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder={placeholder}
            className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
          />
        </div>

        <div className="dialog-footer">
          <button
            className={get_dialog_action_class_name("default")}
            onClick={() => {
              setValue(default_value);
              on_cancel();
            }}
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
        </div>
      </section>
    </div>
  );

  if (typeof document === "undefined") {
    return null;
  }

  return createPortal(dialog, document.body);
}

export function TextareaDialog({
  is_open,
  title,
  message,
  placeholder = "",
  default_value = "",
  rows = 8,
  on_confirm,
  on_cancel,
}: TextareaDialogProps) {
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const [value, setValue] = useState(default_value);

  useEffect(() => {
    if (is_open) {
      setValue(default_value);
    }
  }, [default_value, is_open]);

  useEffect(() => {
    if (is_open && textarea_ref.current) {
      textarea_ref.current.focus();
      textarea_ref.current.setSelectionRange(
        textarea_ref.current.value.length,
        textarea_ref.current.value.length,
      );
    }
  }, [is_open]);

  useEffect(() => {
    const handle_key_down = (event: KeyboardEvent) => {
      if (!is_open) {
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        on_cancel();
        setValue(default_value);
      }
      if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
        event.preventDefault();
        on_confirm(value);
      }
    };

    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [default_value, is_open, on_cancel, on_confirm, value]);

  if (!is_open) {
    return null;
  }

  const dialog = (
    <div
      className="dialog-backdrop z-[9999] animate-in fade-in duration-(--motion-duration-fast)"
      data-modal-root="true"
      role="dialog"
      aria-modal="true"
      aria-labelledby="textarea-dialog-title"
    >
      <section className="dialog-shell radius-shell-lg flex w-full max-w-2xl flex-col overflow-hidden animate-in zoom-in-95 duration-(--motion-duration-fast)">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 id="textarea-dialog-title" className="dialog-title">
              {title}
            </h3>
          </div>
          <button
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            aria-label="关闭"
            onClick={() => {
              setValue(default_value);
              on_cancel();
            }}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="dialog-body">
          {message ? (
            <p className="pb-3 text-sm leading-6 text-muted-foreground">
              {message}
            </p>
          ) : null}

          <textarea
            ref={textarea_ref}
            value={value}
            onChange={(event) => setValue(event.target.value)}
            placeholder={placeholder}
            rows={rows}
            className="dialog-input radius-shell-sm min-h-[180px] w-full resize-y px-4 py-3 text-sm leading-6 text-foreground placeholder:text-muted-foreground/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
          />
          <p className="pt-2 text-xs text-(--text-soft)">
            按{" "}
            <kbd className="rounded bg-black/5 px-1 py-0.5 text-[11px]">
              Cmd/Ctrl + Enter
            </kbd>{" "}
            可直接保存。
          </p>
        </div>

        <div className="dialog-footer">
          <button
            className={get_dialog_action_class_name("default")}
            onClick={() => {
              setValue(default_value);
              on_cancel();
            }}
            type="button"
          >
            取消
          </button>
          <button
            className={get_dialog_action_class_name("primary")}
            onClick={() => on_confirm(value)}
            type="button"
          >
            保存
          </button>
        </div>
      </section>
    </div>
  );

  if (typeof document === "undefined") {
    return null;
  }

  return createPortal(dialog, document.body);
}
