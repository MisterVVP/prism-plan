import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import Board, { aria, handleDragEnd } from '.';
import type { Task } from '../../types';

describe('Board', () => {
  it('renders categories', () => {
    const settings = { tasksPerCategory: 3, showDoneTasks: true };
    render(
      <Board
        tasks={[]}
        settings={settings}
        updateTask={() => {}}
        completeTask={() => {}}
      />
    );
    const board = screen.getByRole('region', { name: aria.root['aria-label'] });
    expect(board).toBeTruthy();
    expect(screen.getByText('Critical')).toBeTruthy();
  });

  it('completes task on double click', () => {
    vi.useFakeTimers();
    const tasks: Task[] = [
      { id: '1', title: 'Sample', category: 'normal', notes: '', order: 0, done: false }
    ];
    const completeTask = vi.fn();
    const { getByText } = render(
      <Board
        tasks={tasks}
        settings={{ tasksPerCategory: 3, showDoneTasks: true }}
        updateTask={() => {}}
        completeTask={completeTask}
      />
    );
    const card = getByText('Sample').parentElement as HTMLElement;
    fireEvent.click(card);
    fireEvent.click(card, { detail: 2 });
    vi.runAllTimers();
    expect(completeTask).toHaveBeenCalledWith('1');
    vi.useRealTimers();
  });

  it('uncompletes task when moved out of done lane', () => {
    const tasks: Task[] = [
      { id: '1', title: 'Done', category: 'fun', notes: '', order: 0, done: true }
    ];
    const updateTask = vi.fn();
    const ev: any = {
      active: { id: '1' },
      over: { id: undefined, data: { current: { category: 'normal' } } }
    };
    handleDragEnd(ev, tasks, updateTask, vi.fn());
    expect(updateTask).toHaveBeenCalledWith('1', {
      category: 'normal',
      order: 0,
      done: false,
    });
  });
});

