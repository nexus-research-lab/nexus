import { useI18n } from "@/shared/i18n/i18n-context";
import { UiMultiSelectMenu } from "@/shared/ui/select-menu";

interface RoomSkillsSelectorProps {
  disabled: boolean;
  error: string | null;
  isLoading: boolean;
  onChange: (names: string[]) => void;
  onQueryChange: (query: string) => void;
  options: Array<{
    description: string;
    label: string;
    value: string;
  }>;
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
  return (
    <div className="shrink-0 space-y-2">
      <p className="dialog-label">{t("room.skills_label")}</p>
      <UiMultiSelectMenu
        ariaLabel={t("room.skills_label")}
        disabled={disabled}
        emptyText={t("room.skills_empty")}
        errorText={error}
        isLoading={isLoading}
        loadingText={t("room.skills_loading")}
        onChange={onChange}
        onQueryChange={onQueryChange}
        options={options}
        placement="top"
        placeholder={t("room.skills_none")}
        query={query}
        searchPlaceholder={t("agent_options.skills.search_placeholder")}
        surface="dialog"
        value={value}
      />
    </div>
  );
}
