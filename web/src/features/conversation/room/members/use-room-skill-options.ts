import { useEffect, useMemo, useState } from "react";

import { getAvailableSkillsApi } from "@/lib/api/capability/skill-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { SkillInfo } from "@/types/capability/skill";

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
      .catch((error: unknown) => {
        if (active) {
          setState({
            error:
              error instanceof Error
                ? error.message
                : t("room.skills_load_error"),
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
  const options = useMemo(
    () =>
      state.items
        .filter((skill) => matchesSkill(skill, normalizedQuery))
        .map((skill) => ({
          description: skill.description || skill.title,
          label: skill.name,
          value: skill.name,
        })),
    [normalizedQuery, state.items],
  );
  return { ...state, options };
}

function matchesSkill(skill: SkillInfo, query: string): boolean {
  if (!query) {
    return true;
  }
  return [skill.name, skill.title, skill.description].some((value) =>
    value.toLowerCase().includes(query),
  );
}
