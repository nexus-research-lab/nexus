"use client";

import { Link2, Search } from "lucide-react";

import { cn } from "@/lib/utils";

import type { ConnectorController } from "@/hooks/use-connector-controller";

/** 类别列表 */
const CATEGORIES = [
  { key: "all", label: "全部" },
  { key: "productivity", label: "效率工具" },
  { key: "social", label: "社交媒体" },
  { key: "ecommerce", label: "电商平台" },
  { key: "development", label: "开发工具" },
  { key: "business", label: "企业管理" },
  { key: "marketing", label: "营销分析" },
  { key: "automation", label: "自动化" },
];

interface ConnectorsSearchBarProps {
  ctrl: ConnectorController;
}

export function ConnectorsSearchBar({ ctrl }: ConnectorsSearchBarProps) {
  return (
    <div className="mb-5 flex flex-col gap-3">
      {/* 搜索行 */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1">
          <Search className="absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            className="home-glass-input w-full rounded-full py-2.5 pl-10 pr-4 text-sm placeholder:text-slate-400"
            onChange={(e) => ctrl.set_search_query(e.target.value)}
            placeholder="搜索应用授权..."
            type="text"
            value={ctrl.search_query}
          />
        </div>
        <div className="flex items-center gap-1.5 text-xs text-slate-500">
          <Link2 className="h-3.5 w-3.5" />
          <span>{ctrl.connectors.length} 个应用授权</span>
        </div>
      </div>

      {/* 类别筛选 */}
      <div className="flex flex-wrap gap-1.5">
        {CATEGORIES.map((cat) => (
          <button
            key={cat.key}
            className={cn(
              "rounded-full px-3 py-1 text-xs font-medium transition-all",
              ctrl.active_category === cat.key
                ? "workspace-chip text-slate-900"
                : "text-slate-500 hover:bg-white/40 hover:text-slate-700",
            )}
            onClick={() => ctrl.set_active_category(cat.key)}
            type="button"
          >
            {cat.label}
          </button>
        ))}
      </div>
    </div>
  );
}
