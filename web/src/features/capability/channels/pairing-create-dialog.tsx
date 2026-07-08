"use client";

import { Loader2, Plus, ShieldCheck } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";

import {
  createPairingApi,
  ImChatType,
  ImChannelType,
  ImPairingStatus,
  PairingView,
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
import { UiSelectMenu } from "@/shared/ui/select-menu";
import type { Agent } from "@/types/agent/agent";

import {
  CHANNEL_OPTIONS,
  CHAT_TYPE_OPTIONS,
  CREATE_PAIRING_STATUS_OPTIONS,
} from "./pairing-options";

export function CreatePairingDialog({
  agents,
  onClose: onClose,
  onCreated: onCreated,
  onError: onError,
}: {
  agents: Agent[];
  onClose: () => void;
  onCreated: (item: PairingView) => void;
  onError: (message: string) => void;
}) {
  const [channelType, setChannelType] = useState<ImChannelType>("feishu");
  const [accountId, setAccountId] = useState("");
  const [chatType, setChatType] = useState<ImChatType>("dm");
  const [externalRef, setExternalRef] = useState("");
  const [threadId, setThreadId] = useState("");
  const [externalName, setExternalName] = useState("");
  const [agentId, setAgentId] = useState(agents[0]?.agent_id || "");
  const [status, setStatus] = useState<ImPairingStatus>("active");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (agentId && agents.some((agent) => agent.agent_id === agentId)) {
      return;
    }
    setAgentId(agents[0]?.agent_id || "");
  }, [agentId, agents]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const normalizedRef = externalRef.trim();
    if (!normalizedRef || !agentId || saving) {
      return;
    }
    setSaving(true);
    try {
      const created = await createPairingApi({
        channel_type: channelType,
        account_id: accountId.trim() || undefined,
        chat_type: chatType,
        external_ref: normalizedRef,
        thread_id: threadId.trim() || undefined,
        external_name: externalName.trim() || undefined,
        agent_id: agentId,
        status,
      });
      onCreated(created);
      onClose();
    } catch (error) {
      onError(error instanceof Error ? error.message : "新增配对失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" labelledBy="create-pairing-dialog-title" onClose={onClose}>
        <UiDialogFormShell className="max-h-[86vh]" onSubmit={handleSubmit} size="lg">
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
                  onChange={(value) => setChannelType(value as ImChannelType)}
                  options={CHANNEL_OPTIONS}
                  size="sm"
                  value={channelType}
                />
              </UiField>
              <UiField label="会话类型">
                <UiSelectMenu
                  ariaLabel="选择会话类型"
                  onChange={(value) => setChatType(value as ImChatType)}
                  options={CHAT_TYPE_OPTIONS}
                  size="sm"
                  value={chatType}
                />
              </UiField>
            </div>

            <UiField
              description="同一智能体可以绑定多个不同外部对象，每个对象会生成独立 IM session。"
              label={<>外部对象 ID <span className="text-(--destructive)">*</span></>}
            >
              <UiInput
                onChange={(event) => setExternalRef(event.target.value)}
                placeholder={chatType === "group" ? "群 ID / chat_id / channel_id" : "用户 ID / open_id / chat_id"}
                required
                value={externalRef}
                variant="dialog"
              />
            </UiField>

            <UiField
              description="可选；多扫码账号或多机器人账号时用于区分同一个外部对象。"
              label="通道账号 ID"
            >
              <UiInput
                onChange={(event) => setAccountId(event.target.value)}
                placeholder="可选，例如扫码账号 ID / bot id"
                value={accountId}
                variant="dialog"
              />
            </UiField>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField label="Thread / 话题 ID">
                <UiInput
                  onChange={(event) => setThreadId(event.target.value)}
                  placeholder="可选，例如 Telegram topic 或 Discord thread"
                  value={threadId}
                  variant="dialog"
                />
              </UiField>
              <UiField label="显示名称">
                <UiInput
                  onChange={(event) => setExternalName(event.target.value)}
                  placeholder="可选，用于配对列表识别"
                  value={externalName}
                  variant="dialog"
                />
              </UiField>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
                <UiSelectMenu
                  ariaLabel="选择处理智能体"
                  disabled={agents.length === 0}
                  onChange={setAgentId}
                  options={agents.map((agent) => ({
                    value: agent.agent_id,
                    label: agent.name,
                  }))}
                  size="sm"
                  value={agentId}
                />
              </UiField>
              <UiField label="初始状态">
                <UiSelectMenu
                  ariaLabel="选择初始配对状态"
                  onChange={(value) => setStatus(value as ImPairingStatus)}
                  options={CREATE_PAIRING_STATUS_OPTIONS}
                  size="sm"
                  value={status}
                />
              </UiField>
            </div>

            <div className="rounded-[12px] border border-(--divider-subtle-color) px-3 py-2 text-[12px] leading-5 text-(--text-muted)">
              手动配对适用于已经从外部平台拿到稳定会话 ID 的场景。首次入站消息仍会自动创建待处理配对。
            </div>
          </UiDialogBody>

          <UiDialogFooter>
            <UiButton className="min-w-[104px]" disabled={saving} onClick={onClose} size="lg" type="button">
              取消
            </UiButton>
            <UiButton
              className="min-w-[124px]"
              disabled={saving || !externalRef.trim() || !agentId}
              size="lg"
              tone="primary"
              type="submit"
              variant="solid"
            >
              {saving ? <Loader2 className="h-5 w-5 animate-spin" /> : <Plus className="h-5 w-5" />}
              {saving ? "创建中..." : "新增配对"}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
