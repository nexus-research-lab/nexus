import { useI18n } from "@/shared/i18n/i18n-context";

import { RoomSkillMultiSelect } from "./room-skill-multi-select";
import type { RoomSkillOption } from "./room-skill-multi-select-model";

interface RoomSkillsSelectorProps {
  disabled: boolean;
  error: string | null;
  isLoading: boolean;
  onChange: (names: string[]) => void;
  onQueryChange: (query: string) => void;
  options: RoomSkillOption[];
  query: string;
  value: string[];
}

export function RoomSkillsSelector({
  disabled,
  error,
  isLoading,
  onChange,
  onQueryChange,
  options,
  query,
  value,
}: RoomSkillsSelectorProps) {
  const { t } = useI18n();
  const label = t("room.skills_label");
  return (
    <div className="shrink-0 space-y-2">
      <p className="dialog-label">{label}</p>
      <RoomSkillMultiSelect
        ariaLabel={label}
        disabled={disabled}
        emptyText={t("room.skills_empty")}
        errorText={error}
        isLoading={isLoading}
        loadingText={t("room.skills_loading")}
        onChange={onChange}
        onQueryChange={onQueryChange}
        options={options}
        placeholder={t("room.skills_none")}
        query={query}
        searchPlaceholder={t("agent_options.skills.search_placeholder")}
        value={value}
      />
    </div>
  );
}
