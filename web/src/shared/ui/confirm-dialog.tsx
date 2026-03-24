"use client";

import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";

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
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!is_open) return;
      if (e.key === "Escape") {
        e.preventDefault();
        on_cancel();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [is_open, on_cancel]);

  if (!is_open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-150"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-dialog-title"
      aria-describedby="confirm-dialog-message"
    >
      <div className="soft-ring radius-shell-lg panel-surface w-full max-w-md p-5 animate-in zoom-in-95 duration-150">
        <div className="flex items-start justify-between gap-3 pb-3">
          <h3 id="confirm-dialog-title" className="text-base font-semibold text-foreground">
            {title}
          </h3>
          <button
            aria-label="关闭"
            className="neo-pill radius-shell-sm p-1 text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={on_cancel}
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <p id="confirm-dialog-message" className="text-sm text-muted-foreground">
          {message}
        </p>

        <div className="mt-4 flex justify-end gap-2">
          <button
            className="neo-pill radius-shell-sm px-4 py-2 text-sm font-medium text-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={on_cancel}
          >
            {cancel_text}
          </button>
          <button
            ref={confirmButtonRef}
            className={`radius-shell-sm px-4 py-2 text-sm font-medium text-primary-foreground transition-colors focus-visible:ring-2 focus-visible:ring-primary/50 ${
              variant === "danger"
                ? "bg-destructive shadow-[0_16px_28px_rgba(235,90,81,0.22)] hover:bg-destructive/90"
                : "bg-primary shadow-[0_16px_28px_rgba(133,119,255,0.22)] hover:bg-primary/90"
            }`}
            onClick={on_confirm}
          >
            {confirm_text}
          </button>
        </div>
      </div>
    </div>
  );
}

// 简单的 prompt 对话框
interface PromptDialogProps {
  is_open: boolean;
  title: string;
  message?: string;
  placeholder?: string;
  default_value?: string;
  on_confirm: (value: string) => void;
  on_cancel: () => void;
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
    const handleKeyDown = (e: KeyboardEvent) => {
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
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [is_open, on_cancel, on_confirm, value, default_value]);

  if (!is_open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-150"
      role="dialog"
      aria-modal="true"
      aria-labelledby="prompt-dialog-title"
    >
      <div className="soft-ring radius-shell-lg panel-surface w-full max-w-md p-5 animate-in zoom-in-95 duration-150">
        <div className="flex items-start justify-between gap-3 pb-3">
          <h3 id="prompt-dialog-title" className="text-base font-semibold text-foreground">
            {title}
          </h3>
          <button
            aria-label="关闭"
            className="neo-pill radius-shell-sm p-1 text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={() => {
              setValue(default_value);
              on_cancel();
            }}
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {message && (
          <p className="pb-3 text-sm text-muted-foreground">{message}</p>
        )}

        <input
          ref={inputRef}
          type="text"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={placeholder}
          className="neo-inset radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
        />

        <div className="mt-4 flex justify-end gap-2">
          <button
            className="neo-pill radius-shell-sm px-4 py-2 text-sm font-medium text-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={() => {
              setValue(default_value);
              on_cancel();
            }}
          >
            取消
          </button>
          <button
            className="radius-shell-sm bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-[0_16px_28px_rgba(133,119,255,0.22)] transition-colors hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={() => on_confirm(value)}
          >
            确认
          </button>
        </div>
      </div>
    </div>
  );
}
