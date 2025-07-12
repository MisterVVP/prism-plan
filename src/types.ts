export type TaskType = 'critical' | 'fun' | 'important' | 'normal';

export interface Task {
  id: string;
  title: string;
  type: TaskType;
}