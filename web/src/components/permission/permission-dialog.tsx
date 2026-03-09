/**
 * 权限确认对话框组件
 *
 * 当Agent需要使用某些工具时，显示此对话框请求用户授权
 */

"use client";

import { createPortal } from "react-dom";
import { AlertTriangle, X } from "lucide-react";

interface PermissionDialogProps {
  /** 是否显示对话框 */
  isOpen: boolean;
  /** 工具名称 */
  toolName: string;
  /** 工具输入参数 */
  toolInput: Record<string, any>;
  /** 允许回调 */
  onAllow: () => void;
  /** 拒绝回调 */
  onDeny: () => void;
  /** 关闭弹窗 */
  onClose: () => void;
}

export function PermissionDialog(
  {
    isOpen,
    toolName,
    toolInput,
    onAllow,
    onDeny,
    onClose
  }: PermissionDialogProps) {
  if (!isOpen) return null;

  // 使用 Portal 渲染到 body
  if (typeof document === 'undefined') return null;

  // 格式化显示工具输入参数
  const formatToolInput = () => {
    const entries = Object.entries(toolInput);
    if (entries.length === 0) return null;

    return (
      <div className="mt-3 space-y-2">
        <p className="text-xs font-medium text-muted-foreground uppercase">参数:</p>
        {entries.map(([key, value]) => (
          <div key={key} className="bg-muted/30 rounded-md p-3">
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

  return createPortal(
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/60 backdrop-blur-sm animate-in fade-in duration-200">
      <div
        className="bg-background border border-border w-full max-w-lg flex flex-col shadow-2xl rounded-xl overflow-hidden animate-in zoom-in-95 duration-200">
        {/* 头部 */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border bg-orange-500/10">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-orange-500/20 flex items-center justify-center text-orange-600">
              <AlertTriangle className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-semibold text-foreground tracking-tight">
                权限确认
              </h2>
              <p className="text-xs text-muted-foreground">
                Agent 请求使用工具
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-full hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* 内容 */}
        <div className="p-5 space-y-4 max-h-[60vh] overflow-auto">
          <div className="bg-orange-500/10 border border-orange-500/20 rounded-lg p-4">
            <p className="text-sm font-medium text-orange-700 dark:text-orange-400">
              Agent 请求使用"{toolName}"工具
            </p>
            <p className="text-xs text-orange-600/90 dark:text-orange-300/90 mt-1.5 leading-relaxed">
              允许此操作将使 Agent 能够执行相应的系统操作。请仔细检查参数后再决定。
            </p>
          </div>

          {formatToolInput()}
        </div>

        {/* 底部按钮 */}
        <div className="flex items-center justify-end gap-3 px-5 py-4 border-t border-border bg-muted/30">
          <button
            onClick={onDeny}
            className="px-4 py-2 rounded-md border border-input bg-background hover:bg-accent hover:text-accent-foreground transition-colors text-sm font-medium shadow-sm"
          >
            拒绝
          </button>
          <button
            onClick={onAllow}
            className="px-4 py-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors text-sm font-medium shadow-sm"
          >
            允许
          </button>
        </div>
      </div>
    </div>,
    document.body
  );
}
