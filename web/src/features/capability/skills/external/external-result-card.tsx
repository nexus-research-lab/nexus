import { Download, Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import type { ExternalSkillSearchItem } from "@/types/capability/skill";

import { SkillDirectoryCard } from "../shared/skill-directory-card";
import {
  buildExternalSkillListItemModel,
  type ExternalSkillImportModel,
} from "./external-skill-model";

interface ExternalResultCardProps {
  busyExternalKeys: ReadonlySet<string>;
  importedExternalSources: Map<string, Set<string>>;
  item: ExternalSkillSearchItem;
  onImport: () => void;
  onPreview: () => void;
}

export function ExternalResultCard({
  busyExternalKeys,
  importedExternalSources,
  item,
  onImport,
  onPreview,
}: ExternalResultCardProps) {
  const { t } = useI18n();
  const model = buildExternalSkillListItemModel(
    item,
    importedExternalSources,
    busyExternalKeys,
    { t },
  );

  return (
    <SkillDirectoryCard
      action={(
        <ExternalResultActions
          importState={model.importState}
          onImport={onImport}
          title={t("capability.skills_external_import_action")}
        />
      )}
      badges={<UiBadge size="xs">{model.sourceLabel}</UiBadge>}
      busy={model.importState.busy}
      description={model.description}
      meta={(
        <>
          <span className="truncate">{model.sourceReference}</span>
          {model.installLabel ? (
            <span className="shrink-0">· {model.installLabel}</span>
          ) : null}
        </>
      )}
      onSelect={onPreview}
      seed={model.avatarSeed}
      title={model.title}
    />
  );
}

interface ExternalResultActionsProps {
  importState: ExternalSkillImportModel;
  onImport: () => void;
  title: string;
}

function ExternalResultActions({
  importState,
  onImport,
  title,
}: ExternalResultActionsProps) {
  return (
    <>
      <UiBadge size="xs" tone={importState.tone}>
        {importState.label}
      </UiBadge>
      {importState.canImport ? (
        <UiListActionButton
          aria-busy={importState.busy || undefined}
          className="pointer-events-auto"
          disabled={importState.busy}
          onClick={onImport}
          size="sm"
          stopPropagation
          tone="primary"
          title={title}
          visibility="visible"
        >
          {importState.busy ? (
            <Loader2 className={getUiSpinnerClassName({ size: "xs" })} />
          ) : (
            <Download className="h-3 w-3" />
          )}
        </UiListActionButton>
      ) : null}
    </>
  );
}
