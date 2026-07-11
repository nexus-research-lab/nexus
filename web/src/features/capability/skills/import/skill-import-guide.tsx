import { FileText, Info } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";

import roomCollaborationMechanismMarkdown from "../../../../../../docs/specs/room-collaboration-mechanism.md?raw";

const SKILL_FRONTMATTER_EXAMPLE = `---
name: room-playbook
title: 群聊协作规则
description: 群聊中的协作流程和成员行为约束
scope: room
tags: [room, workflow]
---

# 群聊协作规则`;

const ROOM_COLLABORATION_MECHANISM_FILE_NAME = "room-collaboration-mechanism.md";

function downloadRoomCollaborationMechanism() {
  const blob = new Blob([roomCollaborationMechanismMarkdown], {
    type: "text/markdown;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = ROOM_COLLABORATION_MECHANISM_FILE_NAME;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export function SkillImportGuide({ importing }: { importing: boolean }) {
  return (
    <aside className="space-y-3">
      <div className="rounded-[12px] border border-(--divider-subtle-color) px-4 py-3">
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2 text-sm font-semibold text-(--text-strong)">
            <Info className="h-4 w-4 text-(--primary)" />
            SKILL.md 规范
          </div>
          <UiButton
            aria-label="下载 Room 协作机制 Markdown 文档"
            className="shrink-0"
            disabled={importing}
            onClick={downloadRoomCollaborationMechanism}
            size="xs"
            tone="primary"
            variant="surface"
          >
            <FileText className="h-3.5 w-3.5" />
            room协作机制
          </UiButton>
        </div>
        <ul className="space-y-1.5 text-xs leading-5 text-(--text-muted)">
          <li>必须包含 `name`，推荐补齐 `title`、`description`、`tags`。</li>
          <li>`scope: any` 可安装到 Agent；`scope: main` 只给主 Agent；`scope: room` 只给群聊。</li>
          <li>编写 Room Skill 时，把“room协作机制”文档交给 agent 参考，先明确公开协作和私下协作的边界。</li>
          <li>Room Skill 导入后在群聊管理弹窗的“群聊技能”里选择，不会安装到单个 Agent。</li>
          <li>Git 导入会保存 URL、branch、path 和 commit，后续检查更新会按这些信息比对远端版本。</li>
        </ul>
      </div>
      <pre className="max-h-[260px] overflow-auto rounded-[12px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,black_2%)] p-3 text-[11px] leading-5 text-(--text-default)">
        {SKILL_FRONTMATTER_EXAMPLE}
      </pre>
    </aside>
  );
}
