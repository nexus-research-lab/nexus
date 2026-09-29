/**
 * INPUT: 当前配对、Room 目录与显式目标切换。
 * OUTPUT: 独立 IM / 精确 Room 话题的选择草稿与版本化更新。
 * POS: 配对会话目标编辑；读取失败保留草稿，不自动选择或提交其他话题。
 */
import { Check, ChevronDown, MessageCircle, UserRound, Users } from "lucide-react";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import { OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";
import { getTabbableElements } from "@/shared/lib/browser/focus-navigation";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { focusAfterAnchoredOverlayExit } from "@/shared/ui/overlay/overlay-focus-navigation";
import type { PairingView, UpdatePairingPayload } from "@/lib/api/capability/channel-api";
import { getRoomContexts, listRooms } from "@/lib/api/conversation/room-resource-api";
import type { RoomAggregate, RoomContextAggregate } from "@/types/conversation/room";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiField, UiSearchInput } from "@/shared/ui/form/form-control";
import { matchesUiSearchFields } from "@/shared/ui/form/search-query";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function PairingSessionTarget({ item, busy, onUpdate }: {
  item: PairingView;
  busy: boolean;
  onUpdate: (item: PairingView, patch: UpdatePairingPayload) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const id = useId();
  const generation = useRef(0);
  const [search, setSearch] = useState("");
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [rooms, setRooms] = useState<RoomAggregate[]>([]);
  const [contexts, setContexts] = useState<RoomContextAggregate[]>([]);
  const [roomId, setRoomId] = useState(item.session_target?.room_id ?? "");
  const [conversationId, setConversationId] = useState(item.session_target?.conversation_id ?? "");

  const anchorRef = useRef<HTMLButtonElement>(null);
  const close = useCallback(() => setOpen(false), []);
  const estimatePosition = useCallback((anchor: HTMLElement) => resolveUiAnchoredOverlayPosition({
    anchor, preset: "form-picker", placement: "auto",
  }), []);
  const { overlayId, overlayRef, overlayStyle, portalContainer } = useAnchoredOverlayLayer({
    anchorRef, disabled: false, estimatePosition, isOpen: open, onClose: close, captureEscape: true,
  });
  const visible = open && overlayStyle.visibility === "visible";
  useEffect(() => {
    const root = overlayRef.current;
    if (!visible || !root) return;
    root.querySelector<HTMLInputElement>("input")?.focus();
    const onTab = (event: KeyboardEvent) => {
      if (event.key !== "Tab" || event.defaultPrevented || isImeKeyboardEvent(event)) return;
      const elements = getTabbableElements(root);
      if (document.activeElement !== (event.shiftKey ? elements[0] : elements.at(-1))) return;
      event.preventDefault();
      close();
      anchorRef.current?.focus();
      focusAfterAnchoredOverlayExit([root], event.shiftKey);
    };
    root.addEventListener("keydown", onTab);
    return () => root.removeEventListener("keydown", onTab);
  }, [close, visible, overlayRef]);

  async function loadRooms() {
    const current = ++generation.current;
    setOpen(true); setLoading(true); setFailed(false);
    try {
      const result = await listRooms(1000);
      const topics = roomId ? await getRoomContexts(roomId) : [];
      if (current !== generation.current) return;
      setRooms(result.filter(({ room, members }) => (room.room_type === "dm" || (room.room_type === "room" && room.private_messages_enabled))
        && members.some((member) => member.member_agent_id === item.agent_id && !member.participation_paused)));
      setContexts(topics);
    } catch { if (current === generation.current) setFailed(true); }
    finally { if (current === generation.current) setLoading(false); }
  }

  async function selectRoom(value: string) {
    const current = ++generation.current;
    setSearch(""); setRoomId(value); setConversationId(""); setContexts([]); setFailed(false);
    if (!value) { setLoading(false); return; }
    setLoading(true);
    try {
      const result = await getRoomContexts(value);
      if (current === generation.current) setContexts(result);
    } catch { if (current === generation.current) setFailed(true); }
    finally { if (current === generation.current) setLoading(false); }
  }

  if (item.chat_type !== "dm") return null;
  const valid = !roomId || (rooms.some(({ room }) => room.id === roomId)
    && contexts.some(({ conversation }) => conversation.id === conversationId));
  const selectedRoom = rooms.find(({ room }) => room.id === roomId);
  const selectedConversation = contexts.find(({ conversation }) => conversation.id === conversationId);
  const draftLabel = roomId
    ? [selectedRoom?.room.name || t("capability.pairing_target_unavailable"), selectedConversation?.conversation.title || t("capability.pairing_target_choose_topic")].join(" / ")
    : t("capability.pairing_target_independent");
  const visibleRooms = rooms.filter(({ room }) => matchesUiSearchFields(search, [room.name]));
  const visibleContexts = contexts.filter(({ conversation }) => matchesUiSearchFields(search, [selectedRoom?.room.name, conversation.title]));
  const targetLabel = item.session_target?.room_id
    ? [item.session_target.room_name, item.session_target.conversation_title].filter(Boolean).join(" / ") || t("capability.pairing_target_room")
    : t("capability.pairing_target_independent");
  return <div className="min-w-0 space-y-3">
    <UiField htmlFor={`${id}-toggle`} label={t("capability.pairing_target")}>
      <UiButton ref={anchorRef} id={`${id}-toggle`} className="w-full justify-between" type="button" size="sm" variant="surface"
        aria-label={`${t("capability.pairing_target")} · ${targetLabel}`} aria-expanded={open} aria-controls={open ? overlayId : undefined} aria-haspopup="dialog"
        disabled={busy} onClick={() => open ? setOpen(false) : void loadRooms()}>
        <span className="min-w-0 flex-1 truncate text-center">{targetLabel}</span>
        <ChevronDown aria-hidden="true" className={`h-4 w-4 shrink-0 text-(--text-muted) ${open ? "rotate-180" : ""}`} />
      </UiButton>
    </UiField>
    {open && portalContainer ? createPortal(<div ref={overlayRef} id={overlayId} role="dialog" aria-label={t("capability.pairing_target")}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES} style={overlayStyle}
      className={`${OVERLAY_SURFACE_CLASS_NAME} fixed ui-layer-popover flex h-80 flex-col gap-2 overflow-hidden p-2`}>
      <UiSearchInput className="w-full shrink-0" aria-label={t("capability.pairing_target_search")} placeholder={t("capability.pairing_target_search")} value={search} onChange={setSearch} />
      <div className="grid min-h-0 flex-1 grid-cols-2 divide-x divide-(--divider-subtle-color) border-y border-(--divider-subtle-color) py-2">
        <div className="flex min-h-0 min-w-0 flex-col pr-2" role="group" aria-label={t("capability.pairing_target_chats")}>
          <p className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>{t("capability.pairing_target_chats")}</p>
          <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-contain">
      <UiButton type="button" size="sm" variant="ghost" className="w-full shrink-0 justify-between" disabled={busy || loading}
        aria-pressed={!roomId} onClick={() => void selectRoom("")}>
        <span className="flex items-center gap-2"><MessageCircle aria-hidden="true" className="h-4 w-4" />{t("capability.pairing_target_independent")}</span>
        {!roomId ? <Check aria-hidden="true" className="h-4 w-4" /> : null}
      </UiButton>

            {visibleRooms.map(({ room }) => <UiButton key={room.id} type="button" variant="ghost" size="sm" className="w-full justify-start text-left"
              disabled={busy || loading} aria-pressed={roomId === room.id} onClick={() => void selectRoom(room.id)}>
              {room.room_type === "dm" ? <UserRound aria-hidden="true" className="h-4 w-4 shrink-0" /> : <Users aria-hidden="true" className="h-4 w-4 shrink-0" />}<span className="truncate">{room.room_type === "dm" ? `${item.agent_name || room.name} · ${t("capability.pairing_target_dm")}` : room.name || t("capability.pairing_target_room")}</span>
            </UiButton>)}
          </div>
        </div>
        <div className="flex min-h-0 min-w-0 flex-col pl-2" role="group" aria-label={t("capability.pairing_target_topic")}>
          <p className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>{t("capability.pairing_target_topic")}</p>
          <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-contain">
            {visibleContexts.map(({ conversation }) => <UiButton key={conversation.id} type="button" variant="ghost" size="sm" className="w-full justify-between text-left"
              disabled={busy || loading} aria-pressed={conversationId === conversation.id} onClick={() => setConversationId(conversation.id)}>
              <span className="truncate">{conversation.title || t("capability.pairing_target_untitled")}</span>
              {conversationId === conversation.id ? <Check aria-hidden="true" className="h-4 w-4 shrink-0" /> : null}
            </UiButton>)}
            {!roomId ? <p className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>{t("capability.pairing_target_select_room")}</p> : null}
          </div>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
      <div className="flex h-8 min-w-0 flex-1 items-center overflow-hidden" role="status">
        {failed ? <UiButton type="button" size="sm" variant="ghost" disabled={busy || loading} onClick={() => void loadRooms()}>{t("capability.pairing_target_retry")}</UiButton>
          : <p className={`truncate ${getUiTypographyClassName({ role: "metadata", tone: "muted" })}`} title={draftLabel}>
            {loading ? t("common.loading") : rooms.length === 0 ? t("capability.pairing_target_empty")
              : visibleRooms.length === 0 && visibleContexts.length === 0 ? t("capability.scheduled_no_matching_chats")
                : `${t("capability.pairing_target_selected")} ${draftLabel}`}
          </p>}
      </div>

      <UiButton className="shrink-0" type="button" size="sm" variant="ghost" onClick={close}>{t("common.cancel")}</UiButton>
      <UiButton className="shrink-0" type="button" size="sm" disabled={busy || loading || failed || !valid || item.binding_version === undefined}
        onClick={() => void onUpdate(item, { session_target: { room_id: roomId, conversation_id: conversationId }, binding_version: item.binding_version })}>
        {t("common.save")}
      </UiButton>
      </div>
    </div>, portalContainer) : null}
  </div>;
}
