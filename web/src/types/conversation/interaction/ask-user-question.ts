export interface QuestionOption {
  label: string;
  description?: string;
}

export interface UserQuestion {
  question: string;
  header?: string;
  multi_select?: boolean;
  // SDK 工具输入可能未经后端字段归一化直接到达前端。
  multiSelect?: boolean;
  options: QuestionOption[];
}

export interface AskUserQuestionInput {
  questions: UserQuestion[];
}

export interface UserQuestionAnswer {
  question_index: number;
  selected_options: string[];
}
