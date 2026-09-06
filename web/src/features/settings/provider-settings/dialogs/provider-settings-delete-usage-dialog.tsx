// INPUT: 仍被 Agent 使用的 Provider、精确使用者和强制删除动作。
// OUTPUT: 目标、后果与完整使用者名称组成的确认弹窗，正文拥有唯一滚动区。
// POS: Provider 删除前的占用确认面，不展示内部 Agent ID 或装饰性危险卡。

import { useId } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiListRow } from "@/shared/ui/list/list-row";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
  const dialogId = useId();

  if (!isOpen || !deleteTargetRecord) {
    return null;
  }

  const deleteUsageAgents = deleteTargetRecord.used_by_agents ?? [];

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialog"
        labelledBy={`${dialogId}-title`}
        onClose={onCancel}
      >
        <UiDialogShell size="sm" viewport="adaptiveMax">
          <UiDialogHeader
            appearance="plain"
            onClose={onCancel}
            title={t("settings.providers.delete_usage_subtitle", { name: getProviderTitle(deleteTargetRecord) })}
            titleId={`${dialogId}-title`}
          />
          <UiDialogBody className="space-y-3 px-5" scrollable>
            <p className={getUiTypographyClassName({ role: "body", tone: "muted" })}>
              {t("settings.providers.force_delete_description")}
            </p>
            {deleteUsageAgents.length > 0 ? (
              <section className="space-y-1.5">
                <h3 className={getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" })}>
                  {t("settings.providers.used_by_agents")}
                </h3>
                <div className="divide-y divide-(--divider-subtle-color) border-y border-(--divider-subtle-color)">
                  {deleteUsageAgents.map((agent) => (
                    <UiListRow
                      density="dense"
                      key={agent.agent_id}
                      variant="flush"
                    >
                      <span className={cn("min-w-0 flex-1 break-words", getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" }))}>
                        {getUsageAgentTitle(agent, t("settings.providers.agent_name_unavailable"))}
                      </span>
                      {agent.is_main ? (
                        <UiBadge size="xs" tone="idle">
                          {t("settings.providers.main_agent_badge")}
                        </UiBadge>
                      ) : null}
                    </UiListRow>
                  ))}
                </div>
              </section>
            ) : (
              <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
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
              aria-busy={pendingAction?.kind === "delete-provider"}
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
