import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import TaskCard from '.';
import type { Task } from '../../types';

describe('TaskCard', () => {
  it('renders task title', () => {
    const task: Task = { id: '1', title: 'Sample', category: 'normal', notes: '', order: 0, done: false };
    render(<TaskCard task={task} />);
    expect(screen.getByText('Sample')).toBeTruthy();
  });
});

