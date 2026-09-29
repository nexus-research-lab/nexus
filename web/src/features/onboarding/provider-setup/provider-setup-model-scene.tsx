// INPUT: 已保存 Provider 的模型目录、选择草稿与恢复锁。
// OUTPUT: 显式模型选择、手填回退及验证提交。
// POS: 初始化向导模型选择视图；不保存凭据或发起网络请求。
import { modelGuidanceLabel, recommendedModelsFirst } from "@/entities/provider/model-guidance";
import type { ReactNode } from "react";
import type { ProviderModelRecord } from "@/types/capability/provider";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function ProviderSetupModelScene({ models, value, onChange, busy, locked, failed, children, onSubmit }: {
  models: ProviderModelRecord[];
  value: string;
  onChange: (value: string) => void;
  busy: boolean;
  locked: boolean;
  failed: boolean;
  children?: ReactNode;
  onSubmit: () => void;
}) {
  const { t } = useI18n();
  return <form className="flex min-h-0 flex-1 flex-col" onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
    <h3 className={getUiTypographyClassName({ role: "objectTitle", tone: "strong", weight: "semibold" })}>{t("onboarding.provider_setup_choose_model")}</h3>
    <div className="soft-scrollbar mt-5 min-h-0 flex-1 space-y-3 overflow-y-auto">
      {failed ? <UiInlineNotice tone="warning" message={t("onboarding.provider_setup_model_list_failed")} /> : null}
      {models.length > 0 ? <UiField label={t("onboarding.provider_setup_choose_model")} htmlFor="provider-setup-model-choice">
        <UiSelectMenu id="provider-setup-model-choice" ariaLabel={t("onboarding.provider_setup_choose_model")} surface="dialog" size="md"
          value={value} onChange={onChange} disabled={busy || locked}
          placeholder={t("onboarding.provider_setup_choose_model")}
          options={recommendedModelsFirst(models, "chat").map((model) => ({ value: model.model_id, label: [model.display_name === model.model_id ? model.model_id : `${model.display_name} (${model.model_id})`, modelGuidanceLabel(model.guidance, "chat", t)].filter(Boolean).join(" · "), disabled: model.guidance?.eligibility.chat.available === false }))} />
      </UiField> : null}
      <UiField label={t("onboarding.provider_setup_model")} htmlFor="provider-setup-selected-model" description={t("onboarding.provider_setup_model_manual")} required>
        <UiInput id="provider-setup-selected-model" value={value} onChange={(event) => onChange(event.target.value)}
          required readOnly={busy || locked} autoCapitalize="off" spellCheck={false} placeholder={t("onboarding.provider_setup_model_placeholder")} />
      </UiField>
      {children}
    </div>
    <div className="flex shrink-0 justify-end border-t border-(--divider-subtle-color) pb-5 pt-3">
      <UiButton type="submit" tone="primary" variant="solid" size="sm" disabled={busy || !value.trim()}>
        {locked ? t("onboarding.provider_setup_reconcile_action") : t("onboarding.provider_setup_model_confirm")}
      </UiButton>
    </div>
  </form>;
}
