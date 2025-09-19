import type { Category } from '@modules/types';

const labelMap: Record<Category | 'done', string> = {
  critical: 'Critical tasks',
  fun: 'Fun tasks',
  important: 'Important tasks',
  normal: 'Normal tasks',
  done: 'Completed tasks'
};

export const aria = {
  section: (cat: Category | 'done') => ({
    role: 'region',
    'aria-label': labelMap[cat]
  })
};
