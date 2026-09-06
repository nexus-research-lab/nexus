// INPUT: 建议作用域、当前选择、必要工具参数与交互禁用事实。
// OUTPUT: 实例隔离的原生单选组、可读标签和关联的禁用原因。
// POS: ToolBlock 权限展示/选择层；实例身份只控制 DOM 分组，不参与请求授权或提交决定。

import { useId } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiRadioChoice } from "@/shared/ui/form/choice";
import { UiField } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { MessageDetailScroll } from "../../ui/message-rail";
import type {
  ToolBlockViewModel,
} from "./tool-block-types";

interface ToolBlockPermissionProps {
  interactionDisabled: boolean;
  interactionDisabledReason?: string;
  model: ToolBlockViewModel;
  onSelectedSuggestionIndexChange: (index: number) => void;
  selectedSuggestionIndex: number;
}

export function ToolBlockPermission({
  interactionDisabled,
  interactionDisabledReason,
  model,
  onSelectedSuggestionIndexChange,
  selectedSuggestionIndex,
}: ToolBlockPermissionProps) {
  const { t } = useI18n();
  const scopeName = useId();
  return (
    <div className="message-cjk-font ml-7 mt-2 space-y-2 border-t border-(--divider-subtle-color) pt-2">
      {model.primaryInputDetail?.value.trim() ? (
        <div className="space-y-1 px-0 py-0 text-compact leading-5 text-(--text-default)">
          <div className={getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" })}>
            {model.primaryInputDetail.label}
          </div>
          <MessageDetailScroll>
            <pre className="message-cjk-font whitespace-pre-wrap break-all text-compact leading-5 text-(--text-default)">
              {model.primaryInputDetail.value}
            </pre>
          </MessageDetailScroll>
        </div>
      ) : null}

      {model.readableSuggestions.length > 0 ? (
        <UiField
          label={t("message.tool_permission_scope")}
          description={interactionDisabled ? interactionDisabledReason : undefined}
        >
          <div className="flex flex-wrap items-center gap-1.5">
            <UiRadioChoice
              checked={selectedSuggestionIndex === -1}
              choiceSize="xs"
              disabled={interactionDisabled}
              name={scopeName}
              onChange={() => onSelectedSuggestionIndexChange(-1)}
            >
              {t("message.tool_permission_once")}
            </UiRadioChoice>
            {model.readableSuggestions.map((suggestion) => (
              <UiRadioChoice
                key={suggestion.index}
                checked={selectedSuggestionIndex === suggestion.index}
                choiceSize="xs"
                disabled={interactionDisabled}
                name={scopeName}
                onChange={() => onSelectedSuggestionIndexChange(suggestion.index)}
              >
                {suggestion.label}
              </UiRadioChoice>
            ))}
          </div>
        </UiField>
      ) : null}
      {model.readableSuggestions.length === 0 && interactionDisabled && interactionDisabledReason ? (
        <div className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
          {interactionDisabledReason}
        </div>
      ) : null}
    </div>
  );
}
