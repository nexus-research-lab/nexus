/**
 * INPUT: Agent 目录、配对草稿与创建命令。
 * OUTPUT: 必填信息优先、可选路由字段按需展开的 plain 配对表单。
 * POS: IM 配对目录的手动创建边界；不在标题区解释匹配协议。
 */
"use client";

import { Loader2 } from "lucide-react";
import {
  type FormEvent,
  useEffect,
  useRef,
  useState,
} from "react";

import {
  type CreatePairingPayload,
  type ImChannelType,
  type ImChatType,
  type ImPairingStatus,
} from "@/lib/api/capability/channel-api";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { FeedbackBanner } from "@/shared/ui/feedback/feedback-banner";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
  blocked: boolean;
  failure: FeedbackBannerProps | null;
  onClose: () => void;
  onCreate: (payload: CreatePairingPayload) => Promise<boolean>;
}

export function CreatePairingDialog({
  agents,
  blocked,
  failure,
  onClose,
  onCreate,
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
    if (!payload || savingRef.current || blocked) {
      return;
    }
    savingRef.current = true;
    setSaving(true);
    try {
      if (await onCreate(payload)) {
        onClose();
      }
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialog"
        labelledBy="create-pairing-dialog-title"
        onClose={onClose}
      >
        <UiDialogFormShell
          onSubmit={handleSubmit}
          size="lg"
          viewport="adaptiveMax"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title="新增配对"
            titleId="create-pairing-dialog-title"
          />

          <UiDialogBody className="space-y-4" scrollable>
            {failure ? <FeedbackBanner {...failure} /> : null}
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
              htmlFor="pairing-external-ref"
              label="外部对象 ID"
              required
            >
              <UiInput
                id="pairing-external-ref"
                onChange={(event) => setField("externalRef", event.target.value)}
                pattern=".*\S.*"
                placeholder={draft.chatType === "group"
                  ? "群 ID / chat_id / channel_id"
                  : "用户 ID / open_id / chat_id"}
                required
                value={draft.externalRef}
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

            <UiField label="处理智能体" required>
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

            <UiDisclosure
              label="账号、话题与初始状态"
              summaryTone="muted"
              variant="section"
            >
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <UiField
                  description="多账号接入时用于区分同一个外部对象。"
                  label="通道账号 ID"
                >
                  <UiInput
                    onChange={(event) => setField("accountId", event.target.value)}
                    placeholder="扫码账号 ID / bot id"
                    value={draft.accountId}
                    variant="dialog"
                  />
                </UiField>
                <UiField label="Thread / 话题 ID">
                  <UiInput
                    onChange={(event) => setField("threadId", event.target.value)}
                    placeholder="Telegram topic / Discord thread"
                    value={draft.threadId}
                    variant="dialog"
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
              <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
                仅在已知稳定外部 ID 时手动创建；首次入站消息仍会生成待处理配对。
              </p>
            </UiDisclosure>
          </UiDialogBody>

          <UiDialogFooter appearance="plain">
            <UiButton
              disabled={saving || blocked}
              onClick={onClose}
              type="button"
            >
              取消
            </UiButton>
            <UiButton
              disabled={saving || blocked || !draft.agentId}
              tone="primary"
              type="submit"
              variant="solid"
            >
              {saving ? (
                <Loader2 className={getUiSpinnerClassName({ size: "md" })} />
              ) : null}
              {saving ? "创建中..." : "新增配对"}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
