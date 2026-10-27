// INPUT: 认证快照中的多人能力探测结果与 refreshStatus。
// OUTPUT: 在线服务不可用时的单行紧凑提醒、重试动作与关闭逻辑；可用或明确关闭时不渲染。
// POS: 聊天目录、联系人目录与建群弹窗共用的在线不可用单行提示。

import { CloudOff, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { isTeamCapabilityUnavailable, useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

export function TeamCapabilityNotice({ className }: { className?: string }) {
  const { t } = useI18n();
  const { status, refreshStatus } = useAuth();
  const [pending, setPending] = useState(false);
  const [dismissed, setDismissed] = useState(false);

  const unavailable = isTeamCapabilityUnavailable(status);

  // 账号作用域变化时恢复提示
  useEffect(() => {
    setDismissed(false);
  }, [status?.organization_id, status?.control_user_id]);

  const retry = useCallback(() => {
    if (pending) return;
    setPending(true);
    void refreshStatus().catch(() => undefined).finally(() => setPending(false));
  }, [pending, refreshStatus]);

  if (!unavailable || dismissed) return null;

  // 右边距 8px + 边框 1px + 内边距 7px + 20px 按钮半宽 10px = 26px；
  // 确保关闭按钮中轴与上方搜索动作、顶栏折叠按钮在同一垂线上。
  return (
    <aside
      aria-label={t("team.capability_unavailable")}
      className={cn(
        "ml-2 mr-2 mb-1.5 flex h-7 shrink-0 items-center justify-between gap-1.5 rounded-md border border-[color:color-mix(in_srgb,var(--warning)_26%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,var(--surface-control-background))] pl-2 pr-[7px] text-xs text-[color:color-mix(in_srgb,var(--warning)_85%,var(--text-strong))] shadow-xs max-[559px]:mx-4",
        className,
      )}
      role="status"
    >
      <div className="flex min-w-0 items-center gap-1.5">
        <CloudOff aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-(--warning)" />
        <span className="truncate">{t("team.capability_unavailable")}</span>
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        <button
          aria-busy={pending || undefined}
          className="flex items-center gap-1 font-medium text-(--warning) hover:underline disabled:opacity-50"
          disabled={pending}
          onClick={retry}
          type="button"
        >
          {pending ? (
            <span
              aria-hidden="true"
              className={cn("inline-block h-3 w-3", getUiSpinnerClassName({ size: "sm" }))}
            />
          ) : null}
          <span>{t("sidebar.retry")}</span>
        </button>
        <button
          aria-label={t("common.close")}
          className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-(--text-muted) transition-colors hover:bg-[color:color-mix(in_srgb,var(--warning)_18%,transparent)] hover:text-(--text-strong)"
          onClick={() => setDismissed(true)}
          type="button"
        >
          <X aria-hidden="true" className="h-3.5 w-3.5" />
        </button>
      </div>
    </aside>
  );
}
