// INPUT: 完整聊天候选、草稿目标与精确选择命令。
// OUTPUT: 按智能体/群聊分组的可搜索锚定浮层；正文布局保持不变，公共浮层材质/层级与 Tab 边界返回表单；加载失败可重试，未解析值保留提示。
// POS: 定时任务运行和接收选择器；不请求资源、不拼装服务端载荷。
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, UserRound, Users } from "lucide-react";
import { getTabbableElements } from "@/shared/lib/browser/focus-navigation";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { focusAfterAnchoredOverlayExit } from "@/shared/ui/overlay/overlay-focus-navigation";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { UiButton } from "@/shared/ui/button/button";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import { OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TaskDestinationOption, TaskFormDraft } from "../scheduled-task-dialog-types";
import { buildTaskDestinationPickerModel } from "./task-destination-picker-model";
import type { TaskBasicsActions, TaskBasicsData } from "./task-basics-model";

export function TaskDestinationPicker({ kind, data, form, actions }: {
  kind: "execution" | "delivery";
  data: TaskBasicsData;
  form: TaskFormDraft;
  actions: TaskBasicsActions;
}) {
  const { t } = useI18n();
  const selectedLabelId = useId();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");
  const [browsingGroup, setBrowsingGroup] = useState<string | null>(null);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const close = useCallback(() => {
    setOpen(false);
    setSearch("");
    setTypeFilter("all");
    setBrowsingGroup(null);
  }, []);
  const estimatePosition = useCallback((anchor: HTMLElement) => resolveUiAnchoredOverlayPosition({
    anchor, preset: "form-picker", placement: "auto",
  }), []);
  const { overlayId, overlayRef, overlayStyle, portalContainer } = useAnchoredOverlayLayer({
    anchorRef, disabled: false, estimatePosition, isOpen: open, onClose: close, captureEscape: true,
  });
  const isVisible = open && overlayStyle.visibility === "visible";
  useEffect(() => {
    const root = overlayRef.current;
    if (!isVisible || !root) return;
    root.querySelector<HTMLInputElement>("input")?.focus();
    const onTab = (event: KeyboardEvent) => {
      if (event.key !== "Tab" || event.defaultPrevented || isImeKeyboardEvent(event)) return;
      const elements = getTabbableElements(root);
      const boundary = event.shiftKey ? elements[0] : elements.at(-1);
      if (document.activeElement !== boundary) return;
      event.preventDefault();
      event.stopPropagation();
      close();
      anchorRef.current?.focus();
      focusAfterAnchoredOverlayExit([root], event.shiftKey);
    };
    root.addEventListener("keydown", onTab);
    return () => root.removeEventListener("keydown", onTab);
  }, [close, isVisible, overlayRef]);
  const execution = kind === "execution";
  const label = t(execution ? "capability.scheduled_run_in" : "capability.scheduled_dialog_delivery");
  const { currentValue, currentLabel, groups, activeGroup, visibleItems, hasMatches } = buildTaskDestinationPickerModel({
    execution, data, form, search, typeFilter, browsingGroup, t,
  });
  const resources = [data.agents, data.rooms, data.destinationStatus];
  const loading = resources.some((resource) => resource.loading);
  const failedResources = resources.filter((resource) => resource.error);
  const retryableResources = failedResources.filter((resource) => !resource.loading);
  const select = (option: TaskDestinationOption | null) => {
    if (execution && option) actions.selectExecution(option);
    else actions.selectDelivery(option);
    close();
    anchorRef.current?.focus();
  };
  return <>
    <UiButton ref={anchorRef} aria-label={label} aria-expanded={open} aria-haspopup="dialog"
      aria-controls={open ? overlayId : undefined} aria-describedby={selectedLabelId} className="w-full justify-between gap-4 text-left"
      variant="ghost" onClick={() => open ? close() : setOpen(true)}>
      <span className="shrink-0">{label}</span>
      <span className="flex min-w-0 items-center gap-2">
        <span id={selectedLabelId} className="truncate">{currentLabel}</span><ChevronDown aria-hidden="true" className="h-4 w-4 shrink-0" />
      </span>
    </UiButton>
    {open && portalContainer ? createPortal(
      <div ref={overlayRef} id={overlayId} role="dialog" aria-label={label}
        {...OPEN_OVERLAY_DATA_ATTRIBUTES} style={overlayStyle}
        className={`${OVERLAY_SURFACE_CLASS_NAME} fixed ui-layer-popover flex flex-col overflow-y-auto overscroll-contain p-2`}>
    <div className="flex shrink-0 items-center gap-2 border-b border-(--divider-subtle-color) pb-2">
      <UiSearchInput className="min-w-0 flex-1" aria-label={t("capability.scheduled_search_chats")}
        placeholder={t("capability.scheduled_search_chats")} value={search} onChange={setSearch} />
      <UiSelectMenu className="w-28 shrink-0" ariaLabel={t("capability.scheduled_target_filter")} value={typeFilter} onChange={setTypeFilter}
        options={[{value: "all", label: t("capability.scheduled_target_all")}, {value: "agent", label: t("capability.scheduled_target_agent")}, {value: "room", label: t("capability.scheduled_target_room")}]} />

    </div>
    {!execution ? <UiButton className="w-full shrink-0 justify-between" variant="ghost" size="sm" onClick={() => select(null)}>
      {t("capability.scheduled_dialog_reply_none")}{currentValue === "none" ? <Check aria-hidden="true" className="h-4 w-4" /> : null}
    </UiButton> : null}
    <div className="grid min-h-24 flex-1 grid-cols-[minmax(100px,35%)_minmax(0,1fr)] overflow-hidden" role="group" aria-label={label}>
      <div className="soft-scrollbar min-h-0 overflow-y-auto overscroll-contain border-r border-(--divider-subtle-color) py-2 pr-2" aria-label={t("capability.scheduled_objects")}>
        {[...groups].map(([key, items]) => <UiButton key={key} className="w-full justify-start text-left" variant="ghost" size="sm"
          aria-pressed={activeGroup === key} onClick={() => setBrowsingGroup(key)}>
          {items[0].targetType === "room" ? <Users aria-hidden="true" className="h-4 w-4 shrink-0" /> : <UserRound aria-hidden="true" className="h-4 w-4 shrink-0" />}
          <span className="truncate">{items[0].group}</span>
        </UiButton>)}
      </div>
      <div className="soft-scrollbar min-h-0 overflow-y-auto overscroll-contain py-2 pl-2">
        {visibleItems.map((option) => <UiButton key={option.value} className="w-full justify-between gap-2 text-left" variant="ghost" size="sm"
          disabled={option.disabled || (!execution && form.executionMode === "main")}
          onClick={() => select(option)} aria-pressed={currentValue === option.value}>
          <span className="min-w-0 truncate">{option.label}{option.badge ? ` · ${option.badge}` : ""}</span>
          {currentValue === option.value ? <Check aria-hidden="true" className="h-4 w-4 shrink-0" /> : null}
        </UiButton>)}
      </div>
    </div>
    <div className="shrink-0">
      {loading ? <p role="status" className={`p-3 ${getUiTypographyClassName({ role: "supporting", tone: "muted" })}`}>{t("common.loading")}</p> : null}
      {failedResources.length > 0 ? <div role="status" className="p-2">
        <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{t("capability.scheduled_dialog_resource_load_title")}</p>
        <UiButton size="sm" variant="text" disabled={retryableResources.length === 0} onClick={() => retryableResources.forEach((resource) => resource.retry())}>{t("state.retry")}</UiButton>
      </div> : null}
      {!hasMatches && !loading && failedResources.length === 0 ? <p className={`p-3 ${getUiTypographyClassName({ role: "supporting", tone: "muted" })}`}>{t("capability.scheduled_no_matching_chats")}</p> : null}
    </div>
      </div>, portalContainer) : null}
  </>;
}
