/**
 * 权限类型定义
 *
 * [INPUT]: 无
 * [OUTPUT]: 对外提供权限请求、建议与响应类型
 * [POS]: types 模块的权限类型中心，被 hook 与权限弹窗消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { UserQuestionAnswer } from './ask-user-question';

export type PermissionRiskLevel = 'low' | 'medium' | 'high';
export type PermissionDecision = 'allow' | 'deny';
export type PermissionUpdateType =
  | 'addRules'
  | 'replaceRules'
  | 'removeRules'
  | 'setMode'
  | 'addDirectories'
  | 'removeDirectories';
export type PermissionBehavior = 'allow' | 'deny' | 'ask';
export type PermissionDestination = 'userSettings' | 'projectSettings' | 'localSettings' | 'session';

export interface PermissionRule {
  toolName: string;
  ruleContent?: string | null;
}

export interface PermissionUpdate {
  type: PermissionUpdateType;
  rules?: PermissionRule[];
  behavior?: PermissionBehavior;
  mode?: 'default' | 'acceptEdits' | 'plan' | 'bypassPermissions';
  directories?: string[];
  destination?: PermissionDestination;
}

export interface PendingPermission {
  request_id: string;
  tool_name: string;
  tool_input: Record<string, any>;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
}

export interface PermissionDecisionPayload {
  decision: PermissionDecision;
  userAnswers?: UserQuestionAnswer[];
  updatedPermissions?: PermissionUpdate[];
  message?: string;
  interrupt?: boolean;
}
