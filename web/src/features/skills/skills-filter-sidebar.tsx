"use client";

import { WorkspaceSidebarItem } from "@/shared/ui/workspace-sidebar-item";
import { WorkspaceSidebarShell } from "@/shared/ui/workspace-sidebar-shell";

export type SkillsFilterKey = "all" | "installed" | "available" | "system";

interface SkillsFilterItem {
  key: SkillsFilterKey;
  label: string;
  count: number;
  dot_class_name?: string;
}

interface SkillsFilterSection {
  title: string;
  items: SkillsFilterItem[];
}

interface SkillsFilterSidebarProps {
  sections: SkillsFilterSection[];
  active_filter: SkillsFilterKey;
  total_count: number;
  on_change_filter: (filter: SkillsFilterKey) => void;
}

export function SkillsFilterSidebar({
  sections,
  active_filter,
  total_count,
  on_change_filter,
}: SkillsFilterSidebarProps) {
  return (
    <WorkspaceSidebarShell
      class_name="w-[248px]"
      subtitle={`${total_count} 个技能`}
      title="技能中心"
    >
      {sections.map((section) => (
        <div key={section.title} className="px-1 py-2">
          <p className="px-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500/86">
            {section.title}
          </p>
          <div className="mt-2 space-y-1.5">
            {section.items.map((item) => (
              <WorkspaceSidebarItem
                key={item.key}
                class_name="shadow-none"
                icon={item.dot_class_name ? <span className={`h-2.5 w-2.5 rounded-full ${item.dot_class_name}`} /> : undefined}
                icon_mode="plain"
                is_active={active_filter === item.key}
                on_click={() => on_change_filter(item.key)}
                size="compact"
                title={item.label}
                trailing={item.count}
              />
            ))}
          </div>
        </div>
      ))}
    </WorkspaceSidebarShell>
  );
}
