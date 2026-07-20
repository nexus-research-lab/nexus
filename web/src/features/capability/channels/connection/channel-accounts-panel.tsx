import {
  Loader2,
  Trash2,
} from "lucide-react";

import { ChannelAccountView } from "@/lib/api/capability/channel-api";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { channelAccountStatusLabel } from "./channel-connection-model";

export function ChannelAccountsPanel({
  accounts,
  deletingAccountId,
  onDelete,
}: {
  accounts: ChannelAccountView[];
  deletingAccountId: string;
  onDelete: (account: ChannelAccountView) => void;
}) {
  return (
    <div className="rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 py-3">
      <div className="flex min-w-0 items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="text-[13px] font-semibold text-(--text-strong)">已连接账号</div>
        </div>
        <UiBadge size="xs">{accounts.length} 个</UiBadge>
      </div>
      {accounts.length === 0 ? (
        <div className="mt-3 rounded-[10px] border border-dashed border-(--divider-subtle-color) px-3 py-2 text-[12px] text-(--text-muted)">
          暂无已连接账号
        </div>
      ) : (
        <div className="mt-2 space-y-1.5">
          {accounts.map((account) => (
            <div
              className="flex min-w-0 items-center justify-between gap-3 rounded-[8px] border border-(--divider-subtle-color) px-2.5 py-2"
              key={account.account_id}
            >
              <div className="min-w-0">
                <div className="flex min-w-0 items-center gap-2">
                  <code className="min-w-0 truncate text-[12px] font-semibold text-(--text-strong)" title={account.account_id}>
                    {account.account_id}
                  </code>
                  <UiBadge size="xs" tone={account.status === "error" ? "danger" : "success"}>
                    {channelAccountStatusLabel(account.status)}
                  </UiBadge>
                </div>
                <div className="mt-0.5 truncate text-[11px] text-(--text-muted)">
                  {account.user_id ? `用户 ${account.user_id} · ` : ""}更新 {new Date(account.updated_at).toLocaleString()}
                </div>
                {account.last_error ? (
                  <div className="mt-0.5 truncate text-[11px] text-(--destructive)" title={account.last_error}>
                    {account.last_error}
                  </div>
                ) : null}
              </div>
              <UiListActionButton
                disabled={deletingAccountId === account.account_id}
                onClick={() => onDelete(account)}
                size="sm"
                stopPropagation
                title="删除该账号"
              >
                {deletingAccountId === account.account_id ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <Trash2 className="h-3 w-3" />
                )}
              </UiListActionButton>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
