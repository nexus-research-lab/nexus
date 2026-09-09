// INPUT: 当前认证加载态、运营角色与宿主类型。
// OUTPUT: 具名等待状态或转向唯一设置运营入口。
// POS: 旧运营路径兼容入口，不挂载运营资源或复制设置布局。
"use client";

import { Navigate } from "react-router-dom";
import { LoaderCircle } from "lucide-react";

import { APP_ROUTE_PATHS, AppRouteBuilders } from "@/shared/navigation/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { canUseOperations } from "@/features/settings/operations/operations-access";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

export function OperationsPage() {
  const { loading, status } = useAuth();
  const { t } = useI18n();

  if (loading) {
    return (
      <div aria-label={t("common.loading")} aria-busy="true" role="status" className="flex h-full items-center justify-center">
        <LoaderCircle
          aria-hidden="true"
          className={getUiSpinnerClassName({ size: "xl", tone: "primary" })}
        />
      </div>
    );
  }

  if (isDesktopRuntime() || !canUseOperations(status?.role)) {
    return <Navigate replace to={APP_ROUTE_PATHS.home} />;
  }

  return <Navigate replace to={AppRouteBuilders.settings("operations")} />;
}
