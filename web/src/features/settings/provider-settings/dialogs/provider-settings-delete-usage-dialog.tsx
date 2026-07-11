import { Trash2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import type { ProviderPendingAction } from "../actions/use-provider-command";
import {
  getProviderTitle,
  getUsageAgentTitle,
} from "../model/provider-config-model";

interface ProviderDeleteUsageDialogProps {
  deleteTargetRecord: ProviderConfigRecord | null;
  isOpen: boolean;
  onCancel: () => void;
  onForceDelete: () => void;
  pendingAction: ProviderPendingAction | null;
}

export function ProviderDeleteUsageDialog({
  deleteTargetRecord,
  isOpen,
  onCancel,
  onForceDelete,
  pendingAction,
}: ProviderDeleteUsageDialogProps) {
  const { t } = useI18n();

  if (!isOpen || !deleteTargetRecord) {
    return null;
  }

  const deleteUsageAgents = deleteTargetRecord.used_by_agents ?? [];

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        labelledBy="provider-delete-blocked-title"
        onClose={onCancel}
      >
        <UiDialogShell size="sm">
          <UiDialogHeader
            icon={<Trash2 className="h-4.5 w-4.5" />}
            onClose={onCancel}
            subtitle={t("settings.providers.delete_usage_subtitle", { name: getProviderTitle(deleteTargetRecord) })}
            title={t("settings.providers.delete_usage_title")}
            titleId="provider-delete-blocked-title"
          />
          <UiDialogBody className="space-y-3">
            <div className="rounded-[12px] border border-(--divider-subtle-color) bg-(--surface-muted-background) px-3 py-2 text-[12px] leading-5 text-(--text-muted)">
              {t("settings.providers.force_delete_description")}
            </div>
            {deleteUsageAgents.length > 0 ? (
              <div className="max-h-64 overflow-y-auto rounded-[12px] border border-(--divider-subtle-color)">
                {deleteUsageAgents.map((agent) => (
                  <div
                    className="flex min-h-11 items-center gap-2 border-b border-(--divider-subtle-color) px-3 py-2 last:border-b-0"
                    key={agent.agent_id}
                  >
                    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[9px] border border-(--divider-subtle-color) bg-(--background) text-[11px] font-semibold text-(--text-muted)">
                      {(getUsageAgentTitle(agent).slice(0, 2) || "AG").toUpperCase()}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 items-center gap-2">
                        <span className="truncate text-[13px] font-semibold text-(--text-strong)">
                          {getUsageAgentTitle(agent)}
                        </span>
                        {agent.is_main ? (
                          <span className="rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[10px] font-semibold text-(--text-muted)">
                            {t("settings.providers.main_agent_badge")}
                          </span>
                        ) : null}
                      </div>
                      <div className="truncate font-mono text-[11px] text-(--text-soft)">
                        {agent.agent_id}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-[12px] border border-(--divider-subtle-color) px-3 py-3 text-[12px] leading-5 text-(--text-muted)">
                {t("settings.providers.delete_usage_stale", { count: deleteTargetRecord.usage_count })}
              </div>
            )}
          </UiDialogBody>
          <UiDialogFooter>
            <UiButton
              onClick={onCancel}
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={pendingAction !== null}
              onClick={onForceDelete}
              tone="danger"
              type="button"
              variant="solid"
            >
              {t("settings.providers.force_delete")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
