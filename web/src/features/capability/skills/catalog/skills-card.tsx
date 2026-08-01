"use client";

import { Trash2 } from "lucide-react";

import { getSkillDisplayDescription } from "@/lib/skill-description";
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
      action={(
        <>
          <UiBadge size="xs" tone={model.stateTone}>{model.stateLabel}</UiBadge>
          {model.showDelete ? (
            <UiListActionButton
              className="pointer-events-auto"
              disabled={busy}
              onClick={onDelete}
              size="sm"
              stopPropagation
              title="从技能库删除"
              tone="danger"
            >
              <Trash2 className="h-3 w-3" />
            </UiListActionButton>
          ) : null}
        </>
      )}
      badges={model.showUpdate ? <UiBadge size="xs" tone="warning">有更新</UiBadge> : null}
      busy={busy}
      description={model.description}
      meta={(
        <>
          <span className="shrink-0">{model.sourceLabel}</span>
          {model.usageLabel ? (
            <span className="shrink-0">· {model.usageLabel}</span>
          ) : null}
          {model.visibleTags.map((tag) => (
            <span className="truncate" key={tag}>
              · {tag}
            </span>
          ))}
        </>
      )}
      onSelect={onSelect}
      seed={skill.name}
      title={model.title}
    />
  );
}
