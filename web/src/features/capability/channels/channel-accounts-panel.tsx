import {
  Loader2,
  Trash2,
} from "lucide-react";

import { ChannelAccountView } from "@/lib/api/channel-api";
import { UiBadge } from "@/shared/ui/badge";
import { UiListActionButton } from "@/shared/ui/list-action";

function channel_account_status_label(status: string) {
  switch (status) {
  case "connected":
    return "已连接";
  case "configured":
    return "已配置";
  case "pending":
    return "待确认";
  case "error":
    return "异常";
  case "disabled":
    return "已停用";
  default:
    return status || "未知";
  }
}

export function ChannelAccountsPanel({
  accounts,
  deleting_account_id,
  on_delete,
}: {
  accounts: ChannelAccountView[];
  deleting_account_id: string;
  on_delete: (account: ChannelAccountView) => void;
}) {
  return (
    <div className="rounded-[14px] border border-(--divider-subtle-color) bg-transparent px-4 py-3">
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
        <div className="mt-3 space-y-2">
          {accounts.map((account) => (
            <div
              className="flex min-w-0 items-center justify-between gap-3 rounded-[10px] border border-(--divider-subtle-color) px-3 py-2"
              key={account.account_id}
            >
              <div className="min-w-0">
                <div className="flex min-w-0 items-center gap-2">
                  <code className="min-w-0 truncate text-[12px] font-semibold text-(--text-strong)" title={account.account_id}>
                    {account.account_id}
                  </code>
                  <UiBadge size="xs" tone={account.status === "error" ? "danger" : "success"}>
                    {channel_account_status_label(account.status)}
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
                disabled={deleting_account_id === account.account_id}
                onClick={() => on_delete(account)}
                size="sm"
                stop_propagation
                title="删除该账号"
              >
                {deleting_account_id === account.account_id ? (
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
