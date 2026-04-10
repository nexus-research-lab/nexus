"use client";

import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";

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

  const dialog = (
    <div
      className="dialog-backdrop z-[9999] animate-in fade-in duration-150"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-dialog-title"
      aria-describedby="confirm-dialog-message"
    >
      <div className="dialog-shell radius-shell-lg w-full max-w-md animate-in zoom-in-95 duration-150">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 id="confirm-dialog-title" className="dialog-title">
              {title}
            </h3>
          </div>
          <WorkspacePillButton
            aria-label="关闭"
            density="compact"
            onClick={on_cancel}
            size="icon"
            variant="icon"
          >
            <X className="h-4 w-4" />
          </WorkspacePillButton>
        </div>

        <div className="dialog-body">
          <p id="confirm-dialog-message" className="text-sm leading-6 text-muted-foreground">
            {message}
          </p>
        </div>

        <div className="dialog-footer">
          <WorkspacePillButton onClick={on_cancel} size="md" variant="tonal">
            {cancel_text}
          </WorkspacePillButton>
          <WorkspacePillButton
            ref={confirmButtonRef}
            onClick={on_confirm}
            size="md"
            tone={variant === "danger" ? "danger" : "default"}
            variant="primary"
          >
            {confirm_text}
          </WorkspacePillButton>
        </div>
      </div>
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

  const dialog = (
    <div
      className="dialog-backdrop z-[9999] animate-in fade-in duration-150"
      role="dialog"
      aria-modal="true"
      aria-labelledby="prompt-dialog-title"
    >
      <div className="dialog-shell radius-shell-lg w-full max-w-md animate-in zoom-in-95 duration-150">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 id="prompt-dialog-title" className="dialog-title">
              {title}
            </h3>
          </div>
          <WorkspacePillButton
            aria-label="关闭"
            density="compact"
            onClick={() => {
              setValue(default_value);
              on_cancel();
            }}
            size="icon"
            variant="icon"
          >
            <X className="h-4 w-4" />
          </WorkspacePillButton>
        </div>

        <div className="dialog-body">
          {message ? (
            <p className="pb-3 text-sm leading-6 text-muted-foreground">{message}</p>
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
          <WorkspacePillButton
            onClick={() => {
              setValue(default_value);
              on_cancel();
            }}
            size="md"
            variant="tonal"
          >
            取消
          </WorkspacePillButton>
          <WorkspacePillButton onClick={() => on_confirm(value)} size="md" variant="primary">
            确认
          </WorkspacePillButton>
        </div>
      </div>
    </div>
  );

  if (typeof document === "undefined") {
    return null;
  }

  return createPortal(dialog, document.body);
}
