// INPUT: Room Skill 目录 API 与当前搜索词。
// OUTPUT: 保留用户表单选择的可筛选 Skill 选项和安全读取失败标记。
// POS: 创建/管理 Room 的 Skill 资源控制器；不展示底层 Skill 或传输异常。
import { useEffect, useMemo, useState } from "react";

import { getAvailableSkillsApi } from "@/lib/api/capability/skill-api";
import { getSkillDisplayDescription } from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { SkillInfo } from "@/types/capability/skill";

import type { RoomSkillOption } from "./room-skill-multi-select-model";

interface RoomSkillState {
  error: string | null;
  items: SkillInfo[];
  loading: boolean;
}

export function useRoomSkillOptions(query: string) {
  const { t } = useI18n();
  const [state, setState] = useState<RoomSkillState>({
    error: null,
    items: [],
    loading: true,
  });

  useEffect(() => {
    let active = true;
    getAvailableSkillsApi({ scope: "room" })
      .then((items) => {
        if (active) {
          setState({ error: null, items, loading: false });
        }
      })
      .catch(() => {
        if (active) {
          setState({
            error: t("room.skills_load_error"),
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      active = false;
    };
  }, [t]);

  const normalizedQuery = query.trim().toLowerCase();
  const options = useMemo<RoomSkillOption[]>(
    () => state.items
      .map((skill) => ({
        description: getSkillDisplayDescription(skill, t),
        skill,
      }))
      .filter(({ description, skill }) => (
        matchesSkill(skill, description, normalizedQuery)
      ))
      .map(({ description, skill }) => ({
        description: description || skill.title,
        label: skill.name,
        value: skill.name,
      })),
    [normalizedQuery, state.items, t],
  );
  return { ...state, options };
}

function matchesSkill(
  skill: SkillInfo,
  description: string,
  query: string,
): boolean {
  if (!query) {
    return true;
  }
  return [skill.name, skill.title, description].some((value) =>
    value.toLowerCase().includes(query),
  );
}
