// INPUT: 当前页面发起的精确 Skill 操作意图，以及只读 Skill 详情快照。
// OUTPUT: 页面内稳定意图键和只基于权威快照的 applied/unproven 对账结论。
// POS: Skill marketplace 写恢复的纯模型；不发请求、不生成业务 ID，也不执行重试。
import type { SkillDetail } from "@/types/capability/skill";

export type SkillOperationIntent =
  | {
      kind: "check_updates";
    }
  | {
      kind: "delete";
      skillName: string;
    }
  | {
      baselineHasUpdate: boolean;
      kind: "update";
      skillName: string;
    }
  | {
      fileLastModified: number;
      fileName: string;
      fileSize: number;
      fileType: string;
      kind: "import_local";
    }
  | {
      branch: string;
      kind: "import_git";
      path: string;
      url: string;
    }
  | {
      kind: "import_external";
      skillName: string;
      sourceKey: string;
      sourceRef: string;
    };

export type SkillOperationReconciliation = "applied" | "unproven";

function intentPart(value: string | number | boolean): string {
  const normalized = String(value);
  return `${normalized.length}:${normalized}`;
}

/**
 * 只用于当前页面的互斥与恢复；这个键不会发送给服务端，也不能充当资源或请求 ID。
 */
export function skillOperationIntentKey(intent: SkillOperationIntent): string {
  const parts: Array<string | number | boolean> = [intent.kind];
  switch (intent.kind) {
    case "check_updates":
      break;
    case "delete":
      parts.push(intent.skillName);
      break;
    case "update":
      // baselineHasUpdate 只是本次对账证据，不属于操作身份；目录刷新后它可能变化，
      // 但同一 Skill 的 update 锁必须保持同一个键。
      parts.push(intent.skillName);
      break;
    case "import_local":
      parts.push(
        intent.fileName,
        intent.fileSize,
        intent.fileLastModified,
        intent.fileType,
      );
      break;
    case "import_git":
      parts.push(intent.url, intent.branch, intent.path);
      break;
    case "import_external":
      parts.push(intent.skillName, intent.sourceKey, intent.sourceRef);
      break;
  }
  return parts.map(intentPart).join("|");
}

export function skillOperationTargetName(
  intent: SkillOperationIntent,
): string | null {
  switch (intent.kind) {
    case "delete":
    case "update":
    case "import_external":
      return intent.skillName;
    default:
      return null;
  }
}

/**
 * 只接受目标端点的权威详情。目录筛选结果、文案、时间邻近或请求诊断 ID
 * 都不能作为写入成功证据。
 */
export function reconcileSkillOperation(
  intent: SkillOperationIntent,
  detail: SkillDetail | null,
): SkillOperationReconciliation {
  switch (intent.kind) {
    case "delete":
      return detail === null ? "applied" : "unproven";
    case "update":
      return intent.baselineHasUpdate && detail?.has_update === false
        ? "applied"
        : "unproven";
    case "import_external":
      return detail !== null &&
          detail.name === intent.skillName &&
          normalizeSourceRef(detail.source_ref) === normalizeSourceRef(intent.sourceRef)
        ? "applied"
        : "unproven";
    case "check_updates":
    case "import_git":
    case "import_local":
      return "unproven";
  }
}

function normalizeSourceRef(value: string): string {
  return value.trim().replace(/^skills\.sh:/i, "");
}
