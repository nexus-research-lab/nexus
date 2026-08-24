"use client";

import { Trash2 } from "lucide-react";

import {
  getSkillDisplayDescription,
  getSkillDisplayTitle,
} from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import type { SkillInfo } from "@/types/capability/skill";

import { SkillDirectoryCard } from "../shared/skill-directory-card";
import { buildSkillCardModel } from "./skills-catalog-model";

interface SkillsCardProps {
  skill: SkillInfo;
  busy?: boolean;
  onSelect: () => void;
  onDelete?: () => void;
}

/** Skill 卡 —— 使用能力页统一的头像驱动目录结构。 */
export function SkillsCard({
  skill,
  busy = false,
  onSelect,
  onDelete,
}: SkillsCardProps) {
  const { t } = useI18n();
  const model = buildSkillCardModel(
    skill,
    getSkillDisplayDescription(skill, t),
  );
  return (
    <SkillDirectoryCard
      action={model.showDelete ? (
        <UiListActionButton
          className="pointer-events-auto"
          disabled={busy}
          onClick={onDelete}
          size="sm"
          stopPropagation
          title={t("capability.skills_remove")}
          tone="danger"
        >
          <Trash2 className="h-3 w-3" />
        </UiListActionButton>
      ) : undefined}
      badges={model.showUpdate ? (
        <UiBadge size="xs" tone="warning">
          {t("capability.skills_update_available")}
        </UiBadge>
      ) : null}
      busy={busy}
      description={model.description}
      onSelect={onSelect}
      seed={skill.name}
      title={getSkillDisplayTitle(skill, t)}
    />
  );
}
