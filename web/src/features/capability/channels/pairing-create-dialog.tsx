"use client";

import { Loader2, Plus, ShieldCheck } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";

import {
  create_pairing_api,
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
  on_close,
  on_created,
  on_error,
}: {
  agents: Agent[];
  on_close: () => void;
  on_created: (item: PairingView) => void;
  on_error: (message: string) => void;
}) {
  const [channel_type, set_channel_type] = useState<ImChannelType>("feishu");
  const [chat_type, set_chat_type] = useState<ImChatType>("dm");
  const [external_ref, set_external_ref] = useState("");
  const [thread_id, set_thread_id] = useState("");
  const [external_name, set_external_name] = useState("");
  const [agent_id, set_agent_id] = useState(agents[0]?.agent_id || "");
  const [status, set_status] = useState<ImPairingStatus>("active");
  const [saving, set_saving] = useState(false);

  useEffect(() => {
    if (agent_id && agents.some((agent) => agent.agent_id === agent_id)) {
      return;
    }
    set_agent_id(agents[0]?.agent_id || "");
  }, [agent_id, agents]);

  const handle_submit = async (event: FormEvent) => {
    event.preventDefault();
    const normalized_ref = external_ref.trim();
    if (!normalized_ref || !agent_id || saving) {
      return;
    }
    set_saving(true);
    try {
      const created = await create_pairing_api({
        channel_type,
        chat_type,
        external_ref: normalized_ref,
        thread_id: thread_id.trim() || undefined,
        external_name: external_name.trim() || undefined,
        agent_id,
        status,
      });
      on_created(created);
      on_close();
    } catch (error) {
      on_error(error instanceof Error ? error.message : "新增配对失败");
    } finally {
      set_saving(false);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop class_name="z-[9999]" labelled_by="create-pairing-dialog-title" on_close={on_close}>
        <UiDialogFormShell class_name="max-h-[86vh]" onSubmit={handle_submit} size="lg">
          <UiDialogHeader
            icon={<ShieldCheck className="h-5 w-5" />}
            on_close={on_close}
            subtitle="为已知外部用户、群或话题预先建立 IM 授权关系；只有渠道、会话类型、外部 ID 和 Thread 都相同时才会更新已有配对。"
            title="新增 IM 配对"
            title_id="create-pairing-dialog-title"
          />

          <UiDialogBody class_name="space-y-4" scrollable>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField label="渠道">
                <UiSelectMenu
                  aria_label="选择 IM 渠道"
                  on_change={(value) => set_channel_type(value as ImChannelType)}
                  options={CHANNEL_OPTIONS}
                  size="sm"
                  value={channel_type}
                />
              </UiField>
              <UiField label="会话类型">
                <UiSelectMenu
                  aria_label="选择会话类型"
                  on_change={(value) => set_chat_type(value as ImChatType)}
                  options={CHAT_TYPE_OPTIONS}
                  size="sm"
                  value={chat_type}
                />
              </UiField>
            </div>

            <UiField
              description="同一智能体可以绑定多个不同外部对象，每个对象会生成独立 IM session。"
              label={<>外部对象 ID <span className="text-(--destructive)">*</span></>}
            >
              <UiInput
                onChange={(event) => set_external_ref(event.target.value)}
                placeholder={chat_type === "group" ? "群 ID / chat_id / channel_id" : "用户 ID / open_id / chat_id"}
                required
                value={external_ref}
                variant="dialog"
              />
            </UiField>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField label="Thread / 话题 ID">
                <UiInput
                  onChange={(event) => set_thread_id(event.target.value)}
                  placeholder="可选，例如 Telegram topic 或 Discord thread"
                  value={thread_id}
                  variant="dialog"
                />
              </UiField>
              <UiField label="显示名称">
                <UiInput
                  onChange={(event) => set_external_name(event.target.value)}
                  placeholder="可选，用于配对列表识别"
                  value={external_name}
                  variant="dialog"
                />
              </UiField>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
                <UiSelectMenu
                  aria_label="选择处理智能体"
                  disabled={agents.length === 0}
                  on_change={set_agent_id}
                  options={agents.map((agent) => ({
                    value: agent.agent_id,
                    label: agent.name,
                  }))}
                  size="sm"
                  value={agent_id}
                />
              </UiField>
              <UiField label="初始状态">
                <UiSelectMenu
                  aria_label="选择初始配对状态"
                  on_change={(value) => set_status(value as ImPairingStatus)}
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
            <UiButton class_name="min-w-[104px]" disabled={saving} onClick={on_close} size="lg" type="button">
              取消
            </UiButton>
            <UiButton
              class_name="min-w-[124px]"
              disabled={saving || !external_ref.trim() || !agent_id}
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
