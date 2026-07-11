import { getUiChoiceClassName } from "@/shared/ui/form/choice-styles";

import {
  FIELD_LABEL_MAP,
  TOOL_DETAIL_SCROLL_CLASS_NAME,
} from "./tool-block-model";
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
  return (
    <div className="message-cjk-font ml-7 mt-2 space-y-2 border-t border-(--divider-subtle-color) pt-2">
      {model.primaryInputDetail?.value.trim() ? (
        <div className="space-y-1 px-0 py-0 text-[12px] leading-5 text-(--text-default)">
          <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
            {FIELD_LABEL_MAP[model.primaryInputDetail.key] || model.primaryInputDetail.key}
          </div>
          <div className={TOOL_DETAIL_SCROLL_CLASS_NAME}>
            <pre className="message-cjk-font whitespace-pre-wrap break-all text-[12px] leading-5 text-(--text-default)">
              {model.primaryInputDetail.value}
            </pre>
          </div>
        </div>
      ) : null}

      {model.readableSuggestions.length > 0 ? (
        <div className="space-y-1">
          <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
            权限范围
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <PermissionChoice
              checked={selectedSuggestionIndex === -1}
              disabled={interactionDisabled}
              label="仅这次"
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
        <div className="text-[11px] text-(--text-soft)">
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
    <label className={getUiChoiceClassName({
      active: checked,
      size: "xs",
      variant: "surface",
    })}>
      <input
        type="radio"
        name={name}
        checked={checked}
        disabled={disabled}
        onChange={onSelect}
        className="sr-only"
      />
      <span>{label}</span>
    </label>
  );
}
