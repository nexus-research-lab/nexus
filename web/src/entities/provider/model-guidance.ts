// INPUT: Server-owned model guidance and the current selection purpose.
// OUTPUT: Localized recommendation/capability labels without inferring capabilities.
// POS: Provider entity presentation shared by onboarding, settings and chat.
import type { ModelGuidance, ModelPurpose } from "@/types/capability/provider";
import type { useI18n } from "@/shared/i18n/i18n-context";

type Translate = ReturnType<typeof useI18n>["t"];
export function modelGuidanceLabel(guidance: ModelGuidance | undefined, purpose: ModelPurpose, t: Translate): string {
  if (!guidance) return "";
  const labels: string[] = [];
  if (guidance.recommendations[purpose]) labels.push(t("settings.providers.model_recommended"));
  const c = guidance.capabilities;
  if (c.vision) labels.push(t("settings.providers.model_multimodal"));
  else if (guidance.text_only) labels.push(t("settings.providers.model_text_only"));
  if (c.image_output) labels.push(t("settings.providers.capability_image_output"));
  if (c.image_editing) labels.push(t("settings.providers.capability_image_editing"));
  return labels.join(" · ");
}

export function recommendedModelsFirst<T extends { guidance?: ModelGuidance }>(models: T[], purpose: ModelPurpose): T[] {
  return [...models].sort((a, b) => Number(Boolean(b.guidance?.recommendations[purpose])) - Number(Boolean(a.guidance?.recommendations[purpose])));
}

export function modelGuidanceHint(guidance: ModelGuidance | undefined, purpose: ModelPurpose, t: Translate): string {
  if (!guidance) return "";
  const reason = guidance.recommendations[purpose];
  const reasons: Record<string, string> = {
    flagship: t("settings.providers.advice_flagship"),
    balanced: t("settings.providers.advice_balanced"),
    image_generation: t("settings.providers.advice_image"),
    image_editing: t("settings.providers.advice_edit"),
  };
  const notices: Record<string, string> = {
    coding_plan: t("settings.providers.advice_coding_plan"),
    glm_redirect: t("settings.providers.advice_glm_redirect"),
    kimi_standard: t("settings.providers.advice_kimi_standard"),
    kimi_k3: t("settings.providers.advice_kimi_k3"),
    kimi_highspeed: t("settings.providers.advice_kimi_highspeed"),
    minimax_plan: t("settings.providers.advice_minimax_plan"),
    qwen_team: t("settings.providers.advice_qwen_team"),
    region_access: t("settings.providers.advice_region_access"),
    volc_k3: t("settings.providers.advice_volc_k3"),
    volc_heavy: t("settings.providers.advice_volc_heavy"),
    modelscope_dynamic: t("settings.providers.advice_modelscope_dynamic"),
  };
  return [
    reason ? reasons[reason] || t("settings.providers.model_recommended_hint") : "",
    guidance.capabilities.vision ? t("settings.providers.model_multimodal_hint") : "",
    guidance.capabilities.image_editing && !guidance.eligibility.image_editing.available ? t("settings.providers.advice_edit_route") : "",
    guidance.evidence?.notice ? notices[guidance.evidence.notice] : "",
    guidance.evidence ? `${t("settings.providers.advice_reviewed")} ${guidance.evidence.reviewed_at}` : "",
  ].filter(Boolean).join(" ");
}
