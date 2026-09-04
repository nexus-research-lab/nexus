import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiToneClassName } from "@/shared/ui/typography/typography-styles";

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
  return (
    <div className="nexus-chat-composer-footer-metadata flex items-center gap-3 text-2xs tabular-nums">
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
    <div className="text-2xs text-(--text-default)">
      {t("composer.history_position", {
        current: historyIndex + 1,
        total: inputHistoryLength,
      })}
    </div>
  );
}
