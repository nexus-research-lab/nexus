import { useMemo } from "react";

import { SkillsDirectory } from "@/features/capability/skills/skills-directory";
import { buildSkillsTour } from "@/features/capability/skills/skills-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";

/** Skills 页面 — 列表目录 + 路由详情页 */
export function SkillsPage() {
  const { t } = useI18n();
  const skillsTour = useMemo(() => buildSkillsTour(t), [t]);

  const { startCurrentTour: startCurrentTour } = usePageOnboardingTour({
    tour: skillsTour,
    autoStartDelayMs: 260,
  });

  return <SkillsDirectory onReplayTour={startCurrentTour} />;
}
