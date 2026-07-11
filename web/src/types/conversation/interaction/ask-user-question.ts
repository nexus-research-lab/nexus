export interface QuestionOption {
  label: string;
  description?: string;
}

export interface UserQuestion {
  question: string;
  header?: string;
  multi_select?: boolean;
  options: QuestionOption[];
}

export interface UserQuestionAnswer {
  question_index: number;
  selected_options: string[];
}
