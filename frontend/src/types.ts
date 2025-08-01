export type Category = 'critical' | 'fun' | 'important' | 'normal';

export interface Task {
  id: string;
  title: string;
  notes?: string;
  category: Category;
  order?: number;
  done?: boolean;
}

export type EventType = 'task-created' | 'task-updated' | 'task-completed';

export interface TaskEvent {
  id: string;
  entityId: string;
  entityType: string;
  type: EventType;
  data?: Partial<Task>;
  time: number;
}