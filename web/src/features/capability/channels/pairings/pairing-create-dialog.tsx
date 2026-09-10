/**
 * INPUT: Agent 目录、配对草稿与创建命令。
 * OUTPUT: plain 配对表单；缺项 Agent 保留原选择并阻止提交，不能自动改绑其他对象。
 * POS: IM 配对目录的手动创建边界；不在标题区解释匹配协议。
 */
"use client";

import { Loader2 } from "lucide-react";
import {
  type FormEvent,
  useEffect,
  useId,
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
import { buildAgentSelectionOptions, includeUnavailableAgentSelection } from "@/lib/agent-selection-options";
import { useI18n } from "@/shared/i18n/i18n-context";
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
  getPairingOptions,
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
  const { t } = useI18n();
  const fieldId = useId();
  const options = getPairingOptions(t);
  const savingRef = useRef(false);
  const [draft, setDraft] = useState(() => createPairingDraft(
    agents[0]?.agent_id || "",
  ));
  const [saving, setSaving] = useState(false);
  const selectedAgentAvailable = agents.some((agent) => agent.agent_id === draft.agentId);
  const agentOptions = includeUnavailableAgentSelection(buildAgentSelectionOptions(agents, t), draft.agentId, t);

  useEffect(() => {
    if (draft.agentId) {
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
    if (!payload || !selectedAgentAvailable || savingRef.current || blocked) {
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

  const close = () => {
    if (!savingRef.current) onClose();
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialog"
        labelledBy={`${fieldId}-title`}
        onClose={close}
      >
        <UiDialogFormShell
          aria-busy={saving}
          onSubmit={handleSubmit}
          size="lg"
          viewport="adaptiveMax"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={close}
            title={t("capability.pairing_new")}
            titleId={`${fieldId}-title`}
          />

          <UiDialogBody className="space-y-4" scrollable>
            {failure ? <FeedbackBanner {...failure} /> : null}
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <UiField htmlFor={`${fieldId}-channel`} label={t("capability.pairing_channel")}>
                <UiSelectMenu
                  disabled={saving || blocked}
                  ariaLabel={t("capability.pairing_select_channel")}
                  id={`${fieldId}-channel`}
                  onChange={(value) => setField(
                    "channelType",
                    value as ImChannelType,
                  )}
                  options={options.channels}
                  size="sm"
                  value={draft.channelType}
                />
              </UiField>
              <UiField htmlFor={`${fieldId}-chat-type`} label={t("capability.pairing_chat_type")}>
                <UiSelectMenu
                  disabled={saving || blocked}
                  ariaLabel={t("capability.pairing_select_chat_type")}
                  id={`${fieldId}-chat-type`}
                  onChange={(value) => setField(
                    "chatType",
                    value as ImChatType,
                  )}
                  options={options.chatTypes}
                  size="sm"
                  value={draft.chatType}
                />
              </UiField>
            </div>

            <UiField
              description={t("capability.pairing_external_hint")}
              htmlFor={`${fieldId}-external-ref`}
              label={t("capability.pairing_external_id")}
              required
            >
              <UiInput
                disabled={saving || blocked}
                id={`${fieldId}-external-ref`}
                onChange={(event) => setField("externalRef", event.target.value)}
                pattern=".*\S.*"
                placeholder={draft.chatType === "group"
                  ? t("capability.pairing_group_placeholder")
                  : t("capability.pairing_user_placeholder")}
                required
                value={draft.externalRef}
                variant="dialog"
              />
            </UiField>

            <UiField htmlFor={`${fieldId}-name`} label={t("capability.pairing_display_name")}>
              <UiInput
                disabled={saving || blocked}
                id={`${fieldId}-name`}
                onChange={(event) => setField("externalName", event.target.value)}
                placeholder={t("capability.pairing_name_placeholder")}
                value={draft.externalName}
                variant="dialog"
              />
            </UiField>

            <UiField htmlFor={`${fieldId}-agent`} label={t("capability.pairing_agent")} required>
                <UiSelectMenu
                  disabled={saving || blocked || agents.length === 0}
                  ariaLabel={t("capability.pairing_select_agent")}
                  id={`${fieldId}-agent`}
                  onChange={(value) => setField("agentId", value)}
                  options={agentOptions}
                  size="sm"
                  value={draft.agentId}
                />
            </UiField>

            <UiDisclosure
              label={t("capability.pairing_advanced")}
              summaryTone="muted"
              variant="section"
            >
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <UiField
                  description={t("capability.pairing_account_hint")}
                  htmlFor={`${fieldId}-account`}
                  label={t("capability.pairing_account")}
                >
                  <UiInput
                    disabled={saving || blocked}
                    id={`${fieldId}-account`}
                    onChange={(event) => setField("accountId", event.target.value)}
                    placeholder={t("capability.pairing_account_placeholder")}
                    value={draft.accountId}
                    variant="dialog"
                  />
                </UiField>
                <UiField htmlFor={`${fieldId}-thread`} label={t("capability.pairing_thread")}>
                  <UiInput
                    disabled={saving || blocked}
                    id={`${fieldId}-thread`}
                    onChange={(event) => setField("threadId", event.target.value)}
                    placeholder="Telegram topic / Discord thread"
                    value={draft.threadId}
                    variant="dialog"
                  />
                </UiField>
                <UiField htmlFor={`${fieldId}-status`} label={t("capability.pairing_initial_status")}>
                  <UiSelectMenu
                    disabled={saving || blocked}
                    ariaLabel={t("capability.pairing_select_initial_status")}
                    id={`${fieldId}-status`}
                    onChange={(value) => setField(
                      "status",
                      value as ImPairingStatus,
                    )}
                    options={options.initialStatuses}
                    size="sm"
                    value={draft.status}
                  />
                </UiField>
              </div>
              <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
                {t("capability.pairing_manual_hint")}
              </p>
            </UiDisclosure>
          </UiDialogBody>

          <UiDialogFooter appearance="plain">
            <UiButton
              disabled={saving || blocked}
              onClick={close}
              type="button"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={saving || blocked || !selectedAgentAvailable}
              tone="primary"
              type="submit"
              variant="solid"
            >
              {saving ? (
                <Loader2 className={getUiSpinnerClassName({ size: "md" })} />
              ) : null}
              {saving ? t("capability.pairing_creating") : t("capability.pairing_new")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
