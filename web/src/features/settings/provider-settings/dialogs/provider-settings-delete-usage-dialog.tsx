// INPUT: 仍被 Agent 使用的 Provider、精确使用者和强制删除动作。
// OUTPUT: 目标、具体后果与使用者列表组成的 plain 高风险弹窗。
// POS: Provider 删除前的占用确认面，不展示内部 Agent ID 或装饰性危险卡。

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
            appearance="plain"
            onClose={onCancel}
            title={t("settings.providers.delete_usage_subtitle", { name: getProviderTitle(deleteTargetRecord) })}
            titleId="provider-delete-blocked-title"
          />
          <UiDialogBody className="space-y-3 px-5">
            <p className="text-sm leading-6 text-(--text-muted)">
              {t("settings.providers.force_delete_description")}
            </p>
            {deleteUsageAgents.length > 0 ? (
              <section className="space-y-1.5">
                <h3 className="text-xs font-medium text-(--text-muted)">
                  {t("settings.providers.used_by_agents")}
                </h3>
                <div className="max-h-64 divide-y divide-(--divider-subtle-color) overflow-y-auto border-y border-(--divider-subtle-color)">
                  {deleteUsageAgents.map((agent) => (
                    <div
                      className="flex min-h-10 items-center px-1 py-2"
                      key={agent.agent_id}
                    >
                      <div className="flex min-w-0 items-center gap-2">
                        <span className="truncate text-sm font-semibold text-(--text-strong)">
                          {getUsageAgentTitle(agent)}
                        </span>
                        {agent.is_main ? (
                          <span className="rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-2xs font-semibold text-(--text-muted)">
                            {t("settings.providers.main_agent_badge")}
                          </span>
                        ) : null}
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            ) : (
              <p className="text-compact leading-5 text-(--text-muted)">
                {t("settings.providers.delete_usage_stale", { count: deleteTargetRecord.usage_count })}
              </p>
            )}
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
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
