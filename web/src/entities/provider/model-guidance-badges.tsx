// INPUT: Backend guidance and current purpose.
// OUTPUT: Compact localized model badges.
// POS: Provider entity view shared by settings and chat.
import type { ModelGuidance, ModelPurpose } from "@/types/capability/provider";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { modelGuidanceLabel } from "./model-guidance";
export function ModelGuidanceBadges({ guidance, purpose = "chat" }: { guidance?: ModelGuidance; purpose?: ModelPurpose }) {
  const { t } = useI18n();
  const label = modelGuidanceLabel(guidance, purpose, t);
  return label ? <UiBadge title={guidance?.recommendations[purpose] ? t("settings.providers.model_recommended_hint") : label} className="max-w-full truncate" size="xs" tone="default">{label}</UiBadge> : null;
}
