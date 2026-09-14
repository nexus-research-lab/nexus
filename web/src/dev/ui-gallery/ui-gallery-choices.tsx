// INPUT: Gallery locale, controlled choice values and two isolated permission views.
// OUTPUT: Real shared size/variant/disabled fixtures and instance-scoped permission keyboard targets.
// POS: Development-only UI evidence; callbacks update local state and never authorize tools.

import { useId, useState } from "react";
import { ToolBlockPermission } from "@/features/conversation/shared/message/blocks/tool/tool-block-permission";
import type { ToolBlockViewModel } from "@/features/conversation/shared/message/blocks/tool/tool-block-types";
import type { Locale } from "@/shared/i18n/messages";
import { UiCheckboxRow } from "@/shared/ui/form/checkbox-row";
import { UiChoiceButton, UiRadioChoice } from "@/shared/ui/form/choice";
import { galleryText } from "./ui-gallery-copy";

export function ChoiceGallery({ locale }: { locale: Locale }) {
  const [enabled, setEnabled] = useState(true);
  const [buttonValue, setButtonValue] = useState("surface");
  const [radioValue, setRadioValue] = useState("surface");
  const radioName = useId();
  return <div className="space-y-4" data-gallery-choices>
    <div className="flex flex-wrap items-center gap-2">
      {(["xs", "sm", "md", "lg"] as const).map((size) => <UiChoiceButton
        choiceSize={size} data-gallery-choice-size={size} key={size}
      >{galleryText(locale, "选项", "Option")} {size}</UiChoiceButton>)}
    </div>
    <UiCheckboxRow checked={enabled} density="compact" label={galleryText(locale, "允许修改选项", "Enable choice changes")} onChange={setEnabled} />
    <fieldset className="flex min-w-0 flex-wrap items-center gap-3" disabled={!enabled}>
      {(["surface", "picker", "calendar", "icon"] as const).map((variant) => <div className="flex items-center gap-2" key={variant}>
        <UiChoiceButton active={buttonValue === variant} aria-label={`Choice button ${variant}`} className={variant === "calendar" ? "w-8" : undefined} data-gallery-choice-button={variant} onClick={() => setButtonValue(variant)} variant={variant}>
          {variant === "icon" ? <span aria-hidden>✦</span> : variant === "surface" ? galleryText(locale, "常规选项", "Surface option") : "12"}
        </UiChoiceButton>
        <UiRadioChoice checked={radioValue === variant} aria-label={`Choice radio ${variant}`} className={variant === "calendar" ? "w-8" : undefined} name={radioName} onChange={() => setRadioValue(variant)} variant={variant}>
          {variant === "icon" ? <span aria-hidden>✦</span> : variant === "surface" ? galleryText(locale, "单选项", "Radio option") : "24"}
        </UiRadioChoice>
      </div>)}
    </fieldset>
    <div className="grid min-w-0 gap-4 sm:grid-cols-2" data-gallery-permission-instances>
      <PermissionInstance label="First permission view" locale={locale} />
      <PermissionInstance label="Second permission view" locale={locale} />
    </div>
  </div>;
}

function PermissionInstance({ label, locale }: { label: string; locale: Locale }) {
  const [selected, setSelected] = useState(3);
  const model: ToolBlockViewModel = {
    collapsedDetailText: null, durationText: "", expandedInputText: null, hasResult: false,
    liveStatusText: null, primaryInputDetail: null,
    readableSuggestions: [
      { index: 3, label: galleryText(locale, "当前会话", "Current session") },
      { index: 7, label: galleryText(locale, "当前工作区", "Current workspace") },
    ],
    status: "waiting_permission", statusText: "", statusTone: "waiting", toolTitle: "Shell",
    toolVisualKind: "terminal", waitingActionHint: "",
  };
  return <section aria-label={label}>
    <ToolBlockPermission interactionDisabled={false} model={model} onSelectedSuggestionIndexChange={setSelected} selectedSuggestionIndex={selected} />
  </section>;
}
