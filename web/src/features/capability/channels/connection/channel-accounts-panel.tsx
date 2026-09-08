/**
 * INPUT: 已连接频道账号、删除状态与删除动作。
 * OUTPUT: 展示可识别账号身份、连接状态、更新时间与当前错误的管理列表。
 * POS: 频道连接详情的账号管理区；账号/用户标识是被管理对象，不是可隐藏的诊断字段。
 */
import {
  Loader2,
  Trash2,
} from "lucide-react";

import type { ChannelAccountView } from "@/lib/api/capability/channel-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { channelAccountStatusLabel } from "./channel-connection-model";

export function ChannelAccountsPanel({
  accounts,
  busy,
  deletingAccountId,
  onDelete,
}: {
  accounts: ChannelAccountView[];
  busy: boolean;
  deletingAccountId: string;
  onDelete: (account: ChannelAccountView) => void;
}) {
  const { t } = useI18n();
  return (
    <UiPanel padding="sm" radius="sm">
      <div className="flex min-w-0 items-center justify-between gap-3">
        <h3 className={cn(
          "min-w-0",
          getUiTypographyClassName({
            role: "control",
            tone: "strong",
            weight: "semibold",
          }),
        )}>
          已连接账号
        </h3>
        <UiBadge size="xs">{accounts.length} 个</UiBadge>
      </div>
      {accounts.length === 0 ? (
        <p className={cn(
          "mt-3",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
        )}>
          暂无已连接账号
        </p>
      ) : (
        <div className="mt-2 space-y-1.5">
          {accounts.map((account) => (
            <UiPanel
              className="flex min-w-0 items-center justify-between gap-3 px-2.5 py-2"
              key={account.account_id}
              padding="none"
              radius="sm"
            >
              <div className="min-w-0">
                <div className="flex min-w-0 items-center gap-2">
                  <code
                    className={cn(
                      "min-w-0 truncate",
                      getUiTypographyClassName({
                        role: "code",
                        tone: "strong",
                        weight: "semibold",
                      }),
                    )}
                    title={account.user_id || account.account_id}
                  >
                    {account.user_id || account.account_id}
                  </code>
                  <UiBadge size="xs" tone={account.status === "error" ? "danger" : "success"}>
                    {channelAccountStatusLabel(account.status)}
                  </UiBadge>
                </div>
                <div className={cn(
                  "mt-0.5 truncate",
                  getUiTypographyClassName({ role: "caption", tone: "muted" }),
                )}>
                  {account.user_id && account.user_id !== account.account_id
                    ? `账号 ${account.account_id} · `
                    : ""}
                  更新于 {new Date(account.updated_at).toLocaleString()}
                </div>
                {account.last_error ? (
                  <UiInlineNotice
                    className="mt-2"
                    message={(
                      <div className="space-y-1">
                        <p>
                          {t("capability.channel_account_error_message")}
                        </p>
                        <RecoverySummary
                          impact={t("capability.channel_account_error_impact")}
                          nextStep={t("capability.channel_account_error_next_step")}
                        />
                      </div>
                    )}
                    title={t("capability.channel_account_error_title")}
                    tone="danger"
                  />
                ) : null}
              </div>
              <UiListActionButton
                disabled={busy || deletingAccountId === account.account_id}
                onClick={() => onDelete(account)}
                size="sm"
                stopPropagation
                title="删除该账号"
              >
                {deletingAccountId === account.account_id ? (
                  <Loader2 className={getUiSpinnerClassName({ size: "xs" })} />
                ) : (
                  <Trash2 className="h-3 w-3" />
                )}
              </UiListActionButton>
            </UiPanel>
          ))}
        </div>
      )}
    </UiPanel>
  );
}
