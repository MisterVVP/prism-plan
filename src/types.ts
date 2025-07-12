export type Category = 'critical' | 'fun' | 'important' | 'normal';

export interface Task {
  id: string;
  title: string;
  notes?: string;
  category: Category;
  order?: number;
}