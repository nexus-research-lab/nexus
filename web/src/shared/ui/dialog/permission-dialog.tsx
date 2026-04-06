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
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_TAG_CLASS_NAME,
  getDialogNoteClassName,
} from "@/shared/ui/dialog/dialog-styles";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
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

const formatInlineValue = (value: unknown): string => {
  if (value == null || value === '') return '空';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) {
    return value.map((item) => formatInlineValue(item)).join('、');
  }
  if (typeof value === 'object') {
    const pairs = Object.entries(value as Record<string, unknown>)
      .slice(0, 4)
      .map(([key, nestedValue]) => `${FIELD_LABEL_MAP[key] || key}：${formatInlineValue(nestedValue)}`);
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

  const riskColorMap: Record<PermissionRiskLevel, string> = {
    low: 'bg-emerald-100 text-emerald-700',
    medium: 'bg-amber-100 text-amber-700',
    high: 'bg-red-100 text-red-700',
  };

  const readableFields = useMemo(() => {
    return Object.entries(tool_input).map(([key, value]) => ({
      key,
      label: FIELD_LABEL_MAP[key] || key,
      value: formatInlineValue(value),
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
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!is_open) return;
      if (event.key === "Escape") {
        event.preventDefault();
        on_close();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [is_open, on_close]);

  // 格式化显示工具输入参数
  const formatToolInput = () => {
    if (readableFields.length === 0) return null;

    return (
      <div className="space-y-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">Parameters</p>
          <h3 className="mt-1 text-base font-semibold text-slate-900">参数</h3>
        </div>
        {readableFields.map((field) => (
          <div key={field.key} className="dialog-card radius-shell-md px-4 py-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
              {field.label}
            </p>
            <p className="mt-2 text-sm leading-7 whitespace-pre-wrap break-words text-slate-800">
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
      className="dialog-backdrop z-9999 animate-in fade-in duration-150"
      onClick={on_close}
    >
      <div
        className="dialog-shell radius-shell-xl flex w-full max-w-2xl flex-col overflow-hidden animate-in zoom-in-95 duration-150"
        onClick={(event) => event.stopPropagation()}
        style={{ maxHeight: "80vh" }}
      >
        <div className="dialog-header">
          <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
            <div
              className={cn(
                DIALOG_HEADER_ICON_CLASS_NAME,
                "h-14 w-14 rounded-[20px]",
                risk_level === "high" && "bg-red-100 text-red-600",
                risk_level === "medium" && "bg-amber-100 text-amber-600",
                (!risk_level || risk_level === "low") && "bg-emerald-100 text-emerald-600",
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
          <WorkspacePillButton
            aria-label="关闭"
            density="compact"
            onClick={on_close}
            size="icon"
            variant="icon"
          >
            <X className="h-5 w-5" />
          </WorkspacePillButton>
        </div>

        <div className="dialog-body dialog-body--scroll soft-scrollbar space-y-5">
          <div className="mb-1 flex flex-wrap gap-2">
            {risk_label ? (
              <span className={cn(DIALOG_TAG_CLASS_NAME, risk_level === "high" && "text-rose-700", risk_level === "medium" && "text-amber-700", (!risk_level || risk_level === "low") && "text-emerald-700")}>
                {risk_label}
              </span>
            ) : null}
            {expires_at ? (
              <span className={DIALOG_TAG_CLASS_NAME}>
                {new Date(expires_at).toLocaleString()} 前确认
              </span>
            ) : null}
          </div>

          <div className={getDialogNoteClassName("default")}>
            <div className="text-[15px] leading-8 break-words text-slate-800">
              {summary || "确认后继续执行"}
            </div>
          </div>

          {readableSuggestions.length > 0 && (
            <div className="space-y-3">
              <div>
                <p className="dialog-label">Policy</p>
                <h3 className="mt-1 text-base font-semibold text-slate-900">授权范围</h3>
              </div>
              <div className="space-y-2">
                <label
                  className={cn(
                    "radius-shell-md flex items-start gap-3 px-4 py-3 transition-all duration-200",
                    selectedSuggestionIndex === -1
                      ? "dialog-card-active"
                      : "dialog-card hover:border-primary/20 hover:bg-white/80",
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
                      "radius-shell-md flex items-start gap-3 px-4 py-3 transition-all duration-200",
                      selectedSuggestionIndex === suggestion.index
                        ? "dialog-card-active"
                        : "dialog-card hover:border-primary/20 hover:bg-white/80",
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

          {formatToolInput()}
        </div>

        <div className="dialog-footer">
          <WorkspacePillButton
            onClick={() => on_deny()}
            size="md"
            variant="tonal"
          >
            拒绝
          </WorkspacePillButton>
          <WorkspacePillButton
            ref={confirmButtonRef}
            onClick={() => {
              const selectedUpdate = selectedSuggestionIndex >= 0
                ? [suggestions[selectedSuggestionIndex]]
                : undefined;
              on_allow(selectedUpdate);
            }}
            size="md"
            variant="primary"
          >
            允许
          </WorkspacePillButton>
        </div>
      </div>
    </div>,
    document.body
  );
}
