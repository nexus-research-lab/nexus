import { Trash2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import {
  get_provider_title,
  get_usage_agent_title,
} from "./provider-settings-model";

interface ProviderDeleteUsageDialogProps {
  delete_target_record: ProviderConfigRecord | null;
  is_open: boolean;
  on_cancel: () => void;
  on_force_delete: () => void;
  submitting: boolean;
}

export function ProviderDeleteUsageDialog({
  delete_target_record,
  is_open,
  on_cancel,
  on_force_delete,
  submitting,
}: ProviderDeleteUsageDialogProps) {
  const { t } = useI18n();

  if (!is_open || !delete_target_record) {
    return null;
  }

  const delete_usage_agents = delete_target_record.used_by_agents ?? [];

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9999]"
        labelled_by="provider-delete-blocked-title"
        on_close={on_cancel}
      >
        <UiDialogShell size="sm">
          <UiDialogHeader
            icon={<Trash2 className="h-4.5 w-4.5" />}
            on_close={on_cancel}
            subtitle={t("settings.providers.delete_usage_subtitle", { name: get_provider_title(delete_target_record) })}
            title={t("settings.providers.delete_usage_title")}
            title_id="provider-delete-blocked-title"
          />
          <UiDialogBody class_name="space-y-3">
            <div className="rounded-[12px] border border-(--divider-subtle-color) bg-(--surface-muted-background) px-3 py-2 text-[12px] leading-5 text-(--text-muted)">
              {t("settings.providers.force_delete_description")}
            </div>
            {delete_usage_agents.length > 0 ? (
              <div className="max-h-64 overflow-y-auto rounded-[12px] border border-(--divider-subtle-color)">
                {delete_usage_agents.map((agent) => (
                  <div
                    className="flex min-h-11 items-center gap-2 border-b border-(--divider-subtle-color) px-3 py-2 last:border-b-0"
                    key={agent.agent_id}
                  >
                    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[9px] border border-(--divider-subtle-color) bg-(--background) text-[11px] font-semibold text-(--text-muted)">
                      {(get_usage_agent_title(agent).slice(0, 2) || "AG").toUpperCase()}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 items-center gap-2">
                        <span className="truncate text-[13px] font-semibold text-(--text-strong)">
                          {get_usage_agent_title(agent)}
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
                {t("settings.providers.delete_usage_stale", { count: delete_target_record.usage_count })}
              </div>
            )}
          </UiDialogBody>
          <UiDialogFooter>
            <UiButton
              onClick={on_cancel}
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={submitting}
              onClick={on_force_delete}
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
