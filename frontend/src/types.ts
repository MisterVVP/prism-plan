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
  id: string;
  entityId: string;
  entityType: string;
  type: string;
  data?: any;
}

export interface Settings {
  tasksPerCategory: number;
  showDoneTasks: boolean;
}