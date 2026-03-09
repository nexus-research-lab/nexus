/**
 * AskUserQuestion 工具类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 AskUserQuestionInput、UserQuestion、QuestionOption、UserQuestionAnswer
 * [POS]: types 模块的工具专用类型，被 ask-user-question-block.tsx 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

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
    questionIndex: number;
    /** 选中的选项 label 列表 */
    selectedOptions: string[];
}

/** 完整回答 */
export interface UserQuestionResponse {
    /** tool_use 的 id */
    toolUseId: string;
    /** 所有问题的回答 */
    answers: UserQuestionAnswer[];
}
