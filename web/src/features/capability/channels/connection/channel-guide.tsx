// INPUT: 当前 Channel 类型及其真实平台配置步骤。
// OUTPUT: 默认收起、按需展开的接入说明，不占据连接表单的首屏。
// POS: Channel 平台准备步骤的单一文案源，操作说明保留但不作为弹窗视觉主角。
import { Fragment } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

import type {
  ChannelConfigView,
  ImChannelType,
} from "@/lib/api/capability/channel-api";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface ChannelGuideContent {
  noteKey?: TranslationKey;
  steps: Array<{ contentKey: TranslationKey; id: string; link?: { href: string; labelKey: TranslationKey } }>;
}

const CHANNEL_GUIDES: Record<ImChannelType, ChannelGuideContent> = {
  dingtalk: {
    steps: [
      { id: "dingtalk-create-app", contentKey: "capability.channel_guide_dingtalk_create_app", link: { href: "https://open.dingtalk.com/", labelKey: "capability.channel_guide_dingtalk_create_app_link" } },
      { id: "dingtalk-stream-mode", contentKey: "capability.channel_guide_dingtalk_stream_mode" },
      { id: "dingtalk-copy-credentials", contentKey: "capability.channel_guide_dingtalk_copy_credentials" },
      { id: "dingtalk-session-webhook", contentKey: "capability.channel_guide_dingtalk_session_webhook" },
      { id: "dingtalk-publish-version", contentKey: "capability.channel_guide_dingtalk_publish_version" },
      { id: "dingtalk-add-bot", contentKey: "capability.channel_guide_dingtalk_add_bot" },
    ],
    noteKey: "capability.channel_guide_dingtalk_note",
  },
  discord: {
    steps: [
      { id: "discord-create-app", contentKey: "capability.channel_guide_discord_create_app", link: { href: "https://discord.com/developers/applications", labelKey: "capability.channel_guide_discord_create_app_link" } },
      { id: "discord-copy-token", contentKey: "capability.channel_guide_discord_copy_token" },
      { id: "discord-message-content", contentKey: "capability.channel_guide_discord_message_content" },
      { id: "discord-authorize", contentKey: "capability.channel_guide_discord_authorize" },
    ],
  },
  feishu: {
    steps: [
      { id: "feishu-create-app", contentKey: "capability.channel_guide_feishu_create_app", link: { href: "https://open.feishu.cn/", labelKey: "capability.channel_guide_feishu_create_app_link" } },
      { id: "feishu-copy-credentials", contentKey: "capability.channel_guide_feishu_copy_credentials" },
      { id: "feishu-permissions", contentKey: "capability.channel_guide_feishu_permissions" },
      { id: "feishu-events", contentKey: "capability.channel_guide_feishu_events" },
      { id: "feishu-verification", contentKey: "capability.channel_guide_feishu_verification" },
      { id: "feishu-add-bot", contentKey: "capability.channel_guide_feishu_add_bot" },
    ],
    noteKey: "capability.channel_guide_feishu_note",
  },
  telegram: {
    steps: [
      { id: "telegram-create-bot", contentKey: "capability.channel_guide_telegram_create_bot", link: { href: "https://t.me/BotFather", labelKey: "capability.channel_guide_telegram_create_bot_link" } },
      { id: "telegram-copy-token", contentKey: "capability.channel_guide_telegram_copy_token" },
      { id: "telegram-save-token", contentKey: "capability.channel_guide_telegram_save_token" },
      { id: "telegram-add-bot", contentKey: "capability.channel_guide_telegram_add_bot" },
    ],
  },
  wechat: {
    steps: [
      { id: "wechat-create-bot", contentKey: "capability.channel_guide_wechat_create_bot", link: { href: "https://developer.work.weixin.qq.com/", labelKey: "capability.channel_guide_wechat_create_bot_link" } },
      { id: "wechat-copy-credentials", contentKey: "capability.channel_guide_wechat_copy_credentials" },
      { id: "wechat-long-connection", contentKey: "capability.channel_guide_wechat_long_connection" },
      { id: "wechat-stream-reply", contentKey: "capability.channel_guide_wechat_stream_reply" },
      { id: "wechat-approve-pairing", contentKey: "capability.channel_guide_wechat_approve_pairing" },
    ],
  },
  "weixin-personal": {
    steps: [
      { id: "weixin-personal-login", contentKey: "capability.channel_guide_weixin_personal_login" },
      { id: "weixin-personal-scan", contentKey: "capability.channel_guide_weixin_personal_scan" },
      { id: "weixin-personal-token", contentKey: "capability.channel_guide_weixin_personal_token" },
      { id: "weixin-personal-send", contentKey: "capability.channel_guide_weixin_personal_send" },
      { id: "weixin-personal-approve", contentKey: "capability.channel_guide_weixin_personal_approve" },
    ],
    noteKey: "capability.channel_guide_weixin_personal_note",
  },
};

export function ChannelGuide({
  item,
  runtimeNote,
}: {
  item: ChannelConfigView;
  runtimeNote?: string;
}) {
  const { t } = useI18n();
  const guide = CHANNEL_GUIDES[item.channel_type];
  return (
    <UiDisclosure label={t("capability.channel_guide_heading")} variant="panel">
      <ol className={cn(
        "list-decimal space-y-1 pl-5 [&_a]:font-semibold [&_a]:text-(--primary) [&_b]:font-semibold",
        getUiTypographyClassName({ role: "supporting", tone: "default" }),
      )}>
        {guide.steps.map((step) => (
          <li key={step.id}>
            {t(step.contentKey).split(/(\*\*.*?\*\*|\{link\})/g).map((part, index) => (
              <Fragment key={index}>
                {part === "{link}" && step.link ? (
                  <a href={step.link.href} target="_blank" rel="noopener noreferrer">
                    {t(step.link.labelKey)}
                  </a>
                ) : part.startsWith("**") && part.endsWith("**") ? (
                  <b>{part.slice(2, -2)}</b>
                ) : part}
              </Fragment>
            ))}
          </li>
        ))}
      </ol>
      {runtimeNote ? (
        <div className={cn(
          "mt-3 border-t border-(--divider-subtle-color) pt-3",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
        )}>
          {runtimeNote}
        </div>
      ) : null}
      {guide.noteKey ? (
        <div className={cn(
          "mt-3 border-t border-(--divider-subtle-color) pt-3",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
        )}>
          {t(guide.noteKey)}
        </div>
      ) : null}
    </UiDisclosure>
  );
}
