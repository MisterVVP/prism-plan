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
        reopenTask={() => {}}
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
        reopenTask={() => {}}
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

  it('appends moved task to end of destination category', () => {
    const tasks: Task[] = [
      { id: '1', title: 'Task A', category: 'normal', order: 0 },
      { id: '2', title: 'Task B', category: 'normal', order: 1 },
      { id: '3', title: 'Task C', category: 'fun', order: 0 },
    ];
    const updateTask = vi.fn();
    const ev: any = {
      active: { id: '3' },
      over: { id: '1', data: { current: { category: 'normal' } } }
    };
    handleDragEnd(ev, tasks, updateTask, vi.fn());
    expect(updateTask).toHaveBeenCalledWith('3', {
      category: 'normal',
      order: 2,
    });
  });

  it('ignores drag and drop within the same category', () => {
    const tasks: Task[] = [
      { id: '1', title: 'Task A', category: 'normal', order: 0 },
      { id: '2', title: 'Task B', category: 'normal', order: 1 },
    ];
    const updateTask = vi.fn();
    const ev: any = {
      active: { id: '1' },
      over: { id: '2', data: { current: { category: 'normal' } } }
    };
    handleDragEnd(ev, tasks, updateTask, vi.fn());
    expect(updateTask).not.toHaveBeenCalled();
  });

  it('reopens task on double click in done lane', () => {
    vi.useFakeTimers();
    const tasks: Task[] = [
      { id: '1', title: 'Done', category: 'fun', notes: '', order: 0, done: true }
    ];
    const reopenTask = vi.fn();
    const { getAllByText } = render(
      <Board
        tasks={tasks}
        settings={{ tasksPerCategory: 3, showDoneTasks: true }}
        updateTask={() => {}}
        completeTask={() => {}}
        reopenTask={reopenTask}
      />
    );
    const cardTitle = getAllByText('Done').find((el) => el.tagName === 'DIV') as HTMLElement;
    const card = cardTitle.parentElement as HTMLElement;
    fireEvent.click(card);
    fireEvent.click(card, { detail: 2 });
    vi.runAllTimers();
    expect(reopenTask).toHaveBeenCalledWith('1');
    vi.useRealTimers();
  });

  it('uses arrow controls to swap tasks within a lane', () => {
    const tasks: Task[] = [
      { id: '1', title: 'First', category: 'normal', order: 0 },
      { id: '2', title: 'Second', category: 'normal', order: 1 },
      { id: '3', title: 'Third', category: 'normal', order: 2 }
    ];
    const updateTask = vi.fn();
    render(
      <Board
        tasks={tasks}
        settings={{ tasksPerCategory: 5, showDoneTasks: false }}
        updateTask={updateTask}
        completeTask={() => {}}
        reopenTask={() => {}}
      />
    );
    const moveDown = screen.getByRole('button', { name: 'Move First down' });
    fireEvent.click(moveDown);
    expect(updateTask).toHaveBeenCalledTimes(2);
    expect(updateTask).toHaveBeenNthCalledWith(1, '1', { order: 1 });
    expect(updateTask).toHaveBeenNthCalledWith(2, '2', { order: 0 });
  });
});

