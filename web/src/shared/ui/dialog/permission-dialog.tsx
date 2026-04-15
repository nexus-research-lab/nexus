/**
 * 权限确认对话框组件
 *
 * 当Agent需要使用某些工具时，显示此对话框请求用户授权
 */

"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { AlertTriangle, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { GlassPanel } from "@/shared/ui/liquid-glass";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_TAG_CLASS_NAME,
  get_dialog_action_class_name,
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";
import { PermissionRiskLevel, PermissionUpdate } from "@/types/permission";

interface PermissionDialogProps {
  is_open: boolean;
  tool_name: string;
  tool_input: Record<string, any>;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
  on_allow: (updated_permissions?: PermissionUpdate[]) => void;
  on_deny: (updated_permissions?: PermissionUpdate[]) => void;
  on_close: () => void;
}

const FIELD_LABEL_MAP: Record<string, string> = {
  query: '搜索内容',
  url: '网址',
  command: '命令',
  path: '路径',
  file_path: '文件路径',
  pattern: '匹配内容',
  prompt: '提示词',
  description: '说明',
  task: '任务',
  mode: '模式',
  directories: '目录',
  answers: '回答',
};

const format_inline_value = (value: unknown): string => {
  if (value == null || value === '') return '空';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) {
    return value.map((item) => format_inline_value(item)).join('、');
  }
  if (typeof value === 'object') {
    const pairs = Object.entries(value as Record<string, unknown>)
      .slice(0, 4)
      .map(([key, nestedValue]) => `${FIELD_LABEL_MAP[key] || key}：${format_inline_value(nestedValue)}`);
    return pairs.join('；');
  }
  return String(value);
};

