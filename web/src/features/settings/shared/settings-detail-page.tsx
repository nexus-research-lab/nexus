// INPUT: 能力型设置分区的图标、名称、开关或主动作、示例任务、分组内容与信息行。
// OUTPUT: 插件详情式版式：身份头、示例横幅、说明、分组小节与键值信息表。
// POS: 设置域共享 Pattern；电脑操控与浏览器共用同一节奏，分区只传入自身数据与动作。
"use client";

import type { CSSProperties, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

// 详情正文使用窄版心，长文说明与权限行保持舒适行长。
export const SETTINGS_DETAIL_BODY_CLASS_NAME = "mx-auto w-full max-w-[720px] space-y-7 pb-10 pt-1";

const MUTED_CLASS_NAME = getUiTypographyClassName({ role: "supporting", tone: "muted" });
const SECTION_HEADING_CLASS_NAME = getUiTypographyClassName({ role: "sectionTitle", tone: "strong" });

// 品牌渐变只由主色与面板 token 混出，深浅主题随 token 自动切换；任意值类名无法稳定解析多层渐变，改用内联样式。
const ICON_TILE_STYLE: CSSProperties = {
  backgroundImage: "linear-gradient(135deg, color-mix(in oklch, var(--primary) 60%, var(--primary-foreground)), var(--primary) 60%)",
};
const EXAMPLES_STYLE: CSSProperties = {
  backgroundImage: [
    "radial-gradient(110% 90% at 12% 0%, color-mix(in oklch, var(--primary) 26%, transparent), transparent 62%)",
    "radial-gradient(80% 80% at 90% 20%, color-mix(in oklch, var(--primary) 16%, transparent), transparent 66%)",
    "radial-gradient(70% 70% at 55% 110%, color-mix(in oklch, var(--primary) 14%, transparent), transparent 70%)",
  ].join(", "),
  backgroundColor: "color-mix(in oklch, var(--primary) 5%, var(--surface-panel-background))",
};

export function SettingsDetailIcon({ children, size = "lg" }: { children: ReactNode; size?: "sm" | "lg" }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "flex shrink-0 items-center justify-center text-(--primary-foreground)",
        size === "lg" ? "h-11 w-11 radius-control-lg [&_svg]:h-5 [&_svg]:w-5" : "h-4 w-4 radius-control-xs [&_svg]:h-2.5 [&_svg]:w-2.5",
      )}
      style={ICON_TILE_STYLE}
    >
      {children}
    </span>
  );
}

interface SettingsDetailHeaderProps {
  action?: ReactNode;
  icon: ReactNode;
  subtitle: ReactNode;
  subtitleId?: string;
  title: string;
}

export function SettingsDetailHeader({ action, icon, subtitle, subtitleId, title }: SettingsDetailHeaderProps) {
  return (
    <header className="flex items-center gap-3.5">
      <SettingsDetailIcon>{icon}</SettingsDetailIcon>
      <div className="min-w-0 flex-1">
        <h2 className={getUiTypographyClassName({ role: "objectTitle", tone: "strong" })}>{title}</h2>
        <div className={cn("mt-0.5", MUTED_CLASS_NAME)} id={subtitleId}>{subtitle}</div>
      </div>
      {action ? <div className="flex shrink-0 items-center gap-2">{action}</div> : null}
    </header>
  );
}

interface SettingsDetailExamplesProps {
  icon: ReactNode;
  items: readonly string[];
  label: string;
  name: string;
}

/** 示例只说明能做什么，不承载发送动作。 */
export function SettingsDetailExamples({ icon, items, label, name }: SettingsDetailExamplesProps) {
  return (
    <ul
      aria-label={label}
      className="flex flex-col items-center justify-center gap-2 surface-radius-lg px-5 py-7"
      style={EXAMPLES_STYLE}
    >
      {items.map((item) => (
        <li
          className="flex max-w-full items-start gap-2 radius-control-lg border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_80%,transparent)] px-3 py-1.5 shadow-xs backdrop-blur-md"
          key={item}
        >
          <span className="mt-[3px]"><SettingsDetailIcon size="sm">{icon}</SettingsDetailIcon></span>
          <span className={getUiTypographyClassName({ role: "supporting", tone: "default" })}>
            <span className="font-medium text-(--text-strong)">{name}</span>
            {" "}
            {item}
          </span>
        </li>
      ))}
    </ul>
  );
}

export function SettingsDetailOverview({ children }: { children: ReactNode }) {
  return <p className={getUiTypographyClassName({ role: "supporting", tone: "default" })}>{children}</p>;
}

interface SettingsDetailSectionProps {
  actions?: ReactNode;
  children: ReactNode;
  meta?: ReactNode;
  title: string;
}

export function SettingsDetailSection({ actions, children, meta, title }: SettingsDetailSectionProps) {
  return (
    <section className="space-y-3">
      <div className="flex min-h-7 flex-wrap items-center gap-x-3 gap-y-2 border-b border-(--divider-subtle-color) pb-2.5">
        <h3 className={SECTION_HEADING_CLASS_NAME}>{title}</h3>
        {meta}
        {actions ? <div className="ml-auto flex items-center gap-1.5">{actions}</div> : null}
      </div>
      {children}
    </section>
  );
}

export function SettingsDetailInfo({ rows, title }: { rows: [label: string, value: ReactNode][]; title: string }) {
  return (
    <SettingsDetailSection title={title}>
      <dl className="grid grid-cols-[minmax(88px,140px)_minmax(0,1fr)] gap-x-6 gap-y-2.5">
        {rows.map(([label, value]) => (
          <div className="contents" key={label}>
            <dt className={MUTED_CLASS_NAME}>{label}</dt>
            <dd className={cn("min-w-0 [overflow-wrap:anywhere]", getUiTypographyClassName({ role: "supporting", tone: "strong" }))}>{value}</dd>
          </div>
        ))}
      </dl>
    </SettingsDetailSection>
  );
}
