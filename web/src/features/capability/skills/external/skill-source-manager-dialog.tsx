"use client";

import { Database, Loader2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { GlassSwitch } from "@/shared/ui/liquid-glass";
import type { ExternalSkillSourceInfo } from "@/types/capability/skill";

interface SkillSourceManagerDialogProps {
  isOpen: boolean;
  loading: boolean;
  onClose: () => void;
  onToggle: (source: ExternalSkillSourceInfo, enabled: boolean) => void;
  sources: ExternalSkillSourceInfo[];
}

const SOURCE_KIND_LABELS: Record<string, string> = {
  browse_sh: "browse.sh",
  claude_plugins: "claude-plugins.dev",
  clawhub: "clawhub.ai",
  git: "Git",
  hermes_index: "Hermes Index",
  skills_sh: "skills.sh",
  url: "URL",
  well_known: "Well-known",
};

const SOURCE_KIND_DESCRIPTIONS: Record<string, string> = {
  browse_sh: "网站自动化 Skill 注册表，后端从 browse.sh catalog 解析 raw SKILL.md。",
  claude_plugins: "Claude Plugins 注册表，后端按 GitHub 仓库和子目录导入。",
  clawhub: "ClawHub 注册表，后端按 slug 下载 zip 包导入。",
  hermes_index: "Hermes 聚合索引，覆盖 GitHub taps、skills.sh、ClawHub、LobeHub 和 browse.sh 等来源；默认关闭以避免重复结果。",
  skills_sh: "skills.sh 注册表，后端按 GitHub 仓库稀疏导入指定技能目录。",
  well_known: "后端内置的 well-known 索引适配器。",
};

function sourceKindLabel(kind: string): string {
  return SOURCE_KIND_LABELS[kind] || kind;
}

function sourceKindDescription(source: ExternalSkillSourceInfo): string {
  return SOURCE_KIND_DESCRIPTIONS[source.kind] || "后端内置来源适配器。";
}

export function SkillSourceManagerDialog({
  isOpen,
  loading,
  onClose,
  onToggle,
  sources,
}: SkillSourceManagerDialogProps) {
  const { t } = useI18n();
  if (!isOpen) return null;

  const sortedSources = [...sources].sort(
    (a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name),
  );

  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" onClose={onClose}>
        <UiDialogShell className="h-[76vh]" size="lg">
          <UiDialogHeader
            icon={<Database className="h-4 w-4" />}
            onClose={onClose}
            subtitle={t("capability.skill_sources_description")}
            title={t("capability.skill_sources_title")}
          />
          <UiDialogBody className="space-y-3" scrollable>
            {loading && !sortedSources.length ? (
              <div className="flex items-center justify-center gap-2 py-12 text-sm text-(--text-soft)">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t("capability.skill_sources_loading")}
              </div>
            ) : sortedSources.length ? (
              sortedSources.map((source) => (
                <SourceRow
                  key={source.source_id}
                  disabled={loading}
                  onToggle={(enabled) => onToggle(source, enabled)}
                  source={source}
                />
              ))
            ) : (
              <div className="rounded-[12px] border border-dashed border-(--divider-subtle-color) px-4 py-8 text-center text-sm text-(--text-soft)">
                {t("capability.skill_sources_empty")}
              </div>
            )}
          </UiDialogBody>

          <UiDialogFooter className="gap-2">
            <UiButton
              disabled={loading}
              onClick={onClose}
              size="sm"
              variant="surface"
            >
              {t("common.close")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

interface SourceRowProps {
  disabled: boolean;
  onToggle: (enabled: boolean) => void;
  source: ExternalSkillSourceInfo;
}

function SourceRow({ disabled, onToggle, source }: SourceRowProps) {
  return (
    <div
      className={cn(
        "flex min-w-0 items-center gap-4 rounded-[14px] border px-4 py-3",
        source.enabled
          ? "border-[color:color-mix(in_srgb,var(--primary)_34%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)]"
          : "border-(--divider-subtle-color) bg-transparent",
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span className="truncate text-sm font-semibold text-(--text-strong)">
            {source.name}
          </span>
          <UiBadge size="xs">{sourceKindLabel(source.kind)}</UiBadge>
          <UiBadge size="xs" tone={source.enabled ? "success" : "idle"}>
            {source.enabled ? "已启用" : "已停用"}
          </UiBadge>
        </div>
        <div className="mt-1 truncate text-xs text-(--text-muted)">
          {source.url}
        </div>
        <div className="mt-1 text-xs leading-5 text-(--text-soft)">
          {sourceKindDescription(source)}
        </div>
        {source.last_error ? (
          <div className="mt-1 truncate text-xs text-(--destructive)">
            {source.last_error}
          </div>
        ) : null}
      </div>
      <div className="shrink-0">
        <GlassSwitch
          checked={source.enabled}
          disabled={disabled}
          onChange={onToggle}
          size="sm"
        />
      </div>
    </div>
  );
}
