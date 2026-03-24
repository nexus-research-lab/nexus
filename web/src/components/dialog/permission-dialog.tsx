/**
 * 权限确认对话框组件
 *
 * 当Agent需要使用某些工具时，显示此对话框请求用户授权
 */

"use client";

import { useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { AlertTriangle, X } from "lucide-react";
import { PermissionRiskLevel, PermissionUpdate } from "@/types/permission";

interface PermissionDialogProps {
  /** 是否显示对话框 */
  isOpen: boolean;
  /** 工具名称 */
  toolName: string;
  /** 工具输入参数 */
  toolInput: Record<string, any>;
  /** 风险级别 */
  riskLevel?: PermissionRiskLevel;
  /** 风险标签 */
  riskLabel?: string;
  /** 摘要 */
  summary?: string;
  /** SDK 建议的权限更新 */
  suggestions?: PermissionUpdate[];
  /** 过期时间 */
  expiresAt?: string;
  /** 允许回调 */
  onAllow: (updatedPermissions?: PermissionUpdate[]) => void;
  /** 拒绝回调 */
  onDeny: (updatedPermissions?: PermissionUpdate[]) => void;
  /** 关闭弹窗 */
  onClose: () => void;
}

export function PermissionDialog(
  {
    isOpen,
    toolName,
    toolInput,
    riskLevel,
    riskLabel,
    summary,
    suggestions = [],
    expiresAt,
    onAllow,
    onDeny,
    onClose
  }: PermissionDialogProps) {
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState<number>(-1);

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
    low: 'text-emerald-700 dark:text-emerald-300',
    medium: 'text-amber-700 dark:text-amber-300',
    high: 'text-red-700 dark:text-red-300',
  };

  // 格式化显示工具输入参数
  const formatToolInput = () => {
    const entries = Object.entries(toolInput);
    if (entries.length === 0) return null;

    return (
      <div className="mt-3 space-y-2">
        <p className="text-xs font-medium text-muted-foreground uppercase">参数详情</p>
        {entries.map(([key, value]) => (
          <div key={key} className="neo-inset radius-shell-sm p-3">
            <span className="text-xs font-medium text-foreground">{key}:</span>
            <pre className="mt-1 text-xs text-muted-foreground overflow-auto max-h-32 whitespace-pre-wrap break-words">
              {typeof value === 'string' && value.length > 200 ? value.substring(0, 200) + '...'
                : typeof value === 'object' ? JSON.stringify(value, null, 2) : String(value)}
            </pre>
          </div>
        ))}
      </div>
    );
  };

  if (!isOpen) return null;

  // 使用 Portal 渲染到 body
  if (typeof document === 'undefined') return null;

  return createPortal(
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-200">
      <div
        className="soft-ring radius-shell-lg panel-surface flex w-full max-w-lg flex-col overflow-hidden animate-in zoom-in-95 duration-200">
        {/* 头部 */}
        <div className="flex items-center justify-between border-b border-white/55 bg-orange-500/10 px-5 py-4">
          <div className="flex items-center gap-3">
            <div className="neo-pill radius-shell-sm flex h-10 w-10 items-center justify-center bg-orange-500/15 text-orange-600">
              <AlertTriangle className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-semibold text-foreground tracking-tight">
                权限确认
              </h2>
              <p className="text-xs text-muted-foreground">
                Agent 想要使用工具
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="neo-pill radius-shell-sm p-2 text-muted-foreground transition-colors hover:text-foreground"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* 内容 */}
        <div className="p-5 space-y-4 max-h-[60vh] overflow-auto">
          <div className="radius-shell-md border border-orange-500/20 bg-orange-500/10 p-4">
            <p className="text-sm font-medium text-orange-700 dark:text-orange-400">
              Agent 想要使用「{toolName}」工具
            </p>
            <p className="text-xs text-orange-600/90 dark:text-orange-300/90 mt-1.5 leading-relaxed">
              允许后，Agent 将执行此工具对应的操作。请检查下方参数是否正确。
            </p>
            {(summary || riskLabel || expiresAt) && (
              <div className="mt-3 space-y-1 text-xs">
                {summary && (
                  <p className="text-foreground/80 break-all">
                    <span className="font-medium text-foreground">摘要：</span>
                    {summary}
                  </p>
                )}
                {riskLabel && (
                  <p className={riskLevel ? riskColorMap[riskLevel] : 'text-foreground/80'}>
                    <span className="font-medium text-foreground">风险：</span>
                    {riskLabel}
                  </p>
                )}
                {expiresAt && (
                  <p className="text-foreground/70">
                    <span className="font-medium text-foreground">过期：</span>
                    {new Date(expiresAt).toLocaleString()}
                  </p>
                )}
              </div>
            )}
          </div>

          {readableSuggestions.length > 0 && (
            <div className="space-y-2">
              <p className="text-xs font-medium text-muted-foreground uppercase">记住这次授权</p>
              <div className="space-y-2">
                <label className="neo-card-flat radius-shell-md flex items-start gap-3 p-3">
                  <input
                    type="radio"
                    name="permission-suggestion"
                    checked={selectedSuggestionIndex === -1}
                    onChange={() => setSelectedSuggestionIndex(-1)}
                    className="mt-0.5"
                  />
                  <div>
                    <p className="text-sm font-medium text-foreground">仅本次生效</p>
                    <p className="text-xs text-muted-foreground">这次允许，下次再问你</p>
                  </div>
                </label>
                {readableSuggestions.map((suggestion) => (
                  <label key={suggestion.index} className="neo-card-flat radius-shell-md flex items-start gap-3 p-3">
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

        {/* 底部按钮 */}
        <div className="flex items-center justify-end gap-3 border-t border-white/55 px-5 py-4">
          <button
            onClick={() => onDeny()}
            className="neo-pill radius-shell-sm px-4 py-2 text-sm font-medium transition-colors hover:text-accent"
          >
            拒绝
          </button>
          <button
            onClick={() => {
              const selectedUpdate = selectedSuggestionIndex >= 0
                ? [suggestions[selectedSuggestionIndex]]
                : undefined;
              onAllow(selectedUpdate);
            }}
            className="radius-shell-sm bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-[0_16px_28px_rgba(133,119,255,0.22)] transition-colors hover:bg-primary/90"
          >
            允许
          </button>
        </div>
      </div>
    </div>,
    document.body
  );
}
