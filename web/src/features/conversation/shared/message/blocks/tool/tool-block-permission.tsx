// INPUT: 工具权限请求、建议作用域、当前选择与交互禁用事实。
// OUTPUT: 使用共享原生 Radio choice 的可访问权限范围选择区。
// POS: ToolBlock 权限展示/选择层；不提交权限决定或解释后端授权。

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiRadioChoice } from "@/shared/ui/form/choice";

import { MessageDetailScroll } from "../../ui/message-rail";
import type {
  ToolBlockViewModel,
  ToolPermissionRequest,
} from "./tool-block-types";

interface ToolBlockPermissionProps {
  interactionDisabled: boolean;
  interactionDisabledReason?: string;
  model: ToolBlockViewModel;
  onSelectedSuggestionIndexChange: (index: number) => void;
  permissionRequest: ToolPermissionRequest;
  selectedSuggestionIndex: number;
}

export function ToolBlockPermission({
  interactionDisabled,
  interactionDisabledReason,
  model,
  onSelectedSuggestionIndexChange,
  permissionRequest,
  selectedSuggestionIndex,
}: ToolBlockPermissionProps) {
  const { t } = useI18n();
  return (
    <div className="message-cjk-font ml-7 mt-2 space-y-2 border-t border-(--divider-subtle-color) pt-2">
      {model.primaryInputDetail?.value.trim() ? (
        <div className="space-y-1 px-0 py-0 text-compact leading-5 text-(--text-default)">
          <div className="text-2xs font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
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
        <div className="space-y-1">
          <div className="text-2xs font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
            {t("message.tool_permission_scope")}
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <PermissionChoice
              checked={selectedSuggestionIndex === -1}
              disabled={interactionDisabled}
              label={t("message.tool_permission_once")}
              name={`permission-suggestion-${permissionRequest.request_id}`}
              onSelect={() => onSelectedSuggestionIndexChange(-1)}
            />
            {model.readableSuggestions.map((suggestion) => (
              <PermissionChoice
                key={suggestion.index}
                checked={selectedSuggestionIndex === suggestion.index}
                disabled={interactionDisabled}
                label={suggestion.label}
                name={`permission-suggestion-${permissionRequest.request_id}`}
                onSelect={() => onSelectedSuggestionIndexChange(suggestion.index)}
              />
            ))}
          </div>
        </div>
      ) : null}
      {interactionDisabled && interactionDisabledReason ? (
        <div className="text-xs text-(--text-soft)">
          {interactionDisabledReason}
        </div>
      ) : null}
    </div>
  );
}

function PermissionChoice({
  checked,
  disabled,
  label,
  name,
  onSelect,
}: {
  checked: boolean;
  disabled: boolean;
  label: string;
  name: string;
  onSelect: () => void;
}) {
  return (
    <UiRadioChoice
      checked={checked}
      choiceSize="xs"
      disabled={disabled}
      name={name}
      onChange={() => onSelect()}
      variant="surface"
    >
      {label}
    </UiRadioChoice>
  );
}
