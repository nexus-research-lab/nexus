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
  if (c.image_output) labels.push(t("settings.providers.capability_image_output"));
  if (c.image_editing) labels.push(t("settings.providers.capability_image_editing"));
  return labels.join(" · ");
}

export function recommendedModelsFirst<T extends { guidance?: ModelGuidance }>(models: T[], purpose: ModelPurpose): T[] {
  return [...models].sort((a, b) => Number(Boolean(b.guidance?.recommendations[purpose])) - Number(Boolean(a.guidance?.recommendations[purpose])));
}
