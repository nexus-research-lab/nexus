import { Download, Loader2, Puzzle } from "lucide-react";

import { UiBadge } from "@/shared/ui/display/badge";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import type { ExternalSkillSearchItem } from "@/types/capability/skill";

import { SkillStatePill } from "../catalog/skill-state-pill";
import {
  buildExternalSkillListItemModel,
  type ExternalSkillImportModel,
} from "./external-skill-model";

interface ExternalResultRowProps {
  busyExternalKeys: ReadonlySet<string>;
  importedExternalSources: Map<string, Set<string>>;
  item: ExternalSkillSearchItem;
  onImport: () => void;
  onPreview: () => void;
}

export function ExternalResultRow({
  busyExternalKeys,
  importedExternalSources,
  item,
  onImport,
  onPreview,
}: ExternalResultRowProps) {
  const model = buildExternalSkillListItemModel(
    item,
    importedExternalSources,
    busyExternalKeys,
  );

  return (
    <UiListRow
      className="min-h-[72px] rounded-[14px] px-2 py-1.5"
      leading={<ExternalResultIcon />}
      onClick={onPreview}
      right={(
        <ExternalResultActions
          importState={model.importState}
          onImport={onImport}
        />
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            {model.title}
          </span>
          <UiBadge size="xs">{model.sourceLabel}</UiBadge>
        </div>
        <div className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
          {model.description}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] leading-4 text-(--text-soft)">
          <span className="truncate">{model.sourceReference}</span>
          <span className="shrink-0">·</span>
          <span className="shrink-0">{model.installLabel}</span>
        </div>
      </div>
    </UiListRow>
  );
}

function ExternalResultIcon() {
  return (
    <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_9%,var(--surface-panel-background))] text-sky-600 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
      <Puzzle className="h-4 w-4" />
    </span>
  );
}

interface ExternalResultActionsProps {
  importState: ExternalSkillImportModel;
  onImport: () => void;
}

function ExternalResultActions({
  importState,
  onImport,
}: ExternalResultActionsProps) {
  return (
    <div className="flex shrink-0 items-center gap-1.5">
      <SkillStatePill tone={importState.tone}>
        {importState.label}
      </SkillStatePill>
      {importState.canImport ? (
        <UiListActionButton
          className="text-(--primary) hover:text-(--primary)"
          disabled={importState.busy}
          onClick={onImport}
          size="sm"
          stopPropagation
          title="导入到技能库"
          visibility="visible"
        >
          {importState.busy ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            <Download className="h-3 w-3" />
          )}
        </UiListActionButton>
      ) : null}
    </div>
  );
}
