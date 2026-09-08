/**
 * INPUT: 业务标题、后果文案、初值、字段错误、执行状态与确认/取消动作。
 * OUTPUT: 双语紧凑决策弹窗、输入法安全的键盘提交、具名字段及执行中关闭锁。
 * POS: 全站轻量确认框与输入框；业务风险和失败事实只能由调用方具体说明。
 */
"use client";

import { CircleAlert } from "lucide-react";
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
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";
import { cn } from "@/shared/ui/class-name";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";

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
  busy?: boolean;
  cancelText?: string;
  confirmText?: string;
  failure?: {
    impact: string;
    nextStep: string;
    title: string;
    tone?: "danger" | "warning";
    urgency?: "assertive" | "polite";
  };
  isOpen: boolean;
  message: string;
  onCancel: () => void;
  onConfirm: () => void;
  subtitle?: string;
  title: string;
  variant?: ConfirmDialogVariant;
}

interface PromptDialogProps {
  busy?: boolean;
  cancelText?: string;
  confirmText?: string;
  defaultValue?: string;
  error?: string;
  inputLabel?: string;
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
  busy = false,
  cancelText,
  confirmText,
  failure,
  isOpen,
  message,
  onCancel,
  onConfirm,
  subtitle,
  title,
  variant = "default",
}: ConfirmDialogProps) {
  const { t } = useI18n();
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
      onClose={busy ? ignoreDialogClose : onCancel}
    >
      <UiDialogHeader
        actions={(
          <UiDialogCloseButton
            disabled={busy}
            onClose={onCancel}
          />
        )}
        appearance="plain"
        subtitle={subtitle}
        title={title}
        titleId={titleId}
      />
      <UiDialogBody className="space-y-3 px-5 pb-4 pt-2">
        <p
          className="whitespace-pre-wrap text-sm leading-6 text-(--text-default)"
          id={messageId}
        >
          {message}
        </p>
        {failure ? (
          <div
            aria-atomic="true"
            aria-live={failure.urgency ?? "polite"}
            className={cn(
              "flex items-start gap-2.5 border-l-2 py-1 pl-3",
              failure.tone === "warning"
                ? "border-[color:color-mix(in_srgb,var(--warning)_42%,transparent)]"
                : "border-[color:color-mix(in_srgb,var(--destructive)_38%,transparent)]",
            )}
            role={failure.urgency === "assertive" ? "alert" : "status"}
          >
            <CircleAlert className={cn(
              "mt-0.5 h-3.5 w-3.5 shrink-0",
              failure.tone === "warning" ? "text-(--warning)" : "text-(--destructive)",
            )} />
            <div className="min-w-0 flex-1">
              <p className={getUiTypographyClassName({
                role: "supporting",
                tone: "strong",
                weight: "medium",
              })}>
                {failure.title}
              </p>
              <RecoverySummary
                className="mt-0.5"
                impact={failure.impact}
                nextStep={failure.nextStep}
              />
            </div>
          </div>
        ) : null}
      </UiDialogBody>
      <DecisionDialogActions
        busy={busy}
        cancelText={cancelText ?? t("common.cancel")}
        confirmButtonRef={confirmButtonRef}
        confirmClassName="min-w-[110px]"
        confirmText={confirmText ?? t("common.confirm")}
        confirmTone={presentation.actionTone}
        onCancel={onCancel}
        onConfirm={onConfirm}
      />
    </DecisionDialogFrame>
  );
}

function ignoreDialogClose(): void {}

export function PromptDialog({
  busy = false,
  cancelText,
  confirmText,
  defaultValue = "",
  error,
  inputLabel,
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
  const { t } = useI18n();
  if (!isOpen) {
    return null;
  }
  return (
    <PromptDialogContent
      busy={busy}
      cancelText={cancelText ?? t("common.cancel")}
      confirmText={confirmText ?? t("common.confirm")}
      defaultValue={defaultValue}
      error={error}
      inputLabel={inputLabel}
      key={defaultValue}
      message={message}
      multiline={multiline}
      onCancel={onCancel}
      onConfirm={onConfirm}
      placeholder={placeholder}
      rows={rows}
      shortcutHint={shortcutHint ?? t("dialog.prompt_shortcut_hint")}
      title={title}
    />
  );
}

function PromptDialogContent({
  busy,
  cancelText,
  confirmText,
  defaultValue,
  error,
  inputLabel,
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
  const messageId = useId();
  const inputId = useId();
  const mode: PromptInputMode = multiline ? "multiline" : "single";
  const initialFocusRef: RefObject<HTMLElement | null> = multiline
    ? textareaRef
    : inputRef;

  const cancel = () => {
    if (busy) return;
    setValue(defaultValue);
    onCancel();
  };
  const submit = () => { if (!busy) onConfirm(value); };
  const handleInputKeyDown = (
    event: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>,
  ) => {
    if (busy || isImeKeyboardEvent(event.nativeEvent)) return;
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
      describedBy={message ? messageId : undefined}
      initialFocusRef={initialFocusRef}
      labelledBy={titleId}
      onClose={cancel}
      size={multiline ? "sm" : "xs"}
    >
      <UiDialogHeader
        actions={<UiDialogCloseButton disabled={busy} onClose={cancel} />}
        appearance="plain"
        title={title}
        titleId={titleId}
      />
      <UiDialogBody className="space-y-3 px-5 pb-4 pt-2">
        {message ? (
          <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })} id={messageId}>{message}</p>
        ) : null}
        <UiField description={multiline ? shortcutHint : undefined} error={error} htmlFor={inputId}>
          <PromptInput
            busy={busy}
            inputId={inputId}
            inputLabel={inputLabel}
            inputRef={inputRef}
            mode={mode}
            onChange={setValue}
            onKeyDown={handleInputKeyDown}
            placeholder={placeholder}
            rows={rows}
            textareaRef={textareaRef}
            titleId={titleId}
            value={value}
          />
        </UiField>
      </UiDialogBody>
      <DecisionDialogActions
        busy={busy}
        cancelText={cancelText}
        confirmText={confirmText}
        onCancel={cancel}
        onConfirm={submit}
      />
    </DecisionDialogFrame>
  );
}

interface PromptDialogContentProps {
  busy: boolean;
  cancelText: string;
  confirmText: string;
  defaultValue: string;
  error?: string;
  inputLabel?: string;
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
  busy: boolean;
  inputId: string;
  inputLabel?: string;
  inputRef: RefObject<HTMLInputElement | null>;
  mode: PromptInputMode;
  onChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
  placeholder: string;
  rows?: number;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  titleId: string;
  value: string;
}

function PromptInput({
  busy,
  inputId,
  inputLabel,
  inputRef,
  mode,
  onChange,
  onKeyDown,
  placeholder,
  rows,
  textareaRef,
  titleId,
  value,
}: PromptInputProps) {
  if (mode === "multiline") {
    return (
      <UiTextarea
        aria-label={inputLabel}
        aria-labelledby={inputLabel ? undefined : titleId}
        className="min-h-[180px] leading-6"
        controlSize="lg"
        disabled={busy}
        id={inputId}
        onChange={(event) => onChange(event.target.value)}
        onFocus={movePromptTextareaCursorToEnd}
        onKeyDown={onKeyDown}
        placeholder={placeholder}
        ref={textareaRef}
        rows={rows}
        value={value}
      />
    );
  }
  return (
    <UiInput
      aria-label={inputLabel}
      aria-labelledby={inputLabel ? undefined : titleId}
      controlSize="lg"
      disabled={busy}
      id={inputId}
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
