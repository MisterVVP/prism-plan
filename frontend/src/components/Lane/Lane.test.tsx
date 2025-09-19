import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import Lane, { aria } from '.';
import type { Task } from '@modules/types';

describe('Lane', () => {
  it('shows lane title', () => {
    const tasks: Task[] = [{ id: '1', title: 'Sample', category: 'critical', notes: '', order: 0, done: false }];
    render(<Lane category="critical" tasks={tasks} limit={3} />);
    const section = screen.getByRole('region', {
      name: aria.section('critical')['aria-label']
    });
    expect(section).toBeTruthy();
    expect(screen.getByText('Critical')).toBeTruthy();
  });
});

