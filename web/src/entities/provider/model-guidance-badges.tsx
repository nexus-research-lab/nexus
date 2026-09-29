// INPUT: Backend guidance and current purpose.
// OUTPUT: Compact localized model badges.
// POS: Provider entity view shared by settings and chat.
import type { ModelGuidance, ModelPurpose } from "@/types/capability/provider";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { modelGuidanceHint, modelGuidanceLabel } from "./model-guidance";
export function ModelGuidanceBadges({ guidance, purpose = "chat", recommendationOnly = false }: { guidance?: ModelGuidance; purpose?: ModelPurpose; recommendationOnly?: boolean }) {
  const { t } = useI18n();
  const label = recommendationOnly
    ? (guidance?.recommendations[purpose] ? t("settings.providers.model_recommended") : "")
    : modelGuidanceLabel(guidance, purpose, t);
  const hint = modelGuidanceHint(guidance, purpose, t);
  return label ? <UiBadge title={hint || label} className="max-w-full truncate" size="xs" tone="default">{label}</UiBadge> : null;
}
