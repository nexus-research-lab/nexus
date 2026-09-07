// INPUT: 当前字符数/限制和历史导航位置。
// OUTPUT: 共享 caption 数字排版与领域计数色调；没有数据时不占底栏间距。
// POS: Composer Footer 辅助元数据；字符限制与历史行为由控制器拥有。

import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiToneClassName, getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { getCharacterCountTone } from "./composer-footer-model";

export function ComposerFooterMetadata({
  charCount,
  historyIndex,
  inputHistoryLength,
  isNearLimit,
  isOverLimit,
  maxLength,
}: {
  charCount: number;
  historyIndex: number;
  inputHistoryLength: number;
  isNearLimit: boolean;
  isOverLimit: boolean;
  maxLength: number;
}) {
  if (charCount === 0 && historyIndex < 0) return null;

  return (
    <div className={`nexus-chat-composer-footer-metadata flex items-center gap-3 tabular-nums ${getUiTypographyClassName({ role: "caption" })}`}>
      <ComposerCharacterCount
        charCount={charCount}
        isNearLimit={isNearLimit}
        isOverLimit={isOverLimit}
        maxLength={maxLength}
      />
      <ComposerHistoryPosition
        historyIndex={historyIndex}
        inputHistoryLength={inputHistoryLength}
      />
    </div>
  );
}

function ComposerCharacterCount({
  charCount,
  isNearLimit,
  isOverLimit,
  maxLength,
}: {
  charCount: number;
  isNearLimit: boolean;
  isOverLimit: boolean;
  maxLength: number;
}) {
  if (charCount === 0) {
    return null;
  }
  return (
    <div>
      <span className={getUiToneClassName(getCharacterCountTone({ isNearLimit, isOverLimit }))}>
        {charCount}
      </span>
      <span className="text-(--text-soft)">/{maxLength}</span>
    </div>
  );
}

function ComposerHistoryPosition({
  historyIndex,
  inputHistoryLength,
}: {
  historyIndex: number;
  inputHistoryLength: number;
}) {
  const { t } = useI18n();
  if (historyIndex < 0) {
    return null;
  }
  return (
    <div className="text-(--text-default)">
      {t("composer.history_position", {
        current: historyIndex + 1,
        total: inputHistoryLength,
      })}
    </div>
  );
}
