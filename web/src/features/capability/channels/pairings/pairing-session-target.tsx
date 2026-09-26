/**
 * INPUT: 当前配对、Room 目录与显式目标切换。
 * OUTPUT: 独立 IM / 精确 Room 话题的选择草稿与版本化更新。
 * POS: 配对会话目标编辑；读取失败保留草稿，不自动选择或提交其他话题。
 */
import { useId, useRef, useState } from "react";
import type { PairingView, UpdatePairingPayload } from "@/lib/api/capability/channel-api";
import { getRoomContexts, listRooms } from "@/lib/api/conversation/room-resource-api";
import type { RoomAggregate, RoomContextAggregate } from "@/types/conversation/room";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiField } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function PairingSessionTarget({ item, busy, onUpdate }: {
  item: PairingView;
  busy: boolean;
  onUpdate: (item: PairingView, patch: UpdatePairingPayload) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const id = useId();
  const generation = useRef(0);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [rooms, setRooms] = useState<RoomAggregate[]>([]);
  const [contexts, setContexts] = useState<RoomContextAggregate[]>([]);
  const [roomId, setRoomId] = useState(item.session_target?.room_id ?? "");
  const [conversationId, setConversationId] = useState(item.session_target?.conversation_id ?? "");

  async function loadRooms() {
    const current = ++generation.current;
    setOpen(true); setLoading(true); setFailed(false);
    try {
      const result = await listRooms(1000);
      const topics = roomId ? await getRoomContexts(roomId) : [];
      if (current !== generation.current) return;
      setRooms(result.filter(({ room, members }) => room.room_type === "group"
        && room.private_messages_enabled
        && members.some((member) => member.member_agent_id === item.agent_id && !member.participation_paused)));
      setContexts(topics);
    } catch { if (current === generation.current) setFailed(true); }
    finally { if (current === generation.current) setLoading(false); }
  }

  async function selectRoom(value: string) {
    const current = ++generation.current;
    setRoomId(value); setConversationId(""); setContexts([]); setFailed(false);
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
  const roomOptions = [{ value: "", label: t("capability.pairing_target_independent") },
    ...rooms.map(({ room }) => ({ value: room.id, label: room.name || t("capability.pairing_target_room") }))];
  if (roomId && !rooms.some(({ room }) => room.id === roomId)) {
    roomOptions.push({ value: roomId, label: t("capability.pairing_target_unavailable") });
  }
  return <div className="space-y-2 px-3 pb-3">
    <UiButton type="button" size="sm" variant="ghost" disabled={busy || loading} onClick={() => open ? setOpen(false) : void loadRooms()}>
      {t("capability.pairing_target")} · {item.session_target?.room_id
        ? [item.session_target.room_name, item.session_target.conversation_title].filter(Boolean).join(" / ") || t("capability.pairing_target_room")
        : t("capability.pairing_target_independent")}
    </UiButton>
    {open ? <div className="space-y-3">
      <UiField htmlFor={`${id}-room`} label={t("capability.pairing_target")}>
        <UiSelectMenu id={`${id}-room`} ariaLabel={t("capability.pairing_target")} value={roomId} options={roomOptions} disabled={busy || loading} onChange={(value) => void selectRoom(value)} />
      </UiField>
      {roomId ? <UiField htmlFor={`${id}-topic`} label={t("capability.pairing_target_topic")}>
        <UiSelectMenu id={`${id}-topic`} ariaLabel={t("capability.pairing_target_topic")} value={conversationId}
          options={[{ value: "", label: t("capability.pairing_target_choose_topic") }, ...contexts.map(({ conversation }) => ({ value: conversation.id, label: conversation.title || t("capability.pairing_target_untitled") }))]}
          disabled={busy || loading} onChange={setConversationId} />
      </UiField> : null}
      <p className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>{t("capability.pairing_target_hint")}</p>
      {failed ? <UiButton type="button" size="sm" variant="ghost" disabled={busy || loading} onClick={() => void loadRooms()}>{t("capability.pairing_target_retry")}</UiButton> : null}
      <UiButton type="button" size="sm" disabled={busy || loading || failed || !valid || item.binding_version === undefined}
        onClick={() => void onUpdate(item, { session_target: { room_id: roomId, conversation_id: conversationId }, binding_version: item.binding_version })}>
        {t("capability.pairing_target_save")}
      </UiButton>
    </div> : null}
  </div>;
}
