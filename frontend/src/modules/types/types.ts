export type Category = 'critical' | 'fun' | 'important' | 'normal';

export interface Task {
  id: string;
  title: string;
  notes?: string;
  category: Category;
  order?: number;
  done?: boolean;
}

export interface Command {
  entityType: string;
  type: string;
  data?: any;
  idempotencyKey?: string;
}

export interface Settings {
  tasksPerCategory: number;
  showDoneTasks: boolean;
}