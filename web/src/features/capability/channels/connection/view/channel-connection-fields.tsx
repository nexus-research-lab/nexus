import { ExternalLink } from "lucide-react";

import type {
  ChannelAccountView,
  ChannelConfigView,
  ChannelCredentialField,
  ChannelLoginView,
} from "@/lib/api/capability/channel-api";
import { UiButton } from "@/shared/ui/button/button";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
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
  currentItem: ChannelConfigView;
  deletingAccountId: string;
  discordOauthUrl: string;
  draft: ChannelConnectionDraft;
  loginLoading: boolean;
  loginView: ChannelLoginView | null;
  requestDeleteAccount: (account: ChannelAccountView) => void;
  setAgentId: (agentId: string) => void;
  submitVerifyCode: (value: string) => Promise<boolean>;
  supportsPersonalWeixinLogin: boolean;
  updateField: (field: ChannelCredentialField, value: string) => void;
}

interface ChannelConnectionFieldsProps {
  agents: Agent[];
  controller: ChannelConnectionFieldsController;
}

function openDiscordOauth(url: string): void {
  if (!url) {
    return;
  }
  window.open(url, "_blank", "noopener,noreferrer");
}

export function ChannelConnectionFields({
  agents,
  controller,
}: ChannelConnectionFieldsProps) {
  const { currentItem, draft } = controller;
  return (
    <>
      <ChannelGuide item={currentItem} />

      {currentItem.runtime_note ? (
        <div className="rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 py-2.5 text-[12px] font-medium leading-5 text-(--text-default)">
          {currentItem.runtime_note}
        </div>
      ) : null}

      {controller.supportsPersonalWeixinLogin ? (
        <ChannelLoginPanel
          loading={controller.loginLoading}
          loginView={controller.loginView}
          onSubmitVerifyCode={(value) => {
            void controller.submitVerifyCode(value);
          }}
        />
      ) : null}

      {controller.supportsPersonalWeixinLogin ? (
        <ChannelAccountsPanel
          accounts={currentItem.accounts || []}
          deletingAccountId={controller.deletingAccountId}
          onDelete={controller.requestDeleteAccount}
        />
      ) : null}

      <UiField label={<>处理智能体 <span className="text-(--destructive)">*</span></>}>
        <UiSelectMenu
          ariaLabel="选择频道处理智能体"
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
            label={(
              <>
                {field.label} {field.required ? <span className="text-(--destructive)">*</span> : null}
              </>
            )}
          >
            <UiInput
              autoCapitalize="none"
              autoComplete={channelFieldAutocomplete(field)}
              autoCorrect="off"
              data-1p-ignore="true"
              data-form-type="other"
              data-lpignore="true"
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
          <UiButton
            className="w-full"
            disabled={!controller.discordOauthUrl}
            onClick={() => openDiscordOauth(controller.discordOauthUrl)}
            size="lg"
            tone="primary"
            type="button"
            variant="solid"
          >
            <ExternalLink className="h-5 w-5" />
            授权机器人
          </UiButton>
        </UiField>
      ) : null}
    </>
  );
}
