import { Download, Loader2, Puzzle } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/badge";
import { UiListActionButton } from "@/shared/ui/list-action";
import { UiListRow } from "@/shared/ui/list-row";
import type { ExternalSkillSearchItem } from "@/types/capability/skill";

import { format_installs } from "./skills-helpers";
import { SkillStatePill } from "./skill-state-pill";
import type { SkillMarketplaceController } from "./skills-view-model";

interface SkillsExternalResultsProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsExternalResults({ ctrl }: SkillsExternalResultsProps) {
  const { t } = useI18n();

  if (ctrl.external_loading) {
    return (
      <div className="flex items-center justify-center gap-2 py-12 text-sm text-(--text-soft)">
        <Loader2 className="h-4 w-4 animate-spin" />
        {t("capability.skills_external_loading")}
      </div>
    );
  }

  if (ctrl.external_query && !ctrl.external_results.length) {
    return (
      <div className="rounded-[12px] border border-dashed border-(--divider-subtle-color) px-5 py-8 text-center text-sm text-(--text-soft)">
        {t("capability.skills_external_empty")}
      </div>
    );
  }

  if (!ctrl.external_results.length) return null;

  return (
    <section>
      <div className="mb-3 flex items-end justify-between border-b border-(--divider-subtle-color) pb-2">
        <h2 className="text-[18px] font-medium tracking-[-0.025em] text-(--text-strong)">
          {t("capability.search_results")}
        </h2>
        <span className="text-[12px] font-medium text-(--text-soft)">
          {t("capability.result_count", { count: ctrl.external_results.length })}
        </span>
      </div>
      <div className="grid grid-cols-1 gap-x-12 gap-y-4 md:grid-cols-2">
        {ctrl.external_results.map((item: ExternalSkillSearchItem) => (
          <ExternalResultRow
            key={`${item.package_spec}@${item.skill_slug}`}
            busy_external_key={ctrl.busy_external_key}
            imported_external_sources={ctrl.imported_external_sources}
            item={item}
            on_import={() => void ctrl.handle_import_external(item)}
            on_preview={() => ctrl.handle_preview_external(item)}
          />
        ))}
      </div>
    </section>
  );
}

/* ── 外部结果行 ─────────────────────────────── */

interface ExternalResultRowProps {
  item: ExternalSkillSearchItem;
  busy_external_key: string | null;
  imported_external_sources: Map<string, Set<string>>;
  on_preview: () => void;
  on_import: () => void;
}

function ExternalResultRow({
  item,
  busy_external_key,
  imported_external_sources,
  on_preview,
  on_import,
}: ExternalResultRowProps) {
  const imported_sources = imported_external_sources.get(item.skill_slug);
  const already_imported = imported_sources?.has(item.package_spec) ?? false;
  const has_name_conflict = !!imported_sources && !already_imported;
  const external_key = `${item.package_spec}@@${item.skill_slug}`;
  const is_busy = busy_external_key === external_key;
  const state_label = already_imported ? "已导入" : has_name_conflict ? "同名冲突" : "可导入";
  const state_tone = already_imported ? "success" : has_name_conflict ? "warning" : "neutral";

  return (
    <UiListRow
      class_name="min-h-[72px] rounded-[14px] px-2 py-1.5"
      leading={(
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_9%,var(--surface-panel-background))] text-sky-600 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          <Puzzle className="h-4 w-4" />
        </span>
      )}
      on_click={on_preview}
      right={(
        <div className="flex shrink-0 items-center gap-1.5">
          <SkillStatePill tone={state_tone}>
            {state_label}
          </SkillStatePill>
          {!already_imported && !has_name_conflict ? (
            <UiListActionButton
              class_name="text-(--primary) hover:text-(--primary)"
              disabled={is_busy || has_name_conflict}
              onClick={on_import}
              size="sm"
              stop_propagation
              title="导入到技能库"
              visibility="visible"
            >
              {is_busy ? (
                <Loader2 className="h-3 w-3 animate-spin" />
              ) : (
                <Download className="h-3 w-3" />
              )}
            </UiListActionButton>
          ) : null}
        </div>
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            {item.title || item.skill_slug}
          </span>
          <UiBadge size="xs">社区</UiBadge>
        </div>
        <div className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
          {item.description || item.readme_markdown || "暂无描述"}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] leading-4 text-(--text-soft)">
          <span className="truncate">{item.package_spec}</span>
          <span className="shrink-0">·</span>
          <span className="shrink-0">{format_installs(item.installs)} 次安装</span>
        </div>
      </div>
    </UiListRow>
  );
}
