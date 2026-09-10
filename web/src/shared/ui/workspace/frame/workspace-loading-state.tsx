// INPUT: 工作区区域加载中的用户可见标签。
// OUTPUT: 填满可用工作区、可访问且遵循共享旋转与排版规范的居中状态。
// POS: Workspace Frame 加载布局；不拥有加载图标 recipe 或判断资源状态。

"use client";

import { LoaderCircle } from "lucide-react";

import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface WorkspaceLoadingStateProps {
  label: string;
}

export function WorkspaceLoadingState({ label }: WorkspaceLoadingStateProps) {
  return (
    <div
      aria-atomic="true"
      aria-busy="true"
      aria-live="polite"
      className="flex min-h-0 min-w-0 flex-1 items-center justify-center"
      role="status"
    >
      <div className="flex min-w-0 max-w-full flex-col items-center gap-3 text-center">
        <LoaderCircle
          aria-hidden
          className={getUiSpinnerClassName({ size: "xl", tone: "muted" })}
        />
        <span className={`max-w-full wrap-anywhere ${getUiTypographyClassName({ role: "supporting", tone: "soft" })}`}>
          {label}
        </span>
      </div>
    </div>
  );
}
