import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import TaskDetails from '.';
import type { Task } from '../../types';

describe('TaskDetails', () => {
  it('renders task information', () => {
    const task: Task = { id: '1', title: 'Detail', category: 'fun', notes: 'note', order: 0, done: false };
    render(<TaskDetails task={task} onBack={() => {}} />);
    expect(screen.getByText('Detail')).toBeTruthy();
  });
});

