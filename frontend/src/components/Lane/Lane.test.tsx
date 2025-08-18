import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import Lane from '.';
import type { Task } from '../../types';

describe('Lane', () => {
  it('shows lane title', () => {
    const tasks: Task[] = [{ id: '1', title: 'Sample', category: 'critical', notes: '', order: 0, done: false }];
    render(<Lane category="critical" tasks={tasks} />);
    expect(screen.getByText('Critical')).toBeTruthy();
  });
});

