// INPUT: Room Skill 表单选择、目录读取状态和现有草稿值。
// OUTPUT: 不清空 Room 草稿并说明目录读取失败影响与恢复路径的选择器。
// POS: Room 创建/管理表单纯视图；不修改 Room，也不解释底层异常。
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
        errorText={error ? t("room.skills_load_error") : null}
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
      {error ? (
        <div
          aria-atomic="true"
          aria-live="polite"
          className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] px-3 py-2.5 text-xs"
          role="status"
        >
          <p className="font-semibold leading-5 text-(--destructive)">
            {t("room.skills_load_error")}
          </p>
          <p className="mt-1 leading-5 text-(--text-muted)">
            {t("room.skills_load_error_impact")}
          </p>
        </div>
      ) : null}
    </div>
  );
}
