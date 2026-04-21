"use client";

import { AlertTriangle } from "lucide-react";
import { Link } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { cn } from "@/lib/utils";

interface ProviderUnavailableBannerProps {
  compact?: boolean;
}

export function ProviderUnavailableBanner({ compact = false }: ProviderUnavailableBannerProps) {
  return (
    <div className={cn(compact ? "px-1 pb-1" : "mx-auto w-full max-w-[980px] px-4 pb-2 sm:px-6 xl:px-8")}>
      <div className="flex items-center gap-2 rounded-xl border border-amber-500/25 bg-amber-500/8 px-3 py-2 text-xs text-amber-900 dark:text-amber-200">
        <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
        <span className="min-w-0 flex-1">
          未配置可用的 LLM Provider，发送后将由 Agent 报错返回。
        </span>
        <Link
          to={APP_ROUTE_PATHS.settings}
          className="shrink-0 font-medium underline-offset-2 hover:underline"
        >
          前往配置
        </Link>
      </div>
    </div>
  );
}
