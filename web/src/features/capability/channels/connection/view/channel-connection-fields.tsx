// INPUT: Channel 配置快照、表单草稿、登录状态与字段更新动作。
// OUTPUT: 首屏配置字段，以及按需展开的完整平台准备和 runtime 说明。
// POS: Channel 连接弹窗的字段视图，不复制控制器状态机。
import { ExternalLink } from "lucide-react";

import type {
  ChannelAccountView,
  ChannelConfigView,
  ChannelCredentialField,
  ChannelLoginView,
} from "@/lib/api/capability/channel-api";
import { UiButton, UiLinkButton } from "@/shared/ui/button/button";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import type { Agent } from "@/types/agent/agent";

import { ChannelAccountsPanel } from "../channel-accounts-panel";
import {
  channelFieldAutocomplete,
  channelFieldInputName,
  type ChannelConnectionDraft,
} from "../channel-connection-model";
import { ChannelGuide } from "../channel-guide";
import { ChannelLoginPanel } from "../login/channel-login-panel";

interface ChannelConnectionFieldsController {
  busy: boolean;
  currentItem: ChannelConfigView;
  deletingAccountId: string;
  discordOauthUrl: string;
  draft: ChannelConnectionDraft;
  loginLoading: boolean;
  loginMutationBlocked: boolean;
  loginRecoveryNotice: FeedbackBannerProps | null;
  loginView: ChannelLoginView | null;
  personalWeixin: boolean;
  requestDeleteAccount: (account: ChannelAccountView) => void;
  setAgentId: (agentId: string) => void;
  showsQRCode: boolean;
  submitVerifyCode: (value: string) => Promise<boolean>;
  updateField: (field: ChannelCredentialField, value: string) => void;
}

interface ChannelConnectionFieldsProps {
  agents: Agent[];
  controller: ChannelConnectionFieldsController;
}

export function ChannelConnectionFields({
  agents,
  controller,
}: ChannelConnectionFieldsProps) {
  const { currentItem, draft } = controller;
  return (
    <>
      <ChannelGuide item={currentItem} runtimeNote={currentItem.runtime_note} />

      {controller.showsQRCode ? (
        <ChannelLoginPanel
          channelType={currentItem.channel_type}
          channelTitle={currentItem.title}
          loading={controller.loginLoading}
          loginView={controller.loginView}
          mutationBlocked={controller.loginMutationBlocked}
          onSubmitVerifyCode={controller.submitVerifyCode}
          recoveryNotice={controller.loginRecoveryNotice}
        />
      ) : null}

      {controller.personalWeixin ? (
        <ChannelAccountsPanel
          accounts={currentItem.accounts || []}
          busy={controller.busy}
          deletingAccountId={controller.deletingAccountId}
          onDelete={controller.requestDeleteAccount}
        />
      ) : null}

      <UiField label="处理智能体" required>
        <UiSelectMenu
          ariaLabel="选择频道处理智能体"
          disabled={controller.busy}
          onChange={controller.setAgentId}
          options={agents.map((agent) => ({
            value: agent.agent_id,
            label: agent.name,
          }))}
          size="sm"
          value={draft.agentId}
        />
      </UiField>

      <div className="space-y-4">
        {currentItem.credential_fields.map((field, index) => (
          <UiField
            key={field.key}
            label={field.label}
            required={field.required}
          >
            <UiInput
              autoCapitalize="none"
              autoComplete={channelFieldAutocomplete(field)}
              autoCorrect="off"
              data-1p-ignore="true"
              data-form-type="other"
              data-lpignore="true"
              disabled={controller.busy}
              name={channelFieldInputName(currentItem.channel_type, index)}
              onChange={(event) => controller.updateField(field, event.target.value)}
              placeholder={field.placeholder || ""}
              required={field.required && !(field.secret && currentItem.has_credentials)}
              type={field.kind === "password" ? "password" : "text"}
              value={field.secret
                ? draft.credentials[field.key] || ""
                : draft.config[field.key] || ""}
              variant="dialog"
            />
          </UiField>
        ))}
      </div>

      {currentItem.channel_type === "discord" ? (
        <UiField label="授权机器人到服务器">
          {controller.busy || !controller.discordOauthUrl ? (
            <UiButton
              className="w-full"
              disabled
              size="lg"
              tone="primary"
              type="button"
              variant="solid"
            >
              <ExternalLink className="h-5 w-5" />
              授权机器人
            </UiButton>
          ) : (
            <UiLinkButton
              className="w-full"
              href={controller.discordOauthUrl}
              rel="noopener noreferrer"
              size="lg"
              target="_blank"
              tone="primary"
              variant="solid"
            >
              <ExternalLink className="h-5 w-5" />
              授权机器人
            </UiLinkButton>
          )}
        </UiField>
      ) : null}
    </>
  );
}
