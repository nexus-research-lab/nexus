import { useMemo } from "react";

import { SkillsDirectory } from "@/features/capability/skills/skills-directory";
import { build_skills_tour } from "@/features/capability/skills/skills-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";

/** Skills 页面 — 卡片网格 + 弹窗详情（不再使用路由切换详情页） */
export function SkillsPage() {
  const { t } = useI18n();
  const skills_tour = useMemo(() => build_skills_tour(t), [t]);

  const { start_current_tour } = usePageOnboardingTour({
    tour: skills_tour,
    auto_start_delay_ms: 260,
  });

  return <SkillsDirectory on_replay_tour={start_current_tour} />;
}
