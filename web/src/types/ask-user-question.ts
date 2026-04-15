/**
 * AskUserQuestion 工具类型定义
 *
 * [INPUT]: 依赖 @/types/message 的 ToolResultContent
 * [OUTPUT]: 对外提供 AskUserQuestionInput、UserQuestion、QuestionOption、UserQuestionAnswer
 * [POS]: types 模块的工具专用类型，被 ask-user-question-block.tsx 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import type { ToolResultContent } from './message';

// ==================== AskUserQuestion 工具输入 ====================

/** 问题选项 */
export interface QuestionOption {
  /** 选项标签（唯一标识） */
  label: string;
  /** 选项描述 */
  description?: string;
}

/** 单个问题 */
export interface UserQuestion {
  /** 问题文本 */
  question: string;
  /** 问题标题/分类 */
  header?: string;
  /** 是否多选，默认 false */
  multi_select?: boolean;
  /** 兼容 Claude / SDK 直接透传的 camelCase 字段 */
  multiSelect?: boolean;
  /** 选项列表 */
  options: QuestionOption[];
}

/** AskUserQuestion 工具输入结构 */
export interface AskUserQuestionInput {
  questions: UserQuestion[];
}

// ==================== 用户回答 ====================

/** 单个问题的回答 */
export interface UserQuestionAnswer {
  /** 问题索引 */
  question_index: number;
  /** 选中的选项 label 列表 */
  selected_options: string[];
}

export const ASK_USER_QUESTION_TIMEOUT_ERROR_CODE = 'permission_request_timeout';

export function is_ask_user_question_timed_out_result(
  tool_result?: Pick<ToolResultContent, 'is_error' | 'error_code'> | null,
): boolean {
  return Boolean(
    tool_result?.is_error
    && tool_result.error_code === ASK_USER_QUESTION_TIMEOUT_ERROR_CODE,
  );
}
