import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import Board from '.';
import type { Task } from '../../types';

describe('Board', () => {
  it('renders categories', () => {
    render(<Board tasks={[]} updateTask={() => {}} completeTask={() => {}} />);
    expect(screen.getByText('Critical')).toBeTruthy();
  });

  it('completes task on double click', () => {
    vi.useFakeTimers();
    const tasks: Task[] = [
      { id: '1', title: 'Sample', category: 'normal', notes: '', order: 0, done: false }
    ];
    const completeTask = vi.fn();
    const { getByText } = render(
      <Board tasks={tasks} updateTask={() => {}} completeTask={completeTask} />
    );
    const card = getByText('Sample').parentElement as HTMLElement;
    fireEvent.click(card);
    fireEvent.click(card, { detail: 2 });
    vi.runAllTimers();
    expect(completeTask).toHaveBeenCalledWith('1');
    vi.useRealTimers();
  });
});

