"use client";

import { Loader2, Plus, ShieldCheck } from "lucide-react";
import {
  type FormEvent,
  useEffect,
  useRef,
  useState,
} from "react";

import {
  createPairingApi,
  type ImChannelType,
  type ImChatType,
  type ImPairingStatus,
  type PairingView,
} from "@/lib/api/channel-api";
import { UiButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { Agent } from "@/types/agent/agent";

import {
  buildCreatePairingPayload,
  createPairingDraft,
  type CreatePairingDraft,
} from "./pairing-model";
import {
  CHANNEL_OPTIONS,
  CHAT_TYPE_OPTIONS,
  CREATE_PAIRING_STATUS_OPTIONS,
} from "./pairing-options";

interface CreatePairingDialogProps {
  agents: Agent[];
  onClose: () => void;
  onCreated: (item: PairingView) => void;
  onError: (message: string) => void;
}

export function CreatePairingDialog({
  agents,
  onClose,
  onCreated,
  onError,
}: CreatePairingDialogProps) {
  const savingRef = useRef(false);
  const [draft, setDraft] = useState(() => createPairingDraft(
    agents[0]?.agent_id || "",
  ));
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (draft.agentId && agents.some(
      (agent) => agent.agent_id === draft.agentId,
    )) {
      return;
    }
    setDraft((current) => ({
      ...current,
      agentId: agents[0]?.agent_id || "",
    }));
  }, [agents, draft.agentId]);

  const setField = <Key extends keyof CreatePairingDraft>(
    key: Key,
    value: CreatePairingDraft[Key],
  ) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const payload = buildCreatePairingPayload(draft);
    if (!payload || savingRef.current) {
      return;
    }
    savingRef.current = true;
    setSaving(true);
    try {
      onCreated(await createPairingApi(payload));
      onClose();
    } catch (error) {
      onError(error instanceof Error ? error.message : "新增配对失败");
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        labelledBy="create-pairing-dialog-title"
        onClose={onClose}
      >
        <UiDialogFormShell
          className="max-h-[86vh]"
          onSubmit={handleSubmit}
          size="lg"
        >
          <UiDialogHeader
            icon={<ShieldCheck className="h-5 w-5" />}
            onClose={onClose}
            subtitle="为已知外部用户、群或话题预先建立 IM 授权关系；只有渠道、会话类型、外部 ID 和 Thread 都相同时才会更新已有配对。"
            title="新增 IM 配对"
            titleId="create-pairing-dialog-title"
          />

          <UiDialogBody className="space-y-4" scrollable>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField label="渠道">
                <UiSelectMenu
                  ariaLabel="选择 IM 渠道"
                  onChange={(value) => setField(
                    "channelType",
                    value as ImChannelType,
                  )}
                  options={CHANNEL_OPTIONS}
                  size="sm"
                  value={draft.channelType}
                />
              </UiField>
              <UiField label="会话类型">
                <UiSelectMenu
                  ariaLabel="选择会话类型"
                  onChange={(value) => setField(
                    "chatType",
                    value as ImChatType,
                  )}
                  options={CHAT_TYPE_OPTIONS}
                  size="sm"
                  value={draft.chatType}
                />
              </UiField>
            </div>

            <UiField
              description="同一智能体可以绑定多个不同外部对象，每个对象会生成独立 IM session。"
              label={<>外部对象 ID <span className="text-(--destructive)">*</span></>}
            >
              <UiInput
                onChange={(event) => setField("externalRef", event.target.value)}
                placeholder={draft.chatType === "group"
                  ? "群 ID / chat_id / channel_id"
                  : "用户 ID / open_id / chat_id"}
                required
                value={draft.externalRef}
                variant="dialog"
              />
            </UiField>

            <UiField
              description="可选；多扫码账号或多机器人账号时用于区分同一个外部对象。"
              label="通道账号 ID"
            >
              <UiInput
                onChange={(event) => setField("accountId", event.target.value)}
                placeholder="可选，例如扫码账号 ID / bot id"
                value={draft.accountId}
                variant="dialog"
              />
            </UiField>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField label="Thread / 话题 ID">
                <UiInput
                  onChange={(event) => setField("threadId", event.target.value)}
                  placeholder="可选，例如 Telegram topic 或 Discord thread"
                  value={draft.threadId}
                  variant="dialog"
                />
              </UiField>
              <UiField label="显示名称">
                <UiInput
                  onChange={(event) => setField("externalName", event.target.value)}
                  placeholder="可选，用于配对列表识别"
                  value={draft.externalName}
                  variant="dialog"
                />
              </UiField>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
                <UiSelectMenu
                  ariaLabel="选择处理智能体"
                  disabled={agents.length === 0}
                  onChange={(value) => setField("agentId", value)}
                  options={agents.map((agent) => ({
                    value: agent.agent_id,
                    label: agent.name,
                  }))}
                  size="sm"
                  value={draft.agentId}
                />
              </UiField>
              <UiField label="初始状态">
                <UiSelectMenu
                  ariaLabel="选择初始配对状态"
                  onChange={(value) => setField(
                    "status",
                    value as ImPairingStatus,
                  )}
                  options={CREATE_PAIRING_STATUS_OPTIONS}
                  size="sm"
                  value={draft.status}
                />
              </UiField>
            </div>

            <div className="rounded-[12px] border border-(--divider-subtle-color) px-3 py-2 text-[12px] leading-5 text-(--text-muted)">
              手动配对适用于已经从外部平台拿到稳定会话 ID 的场景。首次入站消息仍会自动创建待处理配对。
            </div>
          </UiDialogBody>

          <UiDialogFooter>
            <UiButton
              className="min-w-[104px]"
              disabled={saving}
              onClick={onClose}
              size="lg"
              type="button"
            >
              取消
            </UiButton>
            <UiButton
              className="min-w-[124px]"
              disabled={saving || !draft.externalRef.trim() || !draft.agentId}
              size="lg"
              tone="primary"
              type="submit"
              variant="solid"
            >
              {saving
                ? <Loader2 className="h-5 w-5 animate-spin" />
                : <Plus className="h-5 w-5" />}
              {saving ? "创建中..." : "新增配对"}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
