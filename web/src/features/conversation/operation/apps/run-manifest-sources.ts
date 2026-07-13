import type { NexusOperationEvent } from "../operation-types";
import { consoleEventSubsystem } from "./run-manifest-console";

export interface ManifestLogSource {
  id: string;
  label: string;
  detail: string;
  count: number;
}

export function collectManifestLogSources(events: NexusOperationEvent[]): ManifestLogSource[] {
  const groups = new Map<string, ManifestLogSource>();

  groups.set("mac", {
    id: "mac",
    label: "这台 Mac",
    detail: "所有进程",
    count: events.length,
  });
  groups.set("nexus", {
    id: "nexus",
    label: "Nexus",
    detail: "智能体桌面",
    count: events.filter((event) => event.surface !== "conversation").length,
  });

  for (const event of events) {
    const subsystem = consoleEventSubsystem(event);
    const id = subsystem.toLowerCase().replace(/\s+/g, "-");
    const current = groups.get(id);
    if (current) {
      current.count += 1;
      continue;
    }
    groups.set(id, {
      id,
      label: subsystem,
      detail: subsystem_source_detail(subsystem),
      count: 1,
    });
  }

  return [...groups.values()];
}

function subsystem_source_detail(subsystem: string): string {
  if (subsystem === "Terminal") {
    return "命令与进程";
  }
  if (subsystem === "Navi") {
    return "网页与预览";
  }
  if (subsystem === "Finder") {
    return "文件系统";
  }
  if (subsystem === "Code") {
    return "编辑器";
  }
  if (subsystem === "Activity Monitor") {
    return "子任务";
  }
  if (subsystem === "Preview") {
    return "文档阅读";
  }
  return "系统日志";
}