export function PermissionDialog(
  {
    is_open,
    tool_name,
    tool_input,
    risk_level,
    risk_label,
    summary,
    suggestions = [],
    expires_at,
    on_allow,
    on_deny,
    on_close
  }: PermissionDialogProps) {
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState<number>(-1);
  const confirmButtonRef = useRef<HTMLButtonElement>(null);

  const readableSuggestions = useMemo(() => {
    return suggestions.map((suggestion, index) => {
      const destinationMap: Record<string, string> = {
        session: '仅本会话',
        projectSettings: '项目设置',
        userSettings: '用户设置',
      };
      const behaviorMap: Record<string, string> = {
        allow: '允许',
        deny: '拒绝',
        ask: '继续询问',
      };
      const destination = suggestion.destination ? destinationMap[suggestion.destination] || suggestion.destination : '当前会话';
      const behavior = suggestion.behavior ? behaviorMap[suggestion.behavior] || suggestion.behavior : '更新规则';
      const ruleSummary = suggestion.rules?.map((rule) => rule.rule_content || rule.tool_name).filter(Boolean).join('，');
      return {
        index,
        label: `${behavior}并写入${destination}`,
        description: ruleSummary || suggestion.type,
      };
    });
  }, [suggestions]);

  const readableFields = useMemo(() => {
    return Object.entries(tool_input).map(([key, value]) => ({
      key,
      label: FIELD_LABEL_MAP[key] || key,
      value: format_inline_value(value),
    }));
  }, [tool_input]);

  useEffect(() => {
    if (is_open && confirmButtonRef.current) {
      confirmButtonRef.current.focus();
    }
  }, [is_open]);

  useEffect(() => {
    if (is_open) {
      setSelectedSuggestionIndex(-1);
    }
  }, [is_open]);

  useEffect(() => {
    const handle_key_down = (event: KeyboardEvent) => {
      if (!is_open) return;
      if (event.key === "Escape") {
        event.preventDefault();
        on_close();
      }
    };
    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_open, on_close]);

  // 格式化显示工具输入参数
  const format_tool_input = () => {
    if (readableFields.length === 0) return null;

    return (
      <div className="space-y-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">Parameters</p>
          <h3 className="mt-1 text-base font-semibold text-(--text-strong)">参数</h3>
        </div>
        {readableFields.map((field) => (
          <div key={field.key} className="rounded-[16px] border border-(--divider-subtle-color) px-4 py-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
              {field.label}
            </p>
            <p className="mt-2 text-sm leading-7 whitespace-pre-wrap break-words text-(--text-default)">
              {field.value}
            </p>
          </div>
        ))}
      </div>
    );
  };

  if (!is_open) return null;

  // 使用 Portal 渲染到 body
  if (typeof document === 'undefined') return null;

  return createPortal(
    <div
      className="dialog-backdrop z-9999 animate-in fade-in duration-(--motion-duration-fast)"
      onClick={on_close}
    >
      <GlassPanel
        true_glass
        class_name="radius-shell-xl flex w-full max-w-2xl flex-col overflow-hidden animate-in zoom-in-95 duration-(--motion-duration-fast)"
        content_layout="fill-flex"
        content_class_name="min-h-0"
        onClick={(event) => event.stopPropagation()}
        radius={34}
        style={{ maxHeight: "80vh" }}
      >
        <div className="dialog-header">
          <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
            <div
              className={cn(
                DIALOG_HEADER_ICON_CLASS_NAME,
                "h-14 w-14 rounded-[20px]",
                risk_level === "high" && "border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_12%,transparent)] text-[color:color-mix(in_srgb,var(--destructive)_80%,white)]",
                risk_level === "medium" && "border border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-[color:color-mix(in_srgb,var(--warning)_80%,white)]",
                (!risk_level || risk_level === "low") && "border border-[color:color-mix(in_srgb,var(--success)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_12%,transparent)] text-[color:color-mix(in_srgb,var(--success)_80%,white)]",
              )}
            >
              <AlertTriangle className="h-7 w-7" />
            </div>
            <div className="min-w-0">
              <h2 className="dialog-title truncate" data-size="hero">
                {tool_name}
              </h2>
              <p className="dialog-subtitle">{risk_label || "需要确认"}</p>
            </div>
          </div>
          <button
            aria-label="关闭"
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            onClick={on_close}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="dialog-body dialog-body--scroll soft-scrollbar space-y-5">
          <div className="mb-1 flex flex-wrap gap-2">
            {risk_label ? (
              <span className={cn(DIALOG_TAG_CLASS_NAME, risk_level === "high" && "text-[color:color-mix(in_srgb,var(--destructive)_80%,white)]", risk_level === "medium" && "text-[color:color-mix(in_srgb,var(--warning)_80%,white)]", (!risk_level || risk_level === "low") && "text-[color:color-mix(in_srgb,var(--success)_80%,white)]")}>
                {risk_label}
              </span>
            ) : null}
            {expires_at ? (
              <span className={DIALOG_TAG_CLASS_NAME}>
                {new Date(expires_at).toLocaleString()} 前确认
              </span>
            ) : null}
          </div>

          <div className={get_dialog_note_class_name("default")} style={get_dialog_note_style("default")}>
            <div className="text-[15px] leading-8 break-words text-(--text-default)">
              {summary || "确认后继续执行"}
            </div>
          </div>

          {readableSuggestions.length > 0 && (
            <div className="space-y-3">
              <div>
                <p className="dialog-label">Policy</p>
                <h3 className="mt-1 text-base font-semibold text-(--text-strong)">授权范围</h3>
              </div>
              <div className="space-y-2">
                <label
                  className={cn(
                    "radius-shell-md flex items-start gap-3 px-4 py-3 transition-all duration-(--motion-duration-normal)",
                    selectedSuggestionIndex === -1
                      ? "dialog-card-active"
                      : "dialog-card hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
                  )}
                >
                  <input
                    type="radio"
                    name="permission-suggestion"
                    checked={selectedSuggestionIndex === -1}
                    onChange={() => setSelectedSuggestionIndex(-1)}
                    className="mt-0.5"
                  />
                  <div>
                    <p className="text-sm font-medium text-foreground">仅这次</p>
                    <p className="text-xs text-muted-foreground">只对这一次生效</p>
                  </div>
                </label>
                {readableSuggestions.map((suggestion) => (
                  <label
                    key={suggestion.index}
                    className={cn(
                      "radius-shell-md flex items-start gap-3 px-4 py-3 transition-all duration-(--motion-duration-normal)",
                      selectedSuggestionIndex === suggestion.index
                        ? "dialog-card-active"
                        : "dialog-card hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
                    )}
                  >
                    <input
                      type="radio"
                      name="permission-suggestion"
                      checked={selectedSuggestionIndex === suggestion.index}
                      onChange={() => setSelectedSuggestionIndex(suggestion.index)}
                      className="mt-0.5"
                    />
                    <div>
                      <p className="text-sm font-medium text-foreground">{suggestion.label}</p>
                      <p className="text-xs text-muted-foreground break-all">{suggestion.description}</p>
                    </div>
                  </label>
                ))}
              </div>
            </div>
          )}

          {format_tool_input()}
        </div>

        <div className="dialog-footer">
          <button
            className={get_dialog_action_class_name("default")}
            onClick={() => on_deny()}
            type="button"
          >
            拒绝
          </button>
          <button
            className={get_dialog_action_class_name("primary")}
            ref={confirmButtonRef}
            onClick={() => {
              const selectedUpdate = selectedSuggestionIndex >= 0
                ? [suggestions[selectedSuggestionIndex]]
                : undefined;
              on_allow(selectedUpdate);
            }}
            type="button"
          >
            允许
          </button>
        </div>
      </GlassPanel>
    </div>,
    document.body
  );
}
