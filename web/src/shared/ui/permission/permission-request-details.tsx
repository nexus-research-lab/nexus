/**
 * INPUT: 权限请求的风险说明与结构化参数。
 * OUTPUT: 可复用的权限确认详情布局；不持有批准、拒绝或业务状态。
 * POS: Composer 与其他人工确认入口共用的权限详情展示组件。
 */
import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export interface PermissionRequestDetailField {
  label: string;
  value: ReactNode;
}

export interface PermissionRequestDetailsProps {
  description: ReactNode;
  fields: PermissionRequestDetailField[];
  title: ReactNode;
  tone?: "warning" | "neutral";
}

export function PermissionRequestDetails({
  description,
  fields,
  title,
  tone = "neutral",
}: PermissionRequestDetailsProps) {
  return (
    <div className="space-y-3" data-permission-request-details data-testid="permission-request-details">
      <div className={cn(
        "surface-radius-md border px-4 py-3",
        tone === "warning"
          ? "border-[color:color-mix(in_srgb,var(--warning)_38%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)]"
          : "border-(--divider-subtle-color) bg-(--surface-panel-subtle-background)",
      )}>
        <div className={cn(
          "mb-1",
          getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" }),
        )}>
          {title}
        </div>
        <p className={cn(
          "m-0 max-w-[62rem]",
          getUiTypographyClassName({ role: "caption", tone: "muted" }),
        )}>
          {description}
        </p>
      </div>
      <div className="grid min-w-0 gap-3 lg:grid-cols-[minmax(0,1.45fr)_minmax(15rem,1fr)]">
        {fields.map((field) => (
          <div className="min-w-0" key={field.label}>
            <div className={cn(
              "mb-1.5",
              getUiTypographyClassName({ role: "caption", tone: "muted", weight: "medium" }),
            )}>
              {field.label}
            </div>
            <pre className={cn(
              "message-cjk-font m-0 max-h-32 min-h-11 overflow-auto whitespace-pre-wrap break-all rounded-xl border border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) px-3 py-2.5",
              getUiTypographyClassName({ role: "code", tone: "default" }),
            )}>
              {field.value || "—"}
            </pre>
          </div>
        ))}
      </div>
    </div>
  );
}
